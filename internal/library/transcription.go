package library

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/jinzhao1994/kist/internal/ai"
	"github.com/jinzhao1994/kist/internal/domain"
	"github.com/jinzhao1994/kist/internal/extract"
)

type transcriptionExtractor struct{}

func (transcriptionExtractor) ID() string             { return "transcription" }
func (transcriptionExtractor) Version() int           { return 1 }
func (transcriptionExtractor) Namespaces() []string   { return []string{"transcription"} }
func (transcriptionExtractor) Wants(mime string) bool { return strings.HasPrefix(mime, "audio/") }
func (transcriptionExtractor) Extract(context.Context, io.ReadSeeker, string) ([]extract.KV, error) {
	return nil, errors.New("transcription requires an explicit provider")
}

func supportedTranscription(name, mime string) bool {
	if !(transcriptionExtractor{}).Wants(mime) {
		return false
	}
	switch strings.ToLower(filepath.Ext(name)) {
	case ".flac", ".mp3", ".mpga", ".m4a", ".wav", ".ogg", ".webm":
		return true
	default:
		return false
	}
}

func (l *Library) TranscriptionQueue() *OCRQueue { return l.imageTextQueue("transcription") }

func (l *Library) prepareTranscription(ctx context.Context, id domain.FileID, provider ai.Provider, model, expectedBlob string, force bool) (*PreparedOCR, error) {
	p, zone, err := l.prepareAIState(ctx, id, "transcription")
	if err != nil {
		return nil, err
	}
	if expectedBlob != "" && p.file.BlobKey != expectedBlob {
		return nil, ErrOCRChanged
	}
	supplement := ""
	for _, row := range p.rows {
		if row.Key == "transcription:text" && row.Source == extract.UserSource {
			return p, nil
		}
		if row.Key == "transcription:hint" && !row.Tombstone && row.Source == extract.UserSource {
			supplement = row.Value
		}
	}
	if !force && p.version >= 1 {
		return p, nil
	}
	transcriber, ok := provider.(ai.Transcriber)
	if !ok {
		return nil, errors.New("this AI account does not support transcription")
	}
	path, ok := l.blobs.LocalPath(blobKey(p.file.BlobKey))
	if !ok {
		return nil, ErrNoSuchFile
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, ai.MaxAudioBytes+1))
	if err != nil {
		return nil, err
	}
	if len(data) > ai.MaxAudioBytes {
		return nil, errors.New("audio exceeds the 25 MB transcription input limit")
	}
	mime, err := extract.DetectMedia(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	if !supportedTranscription(p.file.Name, mime) {
		return nil, errors.New("unsupported audio file for transcription")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	result, err := transcriber.Transcribe(ctx, model, ai.Audio{Name: p.file.Name, MIME: mime, Data: data}, ai.TextOptions{Supplement: supplement})
	if err != nil {
		return nil, err
	}
	if err := captureModelResult(ctx, "transcription", result); err != nil {
		return nil, err
	}
	facts := []extract.KV{
		{Key: "transcription:hint", Value: "", Type: extract.TypeText},
		{Key: "transcription:text", Value: result.Text, Type: extract.TypeText},
		{Key: "transcription:model", Value: result.Provider + "/" + result.Model, Type: extract.TypeText},
		{Key: "transcription:recipe", Value: fmt.Sprint(1), Type: extract.TypeInt},
	}
	return p.finishAI(ctx, transcriptionExtractor{}, facts, zone)
}
