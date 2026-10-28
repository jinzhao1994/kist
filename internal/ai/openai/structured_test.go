package openai

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jinzhao1994/kist/internal/ai"
	"github.com/jinzhao1994/kist/internal/structured"
)

func TestStructuredInferenceUsesOriginalSourceAndSharedTransport(t *testing.T) {
	for _, document := range []bool{false, true} {
		t.Run(fmt.Sprint(document), func(t *testing.T) {
			options := ai.FactsOptions{TextOptions: ai.TextOptions{Language: "zh-CN", Supplement: "Unique reference 728"}, Outline: `[{"key":"holder","label":"持有人","type":"object"}]`}
			calls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				if r.URL.Path != "/responses" || r.Header.Get("Authorization") != "Bearer test" {
					t.Error("wrong transport")
				}
				var body struct {
					Store        bool   `json:"store"`
					Stream       bool   `json:"stream"`
					Instructions string `json:"instructions"`
					Input        []struct {
						Content []map[string]string `json:"content"`
					} `json:"input"`
				}
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Error(err)
					return
				}
				if body.Store || !body.Stream || !strings.Contains(body.Instructions, "zh-CN") || strings.Contains(body.Instructions, options.Supplement) {
					t.Error("wrong instruction boundary")
				}
				content := body.Input[0].Content
				if len(content) != 2 || !strings.Contains(content[0]["text"], options.Outline) || !strings.Contains(content[0]["text"], options.Supplement) {
					t.Error("missing context")
				}
				if document {
					if content[1]["type"] != "input_file" || content[1]["detail"] != "high" || content[1]["file_data"] != "data:application/pdf;base64,JVBERi0=" {
						t.Error("document source changed")
					}
				} else if content[1]["type"] != "input_image" || content[1]["detail"] != "original" || content[1]["image_url"] != "data:image/png;base64,aW1hZ2U=" {
					t.Error("image source changed")
				}
				text := `{"fields":[{"key":"name","label":"姓名","type":"text","value":"Example Person"}]}`
				payload, _ := json.Marshal(map[string]string{"type": "response.output_text.done", "text": text})
				fmt.Fprintf(w, "data: %s\n\ndata: {\"type\":\"response.completed\",\"response\":{\"status\":\"completed\"}}\n", payload)
			}))
			defer server.Close()
			in := ai.FactsInput{Image: &ai.Image{MIME: "image/png", Data: []byte("image")}}
			if document {
				in = ai.FactsInput{Document: &ai.Document{Data: []byte("%PDF-"), Pages: 200, TotalPages: 200}}
			}
			c := Client{BaseURL: server.URL, Token: "test"}
			result, err := c.ExtractFacts(t.Context(), "test", in, options)
			if err != nil || result.Provider != "openai" {
				t.Fatal(err)
			}
			if _, err := structured.ParseModel(result.Text); err != nil {
				t.Fatal(err)
			}
			for _, bad := range []ai.FactsInput{{}, {Image: in.Image, Document: &ai.Document{}}, {Document: &ai.Document{Data: []byte("%PDF-"), Pages: 10, TotalPages: 200}}, {Document: &ai.Document{Data: []byte("%PDF-"), Pages: 201, TotalPages: 201}}} {
				if _, err := c.ExtractFacts(t.Context(), "test", bad, options); err == nil {
					t.Fatal("invalid input accepted")
				}
			}
			if calls != 1 {
				t.Fatal("invalid input was transmitted")
			}
		})
	}
}
