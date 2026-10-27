package library

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jinzhao1994/kist/internal/ai"
	"github.com/jinzhao1994/kist/internal/ai/openai"
	"github.com/jinzhao1994/kist/internal/config"
	"github.com/jinzhao1994/kist/internal/derive"
	"github.com/jinzhao1994/kist/internal/domain"
	"github.com/jinzhao1994/kist/internal/extract"
	"github.com/jinzhao1994/kist/internal/lockfile"
	"github.com/jinzhao1994/kist/internal/oplog"
	"github.com/jinzhao1994/kist/internal/store"
)

// AIConnection reloads private configuration without changing the settings revision.
func (l *Library) AIConnection(ctx context.Context) (ai.Credentials, error) {
	var out ai.Credentials
	err := l.updateAI(ctx, func(c *ai.Credentials) error { out = *c; return nil })
	return out, err
}

// PrepareAIConnection persists a stable host before an explicit sign-in.
func (l *Library) PrepareAIConnection(ctx context.Context) (ai.Credentials, error) {
	var out ai.Credentials
	err := l.updateAI(ctx, func(c *ai.Credentials) error {
		if c.HostID == "" {
			c.HostID = "urn:uuid:" + uuid.NewString()
		}
		out = *c
		return nil
	})
	return out, err
}
func (l *Library) updateAI(ctx context.Context, fn func(*ai.Credentials) error) error {
	release, err := lockfile.New(filepath.Join(l.layout.Root, "config.lock")).Acquire(ctx)
	if err != nil {
		return err
	}
	defer release()
	cfg, err := config.Load(l.layout.ConfigPath())
	if err != nil {
		return err
	}
	before := cfg.AI
	if err = fn(&cfg.AI); err != nil {
		return err
	}
	if before == cfg.AI {
		return nil
	}
	c := cfg.AI
	return config.Update(l.layout.ConfigPath(), "", map[string]any{"ai.host_id": c.HostID, "ai.provider": c.Provider, "ai.client_id": c.ClientID, "ai.subject": c.Subject, "ai.email": c.Email, "ai.access_token": c.AccessToken, "ai.refresh_token": c.RefreshToken, "ai.id_token": c.IDToken, "ai.scope": c.Scope, "ai.expires_at": c.ExpiresAt})
}
func (l *Library) SaveAIConnection(ctx context.Context, before, next ai.Credentials) error {
	return l.updateAI(ctx, func(c *ai.Credentials) error {
		if *c != before {
			return config.ErrChanged
		}
		*c = next
		return nil
	})
}
func (l *Library) AIProvider(ctx context.Context) (ai.Provider, error) {
	client, err := l.AIHTTP(3 * time.Minute)
	if err != nil {
		return nil, err
	}
	auth, err := l.AIHTTP(30 * time.Second)
	if err != nil {
		return nil, err
	}
	var token string
	err = l.updateAI(ctx, func(c *ai.Credentials) error {
		if c.Provider != "openai" {
			return errors.New("connect ChatGPT first")
		}
		next, err := openai.Refresh(ctx, *c, auth)
		if err != nil {
			return err
		}
		*c = next
		token = c.AccessToken
		return nil
	})
	return openai.Client{Token: token, HTTP: client}, err
}
func (l *Library) DisconnectAI(ctx context.Context) (bool, error) {
	client, err := l.AIHTTP(30 * time.Second)
	if err != nil {
		return false, err
	}
	confirmed := true
	err = l.updateAI(ctx, func(c *ai.Credentials) error {
		if openai.Revoke(ctx, *c, client) != nil {
			confirmed = false
		}
		c.AccessToken = ""
		c.RefreshToken = ""
		c.IDToken = ""
		c.Scope = ""
		c.ExpiresAt = 0
		return nil
	})
	return confirmed, err
}

type ocrExtractor struct{}

func (ocrExtractor) ID() string           { return "ocr" }
func (ocrExtractor) Version() int         { return 1 }
func (ocrExtractor) Namespaces() []string { return []string{"ocr"} }
func (ocrExtractor) Wants(m string) bool {
	return m == "image/jpeg" || m == "image/png" || m == "image/webp" || m == "image/heic" || m == "image/heif"
}
func (ocrExtractor) Extract(context.Context, io.ReadSeeker, string) ([]extract.KV, error) {
	return nil, errors.New("OCR requires an explicit provider")
}

type descriptionExtractor struct{ ocrExtractor }

func (descriptionExtractor) ID() string           { return "desc" }
func (descriptionExtractor) Namespaces() []string { return []string{"desc"} }

// ErrOCRChanged rejects inference based on a stale file or metadata snapshot.
var ErrOCRChanged = errors.New("file changed during AI extraction; reload before trying again")

type PreparedOCR struct {
	extractor string
	library   *Library
	file      domain.File
	rows      []store.MetadataRow
	version   int
	payload   []byte
}

func (l *Library) PrepareOCR(ctx context.Context, id domain.FileID, provider ai.Provider, model string) (*PreparedOCR, error) {
	return l.prepareOCR(ctx, id, provider, model, "")
}
func (l *Library) prepareOCR(ctx context.Context, id domain.FileID, provider ai.Provider, model, expectedBlob string) (*PreparedOCR, error) {
	return l.prepareImageText(ctx, id, provider, model, expectedBlob, "ocr", "", false)
}
func (l *Library) PrepareDescription(ctx context.Context, id domain.FileID, provider ai.Provider, model, language string) (*PreparedOCR, error) {
	return l.prepareImageText(ctx, id, provider, model, "", "desc", language, false)
}
func (l *Library) prepareImageText(ctx context.Context, id domain.FileID, provider ai.Provider, model, expectedBlob, kind, language string, force bool) (*PreparedOCR, error) {
	if ctx.Value(requestContextKey{}) != nil {
		return nil, ErrRequestScope
	}
	p := &PreparedOCR{library: l, extractor: kind}
	var zone *time.Location
	err := l.readTx(ctx, func(tx *store.ViewTx) error {
		var ok bool
		var err error
		p.file, ok, err = tx.GetFile(id)
		if err != nil {
			return err
		}
		if !ok {
			return ErrNoSuchFile
		}
		p.rows, err = store.FileMetadata(tx.Queryer(), id)
		if err != nil {
			return err
		}
		versions, err := store.Extractions(tx.Queryer(), id)
		p.version = versions[kind]
		zone = l.zone()
		return err
	})
	if err != nil {
		return nil, err
	}
	if expectedBlob != "" && p.file.BlobKey != expectedBlob {
		return nil, ErrOCRChanged
	}
	if !force && p.version >= 1 {
		return p, nil
	}
	path, ok := l.blobs.LocalPath(blobKey(p.file.BlobKey))
	if !ok {
		return nil, ErrNoSuchFile
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, ai.MaxImageBytes+1))
	if err != nil {
		return nil, err
	}
	if len(data) > ai.MaxImageBytes {
		return nil, errors.New("AI image exceeds 20 MiB")
	}
	mime, err := extract.DetectMedia(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	var e extract.Extractor = ocrExtractor{}
	if kind == "desc" {
		e = descriptionExtractor{}
	}
	if !e.Wants(mime) {
		return nil, errors.New("AI extraction supports JPEG, PNG, static WebP and still HEIC/HEIF images")
	}
	// Inspect compressed chunks without allocating animation frames.
	if mime == "image/webp" {
		for pos := 12; pos+8 <= len(data); {
			chunk := string(data[pos : pos+4])
			if chunk == "ANIM" || chunk == "ANMF" {
				return nil, errors.New("AI extraction requires a static image")
			}
			n := uint64(binary.LittleEndian.Uint32(data[pos+4 : pos+8]))
			next := uint64(pos) + 8 + n + (n % 2)
			if next > uint64(len(data)) {
				return nil, errors.New("invalid WebP image")
			}
			pos = int(next)
		}
	}
	if mime == "image/heic" || mime == "image/heif" {
		data, err = (derive.Renderer{}).HEIFJPEG(ctx, path, ai.MaxImageBytes)
		if err != nil {
			return nil, err
		}
		mime = "image/jpeg"
	}
	input, err := ai.NormalizeImage(ai.Image{MIME: mime, Data: data})
	if err != nil {
		return nil, err
	}
	var result ai.TextResult
	if kind == "desc" {
		result, err = provider.Describe(ctx, model, input, language)
	} else {
		result, err = provider.OCR(ctx, model, input)
	}
	if err != nil {
		return nil, err
	}
	if err = ctx.Err(); err != nil {
		return nil, err
	}
	if kind == "desc" && strings.TrimSpace(result.Text) == "" {
		return nil, errors.New("description is empty; no result was saved")
	}
	recipe := 1
	if kind == "desc" {
		recipe = ai.DescriptionRecipeVersion
	}
	facts := []extract.KV{{Key: kind + ":text", Value: result.Text, Type: extract.TypeText}, {Key: kind + ":provider", Value: result.Provider, Type: extract.TypeText}, {Key: kind + ":model", Value: result.Model, Type: extract.TypeText}, {Key: kind + ":input_sha256", Value: fmt.Sprint(p.file.BlobKey), Type: extract.TypeText}, {Key: kind + ":recipe", Value: fmt.Sprint(recipe), Type: extract.TypeInt}}
	if kind == "desc" {
		facts = append(facts, extract.KV{Key: "desc:language", Value: language, Type: extract.TypeText})
	}
	if err = extract.Check(e, facts); err != nil {
		return nil, err
	}
	change := extractionChange(e, p.file, facts, p.rows, zone)
	// AI extraction has no date authority and must not reinterpret capture time.
	change.CapturedAt = p.file.CapturedAt
	change.CapturedWas = p.file.CapturedAt
	p.payload, err = oplog.Marshal(oplog.ExtractMetadataPayload{Extractor: kind, Version: 1, PreviousVersion: p.version, Files: []oplog.FileExtraction{*change}})
	return p, err
}

// Commit runs only inside ExecuteRequest, whose lock covers checks and append.
func (p *PreparedOCR) Commit(ctx context.Context) error {
	scope, ok := ctx.Value(requestContextKey{}).(*requestScope)
	if !ok || scope.library != p.library || !scope.active {
		return ErrRequestScope
	}
	if len(p.payload) == 0 {
		return nil
	}
	l := p.library
	file, ok, err := l.File(ctx, p.file.ID)
	if err != nil {
		return err
	}
	if !ok || file.BlobKey != p.file.BlobKey {
		return ErrOCRChanged
	}
	rows, err := store.FileMetadata(l.db, p.file.ID)
	if err != nil {
		return err
	}
	versions, err := store.Extractions(l.db, p.file.ID)
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(rows, p.rows) || versions[p.extractor] != p.version || file.CapturedAt != p.file.CapturedAt {
		return ErrOCRChanged
	}
	_, err = l.write(ctx, oplog.ActionExtractMetadata, p.payload, oplog.OriginExtractor, writeOptions{})
	return err
}

// AIHTTP rereads the per-instance proxy before each connection.
func (l *Library) AIHTTP(timeout time.Duration) (*http.Client, error) {
	cfg, err := config.Load(l.layout.ConfigPath())
	if err != nil {
		return nil, err
	}
	return ai.HTTPClient(cfg.AIOptions.ProxyURL, timeout)
}
