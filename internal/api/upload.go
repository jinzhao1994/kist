package api

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humago"
	"github.com/jinzhao1994/kist/internal/domain"
	"github.com/jinzhao1994/kist/internal/library"
)

const DefaultMaxUploadBytes int64 = 32 << 30
const DefaultMaxUploadFiles = 1000

type uploadInput struct {
	ID     string `path:"id"`
	Folder string `query:"folder"`
	Key    string `header:"Idempotency-Key" required:"true"`
}
type uploadFile struct {
	Path         string `json:"path"`
	LastModified int64  `json:"last_modified"`
	SHA256       string `json:"sha256"`
	Size         int64  `json:"size"`
	local        string
}
type uploadManifest struct {
	Files []uploadFile `json:"files"`
}

func (s *Server) uploadRoutes() {
	huma.Register(s.contract, huma.Operation{
		OperationID: "upload-files", Method: "POST", Path: Prefix + "/boxes/{id}/uploads",
		Summary:     "Upload a complete batch with a JSON manifest followed by file parts",
		Responses:   map[string]*huma.Response{"200": {Description: "Committed request receipt", Content: map[string]*huma.MediaType{"application/json": {Schema: s.contract.OpenAPI().Components.Schemas.Schema(reflect.TypeFor[library.RequestResult](), true, "RequestResult")}}}},
		Description: "The first multipart field is manifest: {files:[{path,last_modified,size,sha256}]}, with modification times in Unix milliseconds, byte lengths and lowercase SHA-256 digests calculated from original client files. All fields are required; outdated clients must refresh. Remaining fields are files, with filenames matching manifest paths. Retries bind paths, timestamps and content hashes, not multipart boundaries.",
		RequestBody: &huma.RequestBody{Required: true, Content: map[string]*huma.MediaType{"multipart/form-data": {Schema: &huma.Schema{Type: "string", Format: "binary", Description: "Streaming multipart body: manifest JSON followed by files. See operation description."}}}},
	}, func(ctx context.Context, in *uploadInput) (*huma.StreamResponse, error) {
		id, err := parseID(in.ID)
		if err != nil {
			return nil, err
		}
		opt := library.ImportOptions{Box: domain.BoxID{ID: id}, Hidden: true}
		if in.Folder != "" {
			folder, err := parseID(in.Folder)
			if err != nil {
				return nil, err
			}
			opt.Into = domain.FolderID{ID: folder}
		}
		if !(library.Request{ID: in.Key, Digest: strings.Repeat("0", 64)}).Valid() {
			return nil, failure(library.ErrInvalidRequest)
		}
		return &huma.StreamResponse{Body: func(hctx huma.Context) {
			req, w := humago.Unwrap(hctx)
			result, err := s.receiveUpload(req, in.Key, opt)
			if err != nil {
				writeFailure(w, err)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(result)
		}}, nil
	})
}

func (s *Server) receiveUpload(r *http.Request, key string, opt library.ImportOptions) (library.RequestResult, error) {
	var zero library.RequestResult
	reader, err := r.MultipartReader()
	if err != nil {
		return zero, invalid("Invalid multipart upload")
	}
	first, err := reader.NextPart()
	if err != nil {
		return zero, uploadReadError(err)
	}
	if first.FormName() != "manifest" {
		return zero, invalid("The first upload part must be manifest")
	}
	raw, err := io.ReadAll(io.LimitReader(first, maxJSONBody+1))
	if err != nil {
		return zero, uploadReadError(err)
	}
	if len(raw) > maxJSONBody {
		return zero, huma.Error413RequestEntityTooLarge("Upload manifest is too large")
	}
	var declared struct {
		Files []struct {
			Path         string `json:"path"`
			LastModified *int64 `json:"last_modified"`
			Size         *int64 `json:"size"`
			SHA256       string `json:"sha256"`
		} `json:"files"`
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&declared); err != nil {
		return zero, invalid("Invalid upload manifest")
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return zero, invalid("Trailing data in upload manifest")
	}
	if len(declared.Files) == 0 || len(declared.Files) > s.maxUploadFiles {
		return zero, invalid("Upload file count is outside the configured limit")
	}
	manifest := uploadManifest{Files: make([]uploadFile, 0, len(declared.Files))}
	for _, file := range declared.Files {
		if file.LastModified == nil {
			return zero, invalid("Every upload file needs last_modified in Unix milliseconds")
		}
		digest, err := hex.DecodeString(file.SHA256)
		if file.Size == nil || file.SHA256 == "" {
			return zero, invalid("Upload integrity information is required; refresh the page and select the files again")
		}
		if err != nil || len(digest) != sha256.Size || hex.EncodeToString(digest) != file.SHA256 || *file.Size < 0 {
			return zero, invalid("Invalid upload SHA-256 or byte length")
		}
		manifest.Files = append(manifest.Files, uploadFile{Path: file.Path, LastModified: *file.LastModified, Size: *file.Size, SHA256: file.SHA256})
	}
	byPath := map[string]int{}
	folded := map[string]bool{}
	for i, f := range manifest.Files {
		if !fs.ValidPath(f.Path) || f.Path == "." || strings.ContainsAny(f.Path, "\\\x00") || len(f.Path) > 4096 {
			return zero, invalid("Invalid upload relative path")
		}
		for _, part := range strings.Split(f.Path, "/") {
			if err := domain.ValidateName(part); err != nil {
				return zero, failure(err)
			}
		}
		if folded[domain.Fold(f.Path)] {
			return zero, invalid("Duplicate upload path")
		}
		folded[domain.Fold(f.Path)] = true
		byPath[f.Path] = i
		if f.LastModified < -9223372036854 || f.LastModified > 9223372036854 {
			return zero, invalid("Upload modification time is out of range")
		}
	}
	temp, err := os.MkdirTemp(s.uploadTempDir, "kist-upload-*")
	if err != nil {
		return zero, err
	}
	defer os.RemoveAll(temp)
	received := map[string]bool{}
	for {
		part, err := reader.NextPart()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return zero, uploadReadError(err)
		}
		_, params, err := mime.ParseMediaType(part.Header.Get("Content-Disposition"))
		if err != nil || params["name"] != "files" {
			return zero, invalid("Expected a file part")
		}
		// Part.FileName strips directories. Read the original header so traversal
		// cannot silently turn into a valid basename and directory imports survive.
		name := params["filename"]
		index, ok := byPath[name]
		if !ok || received[name] {
			return zero, invalid("Unexpected or duplicate upload file")
		}
		received[name] = true
		local := filepath.Join(temp, strconv.Itoa(index))
		handle, err := os.OpenFile(local, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		if err != nil {
			return zero, err
		}
		hash := sha256.New()
		size, copyErr := io.Copy(io.MultiWriter(handle, hash), part)
		closeErr := handle.Close()
		if copyErr != nil {
			return zero, uploadReadError(copyErr)
		}
		if closeErr != nil {
			return zero, closeErr
		}
		if err := r.Context().Err(); err != nil {
			return zero, err
		}
		modified := time.UnixMilli(manifest.Files[index].LastModified)
		if err := os.Chtimes(local, modified, modified); err != nil {
			return zero, err
		}
		manifest.Files[index].local = local
		if expected := manifest.Files[index]; size != expected.Size || hex.EncodeToString(hash.Sum(nil)) != expected.SHA256 {
			return zero, failure(library.ErrImportIntegrity)
		}
	}
	if _, err := io.Copy(io.Discard, r.Body); err != nil {
		return zero, uploadReadError(err)
	}
	if len(received) != len(manifest.Files) {
		return zero, invalid("Upload ended before every declared file arrived")
	}
	if err := r.Context().Err(); err != nil {
		return zero, err
	}
	sort.Slice(manifest.Files, func(i, j int) bool { return manifest.Files[i].Path < manifest.Files[j].Path })
	sources := make([]library.ImportSource, 0, len(manifest.Files))
	for _, f := range manifest.Files {
		sources = append(sources, library.ImportSource{Path: f.local, RelativePath: f.Path, Expected: &library.ImportIntegrity{SHA256: f.SHA256, Size: f.Size}})
	}
	prepared, prepareErr := s.library.PrepareFiles(r.Context(), sources, opt)
	if prepareErr == nil {
		outcome := prepared.Result()
		if outcome.Interrupted {
			prepareErr = r.Context().Err()
			if prepareErr == nil {
				prepareErr = context.Canceled
			}
		}
		for _, file := range outcome.Files {
			if file.Status == library.FileFailed {
				prepareErr = file.Err
				break
			}
		}
	}
	result, err := s.mutate(r.Context(), key, "POST", Prefix+"/boxes/"+opt.Box.String()+"/uploads", struct {
		Folder string       `json:"folder"`
		Files  []uploadFile `json:"files"`
	}{opt.Into.String(), manifest.Files}, func(ctx context.Context) error {
		if prepareErr != nil {
			return prepareErr
		}
		_, err := prepared.Commit(ctx)
		return err
	})
	if err != nil {
		return zero, err
	}
	return result.Body, nil
}
func uploadReadError(err error) error {
	var limit *http.MaxBytesError
	if errors.As(err, &limit) {
		return huma.Error413RequestEntityTooLarge("Upload exceeds the configured byte limit")
	}
	return invalid("Incomplete or malformed multipart upload")
}
