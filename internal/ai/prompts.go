package ai

import (
	_ "embed"
	"strings"
)

// Prompt describes a task independently of a provider's wire format.
type Prompt struct {
	Instructions string
	Input        string
}

//go:embed prompts/ocr.txt
var ocrInstructions string

//go:embed prompts/ocr-input.txt
var ocrInput string

// OCRPrompt returns the shared verbatim text extraction task.
func OCRPrompt() Prompt {
	return Prompt{
		Instructions: strings.TrimSpace(ocrInstructions),
		Input:        strings.TrimSpace(ocrInput),
	}
}
