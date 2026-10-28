package library

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jinzhao1994/kist/internal/ai"
	"github.com/jinzhao1994/kist/internal/derive"
	"github.com/jinzhao1994/kist/internal/extract"
)

// The profile chooses a local converter; that converter validates actual bytes.
func descriptionDocumentRecipe(name, mime string) (string, bool) {
	if mime == "application/pdf" {
		return "", true
	}
	profile, ok := derive.PreviewFor(name, "", mime)
	if ok && (profile.ID == "word" || profile.ID == "slides") {
		return profile.Detail, true
	}
	return "", false
}

func boundedDescription(text string) (string, error) {
	text = strings.TrimSpace(text)
	if text == "" || len(text) > 16<<10 || !utf8.ValidString(text) {
		return "", errors.New("description is empty or exceeds the output limit; no result was saved")
	}
	if len(text) > ai.MaxDescriptionBytes {
		end := ai.MaxDescriptionBytes - len("…")
		for !utf8.RuneStart(text[end]) {
			end--
		}
		text = strings.TrimSpace(text[:end]) + "…"
	}
	return text, nil
}

func (l *Library) prepareDocumentDescription(ctx context.Context, p *PreparedOCR, provider ai.Provider, model, path, mime, language, supplement string, zone *time.Location) (*PreparedOCR, error) {
	recipe, ok := descriptionDocumentRecipe(p.file.Name, mime)
	if !ok {
		return nil, errors.New("descriptions support images, PDF, Word and PowerPoint files")
	}
	describer, ok := provider.(ai.DocumentDescriber)
	if !ok {
		return nil, errors.New("the selected provider does not support document descriptions")
	}
	document, err := (derive.Renderer{}).Document(ctx, path, recipe, ai.MaxDocumentPages, ai.MaxDocumentBytes)
	if err != nil {
		return nil, err
	}
	result, err := describer.DescribeDocument(ctx, model, ai.Document{Data: document.Data, Pages: document.Pages, TotalPages: document.TotalPages}, ai.TextOptions{Language: language, Supplement: supplement})
	if err != nil {
		return nil, err
	}
	result.Text, err = boundedDescription(result.Text)
	if err != nil {
		return nil, err
	}
	facts := []extract.KV{
		{Key: "desc:hint", Value: "", Type: extract.TypeText},
		{Key: "desc:text", Value: result.Text, Type: extract.TypeText},
		{Key: "desc:model", Value: result.Provider + "/" + result.Model, Type: extract.TypeText},
		{Key: "desc:recipe", Value: fmt.Sprint(ai.DescriptionRecipeVersion), Type: extract.TypeInt},
	}
	return p.finishAI(ctx, descriptionExtractor{}, facts, zone)
}
