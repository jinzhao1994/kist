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

//go:embed prompts/description.txt
var descriptionInstructions string

//go:embed prompts/description-input.txt
var descriptionInput string

// DescriptionPrompt describes visible content in the configured archive language.
func DescriptionPrompt(language string) Prompt {
	return Prompt{Instructions: strings.ReplaceAll(strings.TrimSpace(descriptionInstructions), "{{language}}", language), Input: strings.TrimSpace(descriptionInput)}
}
