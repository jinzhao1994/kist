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

// DescriptionRecipeVersion identifies the description instructions used for new results.
const DescriptionRecipeVersion = 5

const OCRRecipeVersion = 3
const OrientationRecipeVersion = 1

//go:embed prompts/orientation.txt
var orientationInstructions string

// OrientationPrompt asks only for an already upright candidate, never an angle.
func OrientationPrompt(overview bool) Prompt {
	input := "These are four versions of a local region cropped from a larger image."
	if overview {
		input = "These are four versions of an overview of the entire image."
	}
	return Prompt{Instructions: strings.TrimSpace(orientationInstructions), Input: input}
}

// DescriptionPrompt describes visible content in the configured archive language.
func DescriptionPrompt(language string) Prompt {
	return Prompt{Instructions: strings.ReplaceAll(strings.TrimSpace(descriptionInstructions), "{{language}}", language), Input: strings.TrimSpace(descriptionInput)}
}
