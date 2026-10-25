package library

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"time"

	"github.com/google/uuid"
	"github.com/jinzhao1994/kist/internal/ai"
	"github.com/jinzhao1994/kist/internal/ai/openai"
	"github.com/jinzhao1994/kist/internal/config"
	"github.com/jinzhao1994/kist/internal/domain"
	"github.com/jinzhao1994/kist/internal/extract"
	"github.com/jinzhao1994/kist/internal/lockfile"
	"github.com/jinzhao1994/kist/internal/oplog"
	"github.com/jinzhao1994/kist/internal/store"
)

// AIConnection reloads private configuration; no token cache can outlive a writer.
func (l *Library) AIConnection(ctx context.Context) (ai.Credentials, error) {
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
	var token string
	err := l.updateAI(ctx, func(c *ai.Credentials) error {
		if c.Provider != "openai" {
			return errors.New("connect ChatGPT first")
		}
		next, err := openai.Refresh(ctx, *c)
		if err != nil {
			return err
		}
		*c = next
		token = c.AccessToken
		return nil
	})
	return openai.Client{Token: token}, err
}
func (l *Library) DisconnectAI(ctx context.Context) (bool, error) {
	confirmed := true
	err := l.updateAI(ctx, func(c *ai.Credentials) error {
		if openai.Revoke(ctx, *c) != nil {
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
	return m == "image/jpeg" || m == "image/png" || m == "image/webp"
}
func (ocrExtractor) Extract(context.Context, io.ReadSeeker, string) ([]extract.KV, error) {
	return nil, errors.New("OCR requires an explicit provider")
}

// ErrOCRChanged rejects inference based on a stale file or metadata snapshot.
var ErrOCRChanged = errors.New("file changed during OCR; reload before trying again")

type PreparedOCR struct {
	library *Library
	file    domain.File
	rows    []store.MetadataRow
	version int
	payload []byte
}

func (l *Library) PrepareOCR(ctx context.Context, id domain.FileID, provider ai.Provider, model string) (*PreparedOCR, error) {
	if ctx.Value(requestContextKey{}) != nil {
		return nil, ErrRequestScope
	}
	p := &PreparedOCR{library: l}
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
		p.version = versions["ocr"]
		zone = l.zone()
		return err
	})
	if err != nil {
		return nil, err
	}
	if p.version >= 1 {
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
		return nil, errors.New("OCR image exceeds 20 MiB")
	}
	mime, err := extract.DetectMedia(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	e := ocrExtractor{}
	if !e.Wants(mime) {
		return nil, errors.New("OCR supports JPEG, PNG and static WebP images")
	}
	// Inspect compressed chunks without allocating animation frames.
	if mime == "image/webp" {
		for pos := 12; pos+8 <= len(data); {
			chunk := string(data[pos : pos+4])
			if chunk == "ANIM" || chunk == "ANMF" {
				return nil, errors.New("OCR requires a static image")
			}
			n := uint64(binary.LittleEndian.Uint32(data[pos+4 : pos+8]))
			next := uint64(pos) + 8 + n + (n % 2)
			if next > uint64(len(data)) {
				return nil, errors.New("invalid WebP image")
			}
			pos = int(next)
		}
	}
	result, err := provider.OCR(ctx, model, ai.Image{MIME: mime, Data: data})
	if err != nil {
		return nil, err
	}
	if err = ctx.Err(); err != nil {
		return nil, err
	}
	facts := []extract.KV{{Key: "ocr:text", Value: result.Text, Type: extract.TypeText}, {Key: "ocr:provider", Value: result.Provider, Type: extract.TypeText}, {Key: "ocr:model", Value: result.Model, Type: extract.TypeText}, {Key: "ocr:input_sha256", Value: fmt.Sprint(p.file.BlobKey), Type: extract.TypeText}, {Key: "ocr:recipe", Value: "1", Type: extract.TypeInt}}
	if err = extract.Check(e, facts); err != nil {
		return nil, err
	}
	change := extractionChange(e, p.file, facts, p.rows, zone)
	// OCR has no date authority and must not reinterpret capture time.
	change.CapturedAt = p.file.CapturedAt
	change.CapturedWas = p.file.CapturedAt
	p.payload, err = oplog.Marshal(oplog.ExtractMetadataPayload{Extractor: "ocr", Version: 1, PreviousVersion: p.version, Files: []oplog.FileExtraction{*change}})
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
	if !reflect.DeepEqual(rows, p.rows) || versions["ocr"] != p.version || file.CapturedAt != p.file.CapturedAt {
		return ErrOCRChanged
	}
	_, err = l.write(ctx, oplog.ActionExtractMetadata, p.payload, oplog.OriginExtractor, writeOptions{})
	return err
}
