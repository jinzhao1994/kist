// Package openai adapts ChatGPT plan authorization and Responses inference.
package openai

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jinzhao1994/kist/internal/ai"
)

const APIURL = "https://api.openai.com/v1"
const Issuer = "https://auth.openai.com"
const Resource = APIURL

// Client is immutable and scoped to a freshly loaded access token.
type Client struct {
	Token   string
	BaseURL string
	HTTP    *http.Client
}

func (c Client) http() *http.Client {
	if c.HTTP != nil {
		return c.HTTP
	}
	return &http.Client{Timeout: 3 * time.Minute, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
}
func (c Client) request(ctx context.Context, method, path string, body io.Reader) (*http.Response, error) {
	base := c.BaseURL
	if base == "" {
		base = APIURL
	}
	req, err := http.NewRequestWithContext(ctx, method, base+path, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.Token)
	req.Header.Set("Content-Type", "application/json")
	res, err := c.http().Do(req)
	if err != nil {
		return nil, errors.New("OpenAI request failed; check connection and try again")
	}
	if res.StatusCode != http.StatusOK {
		res.Body.Close()
		return nil, fmt.Errorf("OpenAI returned HTTP %d; check account access and usage", res.StatusCode)
	}
	return res, nil
}
func (c Client) Models(ctx context.Context) ([]ai.Model, error) {
	res, err := c.request(ctx, "GET", "/models", nil)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	var body struct {
		Models []struct {
			Slug       string `json:"slug"`
			Name       string `json:"display_name"`
			Visibility string `json:"visibility"`
		} `json:"models"`
	}
	if err = json.NewDecoder(io.LimitReader(res.Body, 2<<20)).Decode(&body); err != nil {
		return nil, errors.New("invalid OpenAI model catalog")
	}
	out := []ai.Model{}
	for _, m := range body.Models {
		if m.Visibility == "list" && m.Slug != "" {
			out = append(out, ai.Model{ID: m.Slug, Name: m.Name})
		}
	}
	return out, nil
}
func (c Client) OCR(ctx context.Context, model string, img ai.Image, options ai.TextOptions) (ai.OCRResult, error) {
	return c.imageText(ctx, model, img, ai.OCRPrompt(options.Supplement))
}
func (c Client) Describe(ctx context.Context, model string, img ai.Image, options ai.TextOptions) (ai.TextResult, error) {
	return c.imageText(ctx, model, img, ai.DescriptionPrompt(options.Language, options.Supplement))
}
func (c Client) imageText(ctx context.Context, model string, img ai.Image, prompt ai.Prompt) (ai.TextResult, error) {
	if len(img.Data) == 0 || len(img.Data) > ai.MaxImageBytes {
		return ai.OCRResult{}, errors.New("image exceeds AI input limit")
	}
	content := []any{map[string]string{"type": "input_text", "text": prompt.Input}, imagePart(img)}
	return c.imageContent(ctx, model, prompt, content)
}

func (c Client) DescribeDocument(ctx context.Context, model string, document ai.Document, options ai.TextOptions) (ai.TextResult, error) {
	if len(document.Data) == 0 || len(document.Data) > ai.MaxDocumentBytes || document.Pages < 1 || document.Pages > ai.MaxDocumentPages || document.TotalPages < document.Pages {
		return ai.TextResult{}, errors.New("document exceeds AI input limits")
	}
	prompt := ai.DocumentDescriptionPrompt(options, document.Pages, document.TotalPages)
	content := []any{map[string]string{"type": "input_text", "text": prompt.Input}, map[string]string{"type": "input_file", "filename": "document.pdf", "file_data": "data:application/pdf;base64," + base64.StdEncoding.EncodeToString(document.Data), "detail": "high"}}
	return c.imageContent(ctx, model, prompt, content)
}

func imagePart(img ai.Image) map[string]string {
	return map[string]string{"type": "input_image", "image_url": "data:" + img.MIME + ";base64," + base64.StdEncoding.EncodeToString(img.Data), "detail": "original"}
}

func (c Client) ChooseUpright(ctx context.Context, model string, images []ai.Image, overview bool) (ai.UprightChoice, error) {
	if len(images) != 4 {
		return ai.UprightChoice{}, errors.New("orientation requires four candidates")
	}
	prompt := ai.OrientationPrompt(overview)
	content := []any{map[string]string{"type": "input_text", "text": prompt.Input}}
	for i, img := range images {
		if len(img.Data) == 0 || len(img.Data) > ai.MaxImageBytes {
			return ai.UprightChoice{}, errors.New("image exceeds AI input limit")
		}
		content = append(content, map[string]string{"type": "input_text", "text": fmt.Sprintf("Image %c", 'A'+i)}, imagePart(img))
	}
	result, err := c.imageContent(ctx, model, prompt, content)
	if err != nil {
		return ai.UprightChoice{}, err
	}
	choice, err := ai.ParseUprightChoice(result.Text)
	choice.Provider, choice.Model = result.Provider, result.Model
	return choice, err
}

func (c Client) imageContent(ctx context.Context, model string, prompt ai.Prompt, content []any) (ai.TextResult, error) {
	body := map[string]any{"model": model, "store": false, "stream": true, "reasoning": map[string]string{"effort": "low"}, "instructions": prompt.Instructions, "input": []any{map[string]any{"role": "user", "content": content}}}
	raw, err := json.Marshal(body)
	if err != nil {
		return ai.OCRResult{}, err
	}
	res, err := c.request(ctx, "POST", "/responses", bytes.NewReader(raw))
	if err != nil {
		return ai.OCRResult{}, err
	}
	defer res.Body.Close()
	text, err := readText(res.Body)
	return ai.OCRResult{Text: text, Provider: "openai", Model: model}, err
}
func readText(r io.Reader) (string, error) {
	scanner := bufio.NewScanner(io.LimitReader(r, 8<<20))
	scanner.Buffer(make([]byte, 4096), 2<<20)
	parts := map[[2]int]string{}
	total := 0
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if data == "[DONE]" {
			break
		}
		var event struct {
			Type         string  `json:"type"`
			Delta        string  `json:"delta"`
			Text         *string `json:"text"`
			OutputIndex  int     `json:"output_index"`
			ContentIndex int     `json:"content_index"`
			Response     struct {
				Status string `json:"status"`
			} `json:"response"`
		}
		if json.Unmarshal([]byte(data), &event) != nil {
			return "", errors.New("invalid OpenAI response stream")
		}
		key := [2]int{event.OutputIndex, event.ContentIndex}
		switch event.Type {
		case "response.output_text.delta", "response.output_text.done":
			previous := parts[key]
			next := previous + event.Delta
			if event.Type == "response.output_text.done" {
				if event.Text == nil {
					return "", errors.New("invalid completed image extraction text")
				}
				next = *event.Text
			}
			total += len(next) - len(previous)
			if total > ai.MaxTextBytes || len(parts) > 1024 {
				return "", errors.New("Image extraction text exceeds 256 KiB; no result was saved")
			}
			parts[key] = next
		case "response.refusal.delta", "response.refusal.done", "response.failed", "response.incomplete", "error":
			return "", errors.New("OpenAI did not complete image extraction; no result was saved")
		case "response.completed":
			if event.Response.Status != "completed" || len(parts) == 0 {
				return "", errors.New("OpenAI completed without Image extraction text; no result was saved")
			}
			keys := make([][2]int, 0, len(parts))
			for k := range parts {
				keys = append(keys, k)
			}
			sort.Slice(keys, func(i, j int) bool {
				if keys[i][0] != keys[j][0] {
					return keys[i][0] < keys[j][0]
				}
				return keys[i][1] < keys[j][1]
			})
			var text strings.Builder
			for _, k := range keys {
				text.WriteString(parts[k])
			}
			if !utf8.ValidString(text.String()) {
				return "", errors.New("invalid completed image extraction response")
			}
			return text.String(), nil
		}
	}
	return "", errors.New("Image extraction stream interrupted; no result was saved")
}
