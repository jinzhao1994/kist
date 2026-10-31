package library

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/jinzhao1994/kist/internal/ai"
	"github.com/jinzhao1994/kist/internal/derive"
	"github.com/jinzhao1994/kist/internal/domain"
	"github.com/jinzhao1994/kist/internal/extract"
)

type transcriptionExtractor struct{}

func (transcriptionExtractor) ID() string           { return "transcription" }
func (transcriptionExtractor) Version() int         { return 1 }
func (transcriptionExtractor) Namespaces() []string { return []string{"transcription"} }
func (transcriptionExtractor) Wants(mime string) bool {
	return strings.HasPrefix(mime, "audio/") || strings.HasPrefix(mime, "video/")
}
func (transcriptionExtractor) Extract(context.Context, io.ReadSeeker, string) ([]extract.KV, error) {
	return nil, errors.New("transcription requires an explicit provider")
}

func supportedTranscription(name string, mime string) bool {
	if (transcriptionExtractor{}).Wants(mime) || mime == "application/ogg" {
		return true
	}
	// The byte sniffer does not identify every finite FFmpeg container. These
	// candidates still require the restricted audio-stream probe before upload.
	if mime != "application/octet-stream" {
		return false
	}
	switch strings.ToLower(filepath.Ext(name)) {
	case ".mp4", ".mov", ".mkv", ".m4v", ".avi", ".mpeg", ".mpg", ".wmv", ".flv", ".ts", ".mts", ".m2ts", ".aac", ".wma":
		return true
	default:
		return false
	}
}

func directTranscriptionAudio(name, mime string) bool {
	if !strings.HasPrefix(mime, "audio/") {
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
	result, err := transcribeMedia(ctx, transcriber, model, path, p.file.Name, supplement, derive.Renderer{})
	if err != nil {
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

type transcriptionAudioRenderer interface {
	TranscriptionDuration(context.Context, string) (float64, error)
	RenderTranscriptionAudio(context.Context, string, string, float64, float64) error
}

func transcriptionAudio(path string) ([]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, ai.MaxAudioBytes+1))
	if err != nil {
		return nil, err
	}
	if len(data) == 0 || len(data) > ai.MaxAudioBytes {
		return nil, errors.New("audio exceeds the 25 MB transcription input limit")
	}
	return data, nil
}

// All segments must finish before returning a committable result. Partial model
// output may appear in the attempt diagnostic, never in archive metadata.
func transcribeMedia(ctx context.Context, provider ai.Transcriber, model, path, name, hint string, renderer transcriptionAudioRenderer) (ai.TextResult, error) {
	ctx, cancel := context.WithTimeout(ctx, 45*time.Minute)
	defer cancel()
	file, err := os.Open(path)
	if err != nil {
		return ai.TextResult{}, err
	}
	stat, err := file.Stat()
	if err != nil {
		file.Close()
		return ai.TextResult{}, err
	}
	mime, err := extract.DetectMedia(file)
	file.Close()
	if err != nil {
		return ai.TextResult{}, err
	}
	if !supportedTranscription(name, mime) {
		return ai.TextResult{}, errors.New("unsupported audio or video file for transcription")
	}
	transcribe := func(name, mime, path string) (ai.TextResult, error) {
		if err := ctx.Err(); err != nil {
			return ai.TextResult{}, err
		}
		data, err := transcriptionAudio(path)
		if err != nil {
			return ai.TextResult{}, err
		}
		return provider.Transcribe(ctx, model, ai.Audio{Name: name, MIME: mime, Data: data}, ai.TextOptions{Supplement: hint})
	}
	if stat.Size() <= ai.MaxAudioBytes && directTranscriptionAudio(name, mime) {
		result, err := transcribe(name, mime, path)
		if err == nil {
			err = captureModelResult(ctx, "transcription", result)
		}
		return result, err
	}
	duration, err := renderer.TranscriptionDuration(ctx, path)
	if err != nil {
		return ai.TextResult{}, err
	}
	if math.IsNaN(duration) || math.IsInf(duration, 0) || duration <= 0 || duration > derive.MaxTranscriptionSeconds {
		return ai.TextResult{}, errors.New("invalid or excessive audio duration")
	}
	dir, err := os.MkdirTemp("", "kist-transcription-")
	if err != nil {
		return ai.TextResult{}, err
	}
	defer os.RemoveAll(dir)
	output := filepath.Join(dir, "audio.wav")
	var result ai.TextResult
	for start := float64(0); start < duration; start += derive.TranscriptionSegmentSeconds {
		segment := int(start/derive.TranscriptionSegmentSeconds) + 1
		if err := renderer.RenderTranscriptionAudio(ctx, path, output, start, math.Min(derive.TranscriptionSegmentSeconds, duration-start)); err != nil {
			return ai.TextResult{}, fmt.Errorf("prepare transcription segment %d: %w", segment, err)
		}
		part, err := transcribe("audio.wav", "audio/wav", output)
		if err != nil {
			return ai.TextResult{}, fmt.Errorf("transcription segment %d failed: %w", segment, err)
		}
		if result.Provider != "" && (part.Provider != result.Provider || part.Model != result.Model) {
			return ai.TextResult{}, errors.New("transcription provider changed between segments")
		}
		result.Provider, result.Model = part.Provider, part.Model
		if result.Text != "" && part.Text != "" {
			result.Text += "\n"
		}
		result.Text += part.Text
		if err := captureModelResult(ctx, "transcription", result); err != nil {
			return ai.TextResult{}, err
		}
		if len(result.Text) > ai.MaxTextBytes {
			return ai.TextResult{}, errors.New("transcript exceeds the output limit; no partial result saved")
		}
	}
	return result, nil
}
