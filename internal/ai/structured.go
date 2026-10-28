package ai

import (
	"context"
	_ "embed"
	"errors"
	"strings"
)

const FactsRecipeVersion = 2

// FactsInput has exactly one source, prepared by the library. It never contains
// OCR/description output in place of the original visual source.
type FactsInput struct {
	Image    *Image
	Document *Document
}

type FactsOptions struct {
	TextOptions
	Outline string
}

// Structurer is optional; providers without it must fail explicitly.
type Structurer interface {
	ExtractFacts(context.Context, string, FactsInput, FactsOptions) (TextResult, error)
}

func (in FactsInput) Validate() error {
	if (in.Image == nil) == (in.Document == nil) {
		return errors.New("structured extraction requires exactly one input")
	}
	if in.Image != nil {
		if len(in.Image.Data) == 0 || len(in.Image.Data) > MaxImageBytes {
			return errors.New("image exceeds AI input limit")
		}
		return nil
	}
	d := in.Document
	if len(d.Data) == 0 || len(d.Data) > MaxDocumentBytes || d.Pages < 1 || d.Pages > MaxDocumentPages || d.Pages != d.TotalPages {
		return errors.New("structured extraction requires a complete document within AI input limits")
	}
	return nil
}

//go:embed prompts/structured.txt
var factsInstructions string

//go:embed prompts/structured-input.txt
var factsInput string

//go:embed prompts/structured-supplement.txt
var factsSupplement string

// FactsPrompt has no provider-specific response format or transport settings.
func FactsPrompt(options FactsOptions) Prompt {
	p := Prompt{Instructions: strings.ReplaceAll(strings.TrimSpace(factsInstructions), "{{language}}", options.Language), Input: strings.TrimSpace(factsInput)}
	if options.Outline != "" {
		p.Input += "\n\nExisting field outline (structure only, not facts):\n" + options.Outline
	}
	return withSupplement(p, options.Supplement, factsSupplement)
}
