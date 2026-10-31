package ai

import "testing"

func TestFileTranscriptionModelCapabilities(t *testing.T) {
	input := []Model{{ID: "gpt-6-luna"}, {ID: "gpt-transcribe"}, {ID: "gpt-live-transcribe"}, {ID: "gpt-audio"}, {ID: "gpt-4o-mini-transcribe-2025-12-15"}, {ID: "gpt-4o-transcribe-diarize"}}
	filtered := ModelsForFeature("transcription", input)
	if len(filtered) != 2 || filtered[0].ID != DefaultTranscriptionModel || filtered[1].ID != "gpt-4o-mini-transcribe-2025-12-15" {
		t.Fatal(filtered)
	}
	if len(ModelsForFeature("ocr", input)) != len(input) {
		t.Fatal("unrelated feature catalog filtered")
	}
	if FeatureModel("transcription", Feature{}) != DefaultTranscriptionModel || FeatureModel("ocr", Feature{}) != "" {
		t.Fatal("incorrect default")
	}
	for _, name := range []string{"gpt-6-luna", "gpt-live-transcribe", "gpt-audio", "gpt-4o-transcribe-diarize", "gpt-transcribe-arbitrary"} {
		if SupportedTranscriptionModel(name) {
			t.Fatal("unsupported API contract accepted", name)
		}
	}
}
