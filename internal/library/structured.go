package library

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jinzhao1994/kist/internal/ai"
	"github.com/jinzhao1994/kist/internal/derive"
	"github.com/jinzhao1994/kist/internal/domain"
	"github.com/jinzhao1994/kist/internal/extract"
	"github.com/jinzhao1994/kist/internal/structured"
)

type factsExtractor struct{ ocrExtractor }

func (factsExtractor) ID() string           { return "facts" }
func (factsExtractor) Namespaces() []string { return []string{"facts"} }

func (l *Library) PrepareFacts(ctx context.Context, id domain.FileID, provider ai.Provider, model, language string) (*PreparedOCR, error) {
	return l.prepareImageText(ctx, id, provider, model, "", "facts", language, false)
}

func (l *Library) prepareDocumentFacts(ctx context.Context, p *PreparedOCR, provider ai.Provider, model, path, mime, language, supplement string, zone *time.Location) (*PreparedOCR, error) {
	recipe, ok := descriptionDocumentRecipe(p.file.Name, mime)
	if !ok {
		return nil, errors.New("structured extraction supports images, PDF, Word and PowerPoint files")
	}
	if _, ok := provider.(ai.Structurer); !ok {
		return nil, errors.New("the selected provider does not support structured extraction")
	}
	document, err := (derive.Renderer{}).Document(ctx, path, recipe, ai.MaxDocumentPages, ai.MaxDocumentBytes)
	if err != nil {
		return nil, err
	}
	input := ai.Document{Data: document.Data, Pages: document.Pages, TotalPages: document.TotalPages}
	return p.prepareFacts(ctx, provider, model, ai.FactsInput{Document: &input}, language, supplement, zone)
}

func (p *PreparedOCR) prepareFacts(ctx context.Context, provider ai.Provider, model string, input ai.FactsInput, language, supplement string, zone *time.Location) (*PreparedOCR, error) {
	structurer, ok := provider.(ai.Structurer)
	if !ok {
		return nil, errors.New("the selected provider does not support structured extraction")
	}
	previous := structured.Document{Version: structured.Version, Fields: []structured.Field{}}
	for _, row := range p.rows {
		if row.Key == "facts:data" && !row.Tombstone {
			var err error
			previous, err = structured.Parse(row.Value)
			if err != nil {
				return nil, errors.New("existing structured metadata is invalid; no result was saved")
			}
		}
	}
	result, err := structurer.ExtractFacts(ctx, model, input, ai.FactsOptions{TextOptions: ai.TextOptions{Language: language, Supplement: supplement}, Outline: structured.Outline(previous)})
	if err != nil {
		return nil, err
	}
	next, err := structured.ParseModel(result.Text)
	if err != nil {
		return nil, fmt.Errorf("invalid structured extraction: %w", err)
	}
	merged, err := structured.Merge(previous, next)
	if err != nil {
		return nil, err
	}
	raw, err := structured.Encode(merged)
	if err != nil {
		return nil, err
	}
	return p.finishAI(ctx, factsExtractor{}, []extract.KV{
		{Key: "facts:data", Value: raw, Type: extract.TypeText},
		{Key: "facts:hint", Value: "", Type: extract.TypeText},
		{Key: "facts:model", Value: result.Provider + "/" + result.Model, Type: extract.TypeText},
		{Key: "facts:recipe", Value: fmt.Sprint(ai.FactsRecipeVersion), Type: extract.TypeInt},
	}, zone)
}
