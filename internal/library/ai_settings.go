package library

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"path/filepath"

	"github.com/jinzhao1994/kist/internal/config"
	"github.com/jinzhao1994/kist/internal/lockfile"
	"github.com/jinzhao1994/kist/internal/store"
)

type OCRSettings struct {
	AutoDesc bool   `json:"auto_desc"`
	Model    string `json:"model" maxLength:"200"`
	AutoOCR  bool   `json:"auto_ocr"`
	ProxyURL string `json:"proxy_url" maxLength:"2048"`
	Revision string `json:"revision"`
}

func ocrSettings(c config.AIOptions) OCRSettings {
	// Include the private activation boundary in the revision.
	raw, _ := json.Marshal(struct {
		Options             config.AIOptions
		OCRAfter, DescAfter int64
	}{c, c.AfterOp, c.DescAfterOp})
	sum := sha256.Sum256(raw)
	return OCRSettings{Model: c.Model, AutoOCR: c.AutoOCR, AutoDesc: c.AutoDesc, ProxyURL: c.ProxyURL, Revision: hex.EncodeToString(sum[:])}
}
func (l *Library) OCRSettings(ctx context.Context) (OCRSettings, error) {
	cfg, err := config.Load(l.layout.ConfigPath())
	if err != nil {
		return OCRSettings{}, err
	}
	return ocrSettings(cfg.AIOptions), nil
}
func (l *Library) SaveOCRSettings(ctx context.Context, in OCRSettings) (OCRSettings, error) {
	release, err := l.lock.Acquire(ctx)
	if err != nil {
		return OCRSettings{}, err
	}
	defer release()
	if err = l.catchUpLocked(ctx); err != nil {
		return OCRSettings{}, err
	}
	unlock, err := lockfile.New(filepath.Join(l.layout.Root, "config.lock")).Acquire(ctx)
	if err != nil {
		return OCRSettings{}, err
	}
	defer unlock()
	cfg, err := config.Load(l.layout.ConfigPath())
	if err != nil {
		return OCRSettings{}, err
	}
	if in.Revision == "" || in.Revision != ocrSettings(cfg.AIOptions).Revision {
		return OCRSettings{}, config.ErrChanged
	}
	next := config.AIOptions{AutoDesc: in.AutoDesc, DescAfterOp: cfg.AIOptions.DescAfterOp, Model: in.Model, AutoOCR: in.AutoOCR, AfterOp: cfg.AIOptions.AfterOp, ProxyURL: in.ProxyURL}
	if next.AutoOCR && !cfg.AIOptions.AutoOCR {
		next.AfterOp, err = store.MaxOpID(l.db)
		if err != nil {
			return OCRSettings{}, err
		}
	}
	if next.AutoDesc && !cfg.AIOptions.AutoDesc {
		next.DescAfterOp, err = store.MaxOpID(l.db)
		if err != nil {
			return OCRSettings{}, err
		}
	}
	err = config.Update(l.layout.ConfigPath(), "", map[string]any{"ai_options.auto_desc": next.AutoDesc, "ai_options.desc_after_op": next.DescAfterOp, "ai_options.model": next.Model, "ai_options.auto_ocr": next.AutoOCR, "ai_options.after_op": next.AfterOp, "ai_options.proxy_url": next.ProxyURL})
	return ocrSettings(next), err
}
