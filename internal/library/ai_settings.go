package library

import (
	"context"

	"github.com/jinzhao1994/kist/internal/config"
	"github.com/jinzhao1994/kist/internal/store"
)

type OCRSettings struct {
	Explicit        []string          `json:"explicit" readOnly:"true"`
	Changed         []string          `json:"changed,omitempty" maxItems:"6"`
	Defaults        *config.AIOptions `json:"defaults,omitempty"`
	Reset           []string          `json:"reset,omitempty" maxItems:"6"`
	AutoFacts       bool              `json:"auto_facts"`
	AutoOrientation bool              `json:"auto_orientation"`
	AutoDesc        bool              `json:"auto_desc"`
	Model           string            `json:"model" maxLength:"200"`
	AutoOCR         bool              `json:"auto_ocr"`
	ProxyURL        string            `json:"proxy_url" maxLength:"2048"`
	Revision        string            `json:"revision"`
}

func ocrSettings(c config.AIOptions) OCRSettings {
	defaults := config.Defaults("library").AIOptions
	return OCRSettings{Defaults: &defaults, AutoFacts: c.AutoFacts, AutoOrientation: c.AutoOrientation, Model: c.Model, AutoOCR: c.AutoOCR, AutoDesc: c.AutoDesc, ProxyURL: c.ProxyURL}
}
func (l *Library) OCRSettings(ctx context.Context) (OCRSettings, error) {
	cfg, explicit, revision, err := settingsSnapshot(l.layout.ConfigPath(), "ai_options", config.Defaults("library").AIOptions)
	if err != nil {
		return OCRSettings{}, err
	}
	out := ocrSettings(cfg.AIOptions)
	out.Explicit, out.Revision = explicit, revision
	return out, ctx.Err()
}
func (l *Library) SaveOCRSettings(ctx context.Context, in OCRSettings) (OCRSettings, error) {
	if err := l.UpdateCombinedSettings(ctx, in.Revision, map[string]any{}, &in, nil); err != nil {
		return OCRSettings{}, err
	}
	return l.OCRSettings(ctx)
}

func (l *Library) ocrSettingsChanges(cfg *config.Config, in OCRSettings) (map[string]any, error) {
	in, selected, err := prepareSettings(in, ocrSettings(cfg.AIOptions), config.Defaults("library").AIOptions, in.Changed, in.Reset)
	if err != nil {
		return nil, err
	}
	next := config.AIOptions{AutoFacts: in.AutoFacts, FactsAfterOp: cfg.AIOptions.FactsAfterOp, AutoOrientation: in.AutoOrientation, OrientationAfterOp: cfg.AIOptions.OrientationAfterOp, AutoDesc: in.AutoDesc, DescAfterOp: cfg.AIOptions.DescAfterOp, Model: in.Model, AutoOCR: in.AutoOCR, AfterOp: cfg.AIOptions.AfterOp, ProxyURL: in.ProxyURL}
	if next.AutoOCR && !cfg.AIOptions.AutoOCR {
		next.AfterOp, err = store.MaxOpID(l.db)
		if err != nil {
			return nil, err
		}
	}
	if next.AutoFacts && !cfg.AIOptions.AutoFacts {
		next.FactsAfterOp, err = store.MaxOpID(l.db)
		if err != nil {
			return nil, err
		}
	}
	if next.AutoDesc && !cfg.AIOptions.AutoDesc {
		next.DescAfterOp, err = store.MaxOpID(l.db)
		if err != nil {
			return nil, err
		}
	}
	if next.AutoOrientation && !cfg.AIOptions.AutoOrientation {
		next.OrientationAfterOp, err = store.MaxOpID(l.db)
		if err != nil {
			return nil, err
		}
	}
	changes := map[string]any{"ai_options.auto_facts": next.AutoFacts, "ai_options.auto_orientation": next.AutoOrientation, "ai_options.auto_desc": next.AutoDesc, "ai_options.model": next.Model, "ai_options.auto_ocr": next.AutoOCR, "ai_options.proxy_url": next.ProxyURL}
	for key := range changes {
		field := key[len("ai_options."):]
		if !selected[field] {
			delete(changes, key)
		}
	}
	for key, value := range map[string]int64{"after_op": next.AfterOp, "facts_after_op": next.FactsAfterOp, "orientation_after_op": next.OrientationAfterOp, "desc_after_op": next.DescAfterOp} {
		changes["ai_options."+key] = value
	}
	for _, key := range in.Reset {
		changes["ai_options."+key] = nil
	}
	return changes, nil
}
