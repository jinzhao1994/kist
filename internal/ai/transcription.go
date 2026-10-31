package ai

import "slices"

const DefaultTranscriptionModel = "gpt-transcribe"

// TranscriptionModels supports the file endpoint's plain JSON text contract and
// optional prompt. Realtime, speech generation and diarization are separate APIs.
// The legacy models remain usable until the provider's 2027-02-26 shutdown.
func TranscriptionModels() []string {
	return []string{DefaultTranscriptionModel, "gpt-4o-transcribe", "gpt-4o-mini-transcribe", "gpt-4o-mini-transcribe-2025-12-15", "whisper-1"}
}

func SupportedTranscriptionModel(model string) bool {
	return slices.Contains(TranscriptionModels(), model)
}

func FeatureModel(kind string, feature Feature) string {
	if kind == "transcription" && feature.Model == "" {
		return DefaultTranscriptionModel
	}
	return feature.Model
}

func ModelsForFeature(kind string, models []Model) []Model {
	if kind != "transcription" {
		return models
	}
	out := []Model{}
	for _, model := range models {
		if SupportedTranscriptionModel(model.ID) {
			out = append(out, model)
		}
	}
	return out
}
