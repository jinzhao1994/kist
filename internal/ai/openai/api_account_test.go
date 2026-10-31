package openai

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jinzhao1994/kist/internal/ai"
)

func TestAPIAccountCatalogAndIndependentParameters(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if r.Header.Get("Authorization") != "Bearer synthetic-api-key" {
			t.Error("missing account key")
		}
		if r.URL.Path == "/models" {
			io.WriteString(w, `{"data":[{"id":"synthetic-vision"},{"id":"synthetic-audio"}]}`)
			return
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
			return
		}
		if body["model"] != "synthetic-vision" {
			t.Error("wrong model")
		}
		content := body["input"].([]any)[0].(map[string]any)["content"].([]any)
		image := content[1].(map[string]any)
		if requests == 2 {
			if body["reasoning"].(map[string]any)["effort"] != "high" || image["detail"] != "low" {
				t.Error("lost feature parameters")
			}
		} else if body["reasoning"] != nil || image["detail"] != nil {
			t.Error("default controls should be omitted")
		}
		io.WriteString(w, "data: {\"type\":\"response.output_text.done\",\"text\":\"Synthetic text\"}\n\ndata: {\"type\":\"response.completed\",\"response\":{\"status\":\"completed\"}}\n")
	}))
	defer server.Close()
	client := Client{APIKey: true, Token: "synthetic-api-key", BaseURL: server.URL, HTTP: server.Client(), Parameters: &ai.ModelParameters{ReasoningEffort: "high", ImageDetail: "low"}}
	models, err := client.Models(context.Background())
	if err != nil || len(models) != 2 || models[0].ID != "synthetic-vision" {
		t.Fatal(models, err)
	}
	result, err := client.OCR(context.Background(), "synthetic-vision", ai.Image{MIME: "image/png", Data: []byte("synthetic")}, ai.TextOptions{})
	if err != nil || result.Text != "Synthetic text" {
		t.Fatal(result, err)
	}
	client.Parameters = &ai.ModelParameters{}
	if _, err := client.Describe(context.Background(), "synthetic-vision", ai.Image{MIME: "image/png", Data: []byte("synthetic")}, ai.TextOptions{}); err != nil {
		t.Fatal(err)
	}
}

func TestAPITranscriptionMultipartAndCompleteResult(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/audio/transcriptions" || r.Header.Get("Authorization") != "Bearer synthetic-api-key" {
			t.Error("wrong route or credentials")
		}
		if err := r.ParseMultipartForm(1 << 20); err != nil {
			t.Error(err)
			return
		}
		defer r.MultipartForm.RemoveAll()
		if r.FormValue("model") != "gpt-transcribe" || r.FormValue("prompt") != "Synthetic terms" || r.FormValue("response_format") != "json" {
			t.Error("lost transcription fields")
		}
		if r.FormValue("reasoning") != "" || r.FormValue("detail") != "" {
			t.Error("image controls escaped into audio request")
		}
		file, header, err := r.FormFile("file")
		if err != nil {
			t.Error(err)
			return
		}
		defer file.Close()
		data, _ := io.ReadAll(file)
		if header.Filename != "sample.wav" || header.Header.Get("Content-Type") != "audio/wav" || string(data) != "RIFF synthetic recording" {
			t.Error("invalid uploaded audio")
		}
		io.WriteString(w, `{"text":"A synthetic transcript."}`)
	}))
	defer server.Close()
	client := Client{APIKey: true, Token: "synthetic-api-key", BaseURL: server.URL, HTTP: server.Client()}
	result, err := client.Transcribe(context.Background(), "gpt-transcribe", ai.Audio{Name: "sample.wav", MIME: "audio/wav", Data: []byte("RIFF synthetic recording")}, ai.TextOptions{Supplement: "Synthetic terms"})
	if err != nil || result.Text != "A synthetic transcript." || result.Model != "gpt-transcribe" {
		t.Fatal(result, err)
	}
}

func TestTranscriptionRejectsUnsupportedInputAndInvalidResponses(t *testing.T) {
	calls := 0
	response := "{}"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls++; io.WriteString(w, response) }))
	defer server.Close()
	client := Client{APIKey: true, Token: "synthetic-api-key", BaseURL: server.URL, HTTP: server.Client()}
	good := ai.Audio{Name: "test.wav", MIME: "audio/wav", Data: []byte("synthetic")}
	for _, audio := range []ai.Audio{{Name: "file.txt", Data: []byte("synthetic")}, {Name: "test.wav"}, {Name: "test.wav", Data: make([]byte, ai.MaxAudioBytes+1)}} {
		if _, err := client.Transcribe(context.Background(), "gpt-transcribe", audio, ai.TextOptions{}); err == nil {
			t.Fatal("accepted unsupported input")
		}
	}
	for _, model := range []string{"gpt-6-luna", "gpt-live-transcribe", "gpt-4o-transcribe-diarize"} {
		if _, err := client.Transcribe(context.Background(), model, good, ai.TextOptions{}); err == nil {
			t.Fatal("incompatible model accepted")
		}
	}
	login := client
	login.APIKey = false
	if _, err := login.Transcribe(context.Background(), "gpt-transcribe", good, ai.TextOptions{}); err == nil {
		t.Fatal("accepted ChatGPT account")
	}
	if calls != 0 {
		t.Fatal("invalid input caused an upload")
	}
	for _, value := range []string{"{}", `{"text":null}`, `{"text":1}`, `{"text":"partial"`, strings.Repeat("x", 2*ai.MaxTextBytes+1)} {
		response = value
		if result, err := client.Transcribe(context.Background(), "gpt-transcribe", good, ai.TextOptions{}); err == nil || result.Text != "" {
			t.Fatal("incomplete response escaped")
		}
	}
	response = `{"text":""}`
	if _, err := client.Transcribe(context.Background(), "gpt-transcribe", good, ai.TextOptions{}); err != nil {
		t.Fatal("valid silence rejected", err)
	}
}
