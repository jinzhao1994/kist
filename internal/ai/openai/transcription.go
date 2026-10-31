package openai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"mime/multipart"
	"net/textproto"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/jinzhao1994/kist/internal/ai"
)

// Transcribe uses the API account's file transcription endpoint. No upload is
// attempted for unsupported inputs, and a partial transcript is never saved.
func (c Client) Transcribe(ctx context.Context, model string, audio ai.Audio, options ai.TextOptions) (ai.TextResult, error) {
	if !c.APIKey {
		return ai.TextResult{}, errors.New("transcription requires an OpenAI API account")
	}
	if len(audio.Data) == 0 || len(audio.Data) > ai.MaxAudioBytes {
		return ai.TextResult{}, errors.New("audio exceeds the 25 MB transcription input limit")
	}
	name := filepath.Base(audio.Name)
	switch strings.ToLower(filepath.Ext(name)) {
	case ".flac", ".mp3", ".mp4", ".mpeg", ".mpga", ".m4a", ".ogg", ".wav", ".webm":
	default:
		return ai.TextResult{}, errors.New("unsupported audio format for transcription")
	}
	if model == "" || strings.ContainsAny(name, "\r\n") {
		return ai.TextResult{}, errors.New("invalid transcription input")
	}
	var body bytes.Buffer
	form := multipart.NewWriter(&body)
	if err := form.WriteField("model", model); err != nil {
		return ai.TextResult{}, err
	}
	if err := form.WriteField("response_format", "json"); err != nil {
		return ai.TextResult{}, err
	}
	if options.Supplement != "" {
		if err := form.WriteField("prompt", options.Supplement); err != nil {
			return ai.TextResult{}, err
		}
	}
	contentType := audio.MIME
	if _, _, err := mime.ParseMediaType(contentType); err != nil {
		contentType = "application/octet-stream"
	}
	header := textproto.MIMEHeader{}
	header.Set("Content-Disposition", mime.FormatMediaType("form-data", map[string]string{"name": "file", "filename": name}))
	header.Set("Content-Type", contentType)
	part, err := form.CreatePart(header)
	if err != nil {
		return ai.TextResult{}, err
	}
	if _, err = part.Write(audio.Data); err != nil {
		return ai.TextResult{}, err
	}
	if err = form.Close(); err != nil {
		return ai.TextResult{}, err
	}
	res, err := c.requestContent(ctx, "POST", "/audio/transcriptions", &body, form.FormDataContentType())
	if err != nil {
		return ai.TextResult{}, err
	}
	defer res.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(res.Body, 2*ai.MaxTextBytes+1))
	if err != nil || len(raw) > 2*ai.MaxTextBytes {
		return ai.TextResult{}, errors.New("invalid or oversized transcription response")
	}
	var result struct {
		Text *string `json:"text"`
	}
	if !utf8.Valid(raw) || json.Unmarshal(raw, &result) != nil || result.Text == nil || len(*result.Text) > ai.MaxTextBytes {
		return ai.TextResult{}, errors.New("invalid or oversized transcription response")
	}
	return ai.TextResult{Text: *result.Text, Provider: "openai", Model: model}, nil
}
