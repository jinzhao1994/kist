package openai

import (
	"context"
	"encoding/base64"
	"github.com/jinzhao1994/kist/internal/ai"
)

func (c Client) ExtractFacts(ctx context.Context, model string, in ai.FactsInput, options ai.FactsOptions) (ai.TextResult, error) {
	if err := in.Validate(); err != nil {
		return ai.TextResult{}, err
	}
	prompt := ai.FactsPrompt(options)
	var part any
	if in.Image != nil {
		part = imagePart(*in.Image)
	} else {
		part = map[string]string{"type": "input_file", "filename": "document.pdf", "file_data": "data:application/pdf;base64," + base64.StdEncoding.EncodeToString(in.Document.Data), "detail": "high"}
	}
	return c.imageContent(ctx, model, prompt, []any{map[string]string{"type": "input_text", "text": prompt.Input}, part})
}
