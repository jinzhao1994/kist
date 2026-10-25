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
		w.Write([]byte("data: {\"type\":\"response.output_text.delta\",\"delta\":\"hello\"}\n\ndata: {\"type\":\"response.completed\",\"response\":{\"status\":\"completed\"}}\n"))
	}))
	defer server.Close()
	c := Client{BaseURL: server.URL, Token: "test-token"}
	models, err := c.Models(t.Context())
	if err != nil || len(models) != 1 {
		t.Fatal(models, err)
	}
	out, err := c.OCR(t.Context(), models[0].ID, ai.Image{MIME: "image/png", Data: []byte("image")})
	if err != nil || out.Text != "hello" || out.Model != "gpt-6-luna" {
		t.Fatal(out, err)
	}
}
