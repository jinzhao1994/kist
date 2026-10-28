package openai

import (
	"encoding/json"
	"github.com/jinzhao1994/kist/internal/ai"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestOCRRequiresCompletedStream(t *testing.T) {
	cases := []struct {
		name, stream string
		ok           bool
	}{
		{"complete", "data: {\"type\":\"response.output_text.delta\",\"delta\":\"姓名：张三\\n号码：001\"}\n\ndata: {\"type\":\"response.completed\",\"response\":{\"status\":\"completed\"}}\n", true},
		{"done text without deltas", "data: {\"type\":\"response.output_text.done\",\"text\":\"full text\"}\n\ndata: {\"type\":\"response.completed\",\"response\":{\"status\":\"completed\"}}\n", true},
		{"missing text", "data: {\"type\":\"response.completed\",\"response\":{\"status\":\"completed\"}}\n", false},
		{"cut off", "data: {\"type\":\"response.output_text.delta\",\"delta\":\"partial\"}\n", false},
		{"incomplete", "data: {\"type\":\"response.incomplete\"}\n", false},
		{"refused", "data: {\"type\":\"response.refusal.delta\"}\n", false},
		{"failed", "data: {\"type\":\"response.failed\"}\n", false},
		{"done only", "data: [DONE]\n", false},
		{"oversize", "data: {\"type\":\"response.output_text.delta\",\"delta\":\"" + strings.Repeat("a", ai.MaxTextBytes+1) + "\"}\n", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			text, err := readText(strings.NewReader(tc.stream))
			if (err == nil) != tc.ok {
				t.Fatal(text, err)
			}
			if err != nil && text != "" {
				t.Fatal("partial output escaped")
			}
		})
	}
}
func TestOCRWireAndModelCatalog(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-token" {
			t.Error("missing bearer")
		}
		if r.URL.Path == "/models" {
			w.Write([]byte(`{"models":[{"slug":"gpt-6-luna","display_name":"Luna","visibility":"list"},{"slug":"hidden","visibility":"hidden"}]}`))
			return
		}
		var body map[string]any
		if json.NewDecoder(r.Body).Decode(&body) != nil {
			t.Fatal("bad request")
		}
		if body["store"] != false || body["stream"] != true || body["model"] != "gpt-6-luna" {
			t.Error(body)
		}
		imageInput := body["input"].([]any)[0].(map[string]any)["content"].([]any)[1].(map[string]any)
		if imageInput["detail"] != "original" {
			t.Errorf("image detail = %v, want original", imageInput["detail"])
		}
		prompt := ai.OCRPrompt("")
		if prompt.Instructions == "" || body["instructions"] != prompt.Instructions {
			t.Error("OCR instructions did not come from the shared task prompt")
		}
		input := body["input"].([]any)[0].(map[string]any)
		content := input["content"].([]any)
		if prompt.Input == "" || content[0].(map[string]any)["text"] != prompt.Input {
			t.Error("OCR input did not come from the shared task prompt")
		}
		w.Write([]byte("data: {\"type\":\"response.output_text.delta\",\"delta\":\"hello\"}\n\ndata: {\"type\":\"response.completed\",\"response\":{\"status\":\"completed\"}}\n"))
	}))
	defer server.Close()
	c := Client{BaseURL: server.URL, Token: "test-token"}
	models, err := c.Models(t.Context())
	if err != nil || len(models) != 1 {
		t.Fatal(models, err)
	}
	out, err := c.OCR(t.Context(), models[0].ID, ai.Image{MIME: "image/png", Data: []byte("image")}, ai.TextOptions{})
	if err != nil || out.Text != "hello" || out.Model != "gpt-6-luna" {
		t.Fatal(out, err)
	}
}

func TestDescriptionWireUsesIndependentLocalizedPrompt(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
			return
		}
		imageInput := body["input"].([]any)[0].(map[string]any)["content"].([]any)[1].(map[string]any)
		if imageInput["detail"] != "original" {
			t.Errorf("image detail = %v, want original", imageInput["detail"])
		}
		prompt := ai.DescriptionPrompt("zh-CN", "")
		if body["instructions"] != prompt.Instructions || !strings.Contains(prompt.Instructions, "zh-CN") || strings.Contains(prompt.Instructions, "{{language}}") || prompt.Instructions == ai.OCRPrompt("").Instructions {
			t.Error("wrong description prompt")
		}
		if body["store"] != false || body["stream"] != true {
			t.Error("unexpected request policy")
		}
		w.Write([]byte("data: {\"type\":\"response.output_text.done\",\"text\":\"一张测试文档。\"}\n\ndata: {\"type\":\"response.completed\",\"response\":{\"status\":\"completed\"}}\n"))
	}))
	defer server.Close()
	c := Client{BaseURL: server.URL, Token: "test-token"}
	out, err := c.Describe(t.Context(), "test", ai.Image{MIME: "image/png", Data: []byte("image")}, ai.TextOptions{Language: "zh-CN"})
	if err != nil || out.Text != "一张测试文档。" {
		t.Fatal(out, err)
	}
}

func TestOrientationWireFourCandidates(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
			return
		}
		if body["instructions"] != ai.OrientationPrompt(true).Instructions || body["reasoning"].(map[string]any)["effort"] != "low" {
			t.Error("wrong orientation prompt or effort")
		}
		content := body["input"].([]any)[0].(map[string]any)["content"].([]any)
		if len(content) != 9 {
			t.Error("missing four labeled images")
			return
		}
		for i := range 4 {
			if content[1+i*2].(map[string]any)["text"] != "Image "+string(rune('A'+i)) || content[2+i*2].(map[string]any)["detail"] != "original" {
				t.Error("candidate order or detail")
			}
		}
		raw, _ := json.Marshal(map[string]any{"type": "response.output_text.done", "text": `{"decision":"C","confidence":0.98,"evidence":"text"}`})
		w.Write([]byte("data: " + string(raw) + "\n\ndata: {\"type\":\"response.completed\",\"response\":{\"status\":\"completed\"}}\n"))
	}))
	defer server.Close()
	c := Client{BaseURL: server.URL, Token: "test"}
	images := make([]ai.Image, 4)
	for i := range images {
		images[i] = ai.Image{MIME: "image/png", Data: []byte("synthetic")}
	}
	got, err := c.ChooseUpright(t.Context(), "test", images, true)
	if err != nil || got.Index != 2 || got.Provider != "openai" || got.Model != "test" {
		t.Fatal(got, err)
	}
}

func TestHintsReachImageRequestAsReferenceData(t *testing.T) {
	for _, kind := range []string{"ocr", "desc"} {
		t.Run(kind, func(t *testing.T) {
			const hint = "Example Person"
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var body map[string]any
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Fatal(err)
				}
				content := body["input"].([]any)[0].(map[string]any)["content"].([]any)
				text := content[0].(map[string]any)["text"].(string)
				if !strings.Contains(text, `"Example Person"`) || strings.Contains(body["instructions"].(string), hint) {
					t.Fatal("reference crossed instruction boundary")
				}
				if content[1].(map[string]any)["detail"] != "original" || body["reasoning"].(map[string]any)["effort"] != "low" {
					t.Fatal("image quality or effort changed")
				}
				w.Write([]byte("data: {\"type\":\"response.output_text.done\",\"text\":\"Synthetic result\"}\n\ndata: {\"type\":\"response.completed\",\"response\":{\"status\":\"completed\"}}\n"))
			}))
			defer server.Close()
			c := Client{BaseURL: server.URL}
			options := ai.TextOptions{Language: "zh-CN", Supplement: hint}
			img := ai.Image{MIME: "image/png", Data: []byte("synthetic")}
			var err error
			if kind == "ocr" {
				_, err = c.OCR(t.Context(), "test", img, options)
			} else {
				_, err = c.Describe(t.Context(), "test", img, options)
			}
			if err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestDocumentDescriptionInlinePDF(t *testing.T) {
	options := ai.TextOptions{Language: "zh-CN", Supplement: "Example Person"}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
			return
		}
		prompt := ai.DocumentDescriptionPrompt(options, 10, 28)
		if body["instructions"] != prompt.Instructions || body["store"] != false || body["stream"] != true || body["max_output_tokens"] != nil {
			t.Error("wrong request policy")
		}
		content := body["input"].([]any)[0].(map[string]any)["content"].([]any)
		if content[0].(map[string]any)["text"] != prompt.Input || !strings.Contains(prompt.Input, "Example Person") {
			t.Error("missing reference data")
		}
		file := content[1].(map[string]any)
		if file["type"] != "input_file" || file["filename"] != "document.pdf" || file["file_data"] != "data:application/pdf;base64,JVBERi0=" || file["detail"] != "high" {
			t.Error(file)
		}
		w.Write([]byte("data: {\"type\":\"response.output_text.done\",\"text\":\"Synthetic description\"}\n\ndata: {\"type\":\"response.completed\",\"response\":{\"status\":\"completed\"}}\n"))
	}))
	defer server.Close()
	c := Client{BaseURL: server.URL, Token: "test-token"}
	result, err := c.DescribeDocument(t.Context(), "test", ai.Document{Data: []byte("%PDF-"), Pages: 10, TotalPages: 28}, options)
	if err != nil || result.Text != "Synthetic description" {
		t.Fatal(result, err)
	}
	for _, doc := range []ai.Document{{}, {Data: []byte("%PDF-"), Pages: 11, TotalPages: 11}, {Data: []byte("%PDF-"), Pages: 2, TotalPages: 1}} {
		if _, err := c.DescribeDocument(t.Context(), "test", doc, options); err == nil {
			t.Fatal("accepted invalid document")
		}
	}
}
