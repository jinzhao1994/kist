package ai

import (
	_ "embed"
	"encoding/json"
	"fmt"
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
func OCRPrompt(supplement string) Prompt {
	return withSupplement(Prompt{
		Instructions: strings.TrimSpace(ocrInstructions),
		Input:        strings.TrimSpace(ocrInput),
	}, supplement, ocrSupplement)
}

//go:embed prompts/description.txt
var descriptionInstructions string

//go:embed prompts/description-input.txt
var descriptionInput string

// DescriptionRecipeVersion identifies the description instructions used for new results.
const DescriptionRecipeVersion = 8

const OCRRecipeVersion = 4
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
func DescriptionPrompt(language, supplement string) Prompt {
	return withSupplement(Prompt{Instructions: strings.ReplaceAll(strings.TrimSpace(descriptionInstructions), "{{language}}", language), Input: strings.TrimSpace(descriptionInput)}, supplement, descriptionSupplement)
}

//go:embed prompts/ocr-supplement.txt
var ocrSupplement string

//go:embed prompts/description-supplement.txt
var descriptionSupplement string

func withSupplement(prompt Prompt, supplement, instructions string) Prompt {
	if strings.TrimSpace(supplement) == "" {
		return prompt
	}
	prompt.Instructions += "\n\n" + strings.TrimSpace(instructions)
	encoded, _ := json.Marshal(supplement)
	prompt.Input += "\n\nUser-provided reference information (JSON string):\n" + string(encoded)
	return prompt
}

//go:embed prompts/document-description.txt
var documentDescriptionInstructions string

// DocumentDescriptionPrompt describes the complete document within input limits.
func DocumentDescriptionPrompt(options TextOptions, pages int) Prompt {
	prompt := Prompt{Instructions: strings.ReplaceAll(strings.TrimSpace(documentDescriptionInstructions), "{{language}}", options.Language), Input: fmt.Sprintf("Describe the complete supplied document (%d pages). Consider the whole document, including its later sections and conclusions.", pages)}
	return withSupplement(prompt, options.Supplement, strings.ReplaceAll(descriptionSupplement, "the image", "the document"))
}
