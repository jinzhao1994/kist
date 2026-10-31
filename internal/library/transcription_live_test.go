package library

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/jinzhao1994/kist/internal/ai"
	"github.com/jinzhao1994/kist/internal/config"
)

type liveTranscriptionUpload struct {
	Name   string `json:"name"`
	MIME   string `json:"mime"`
	Bytes  int    `json:"bytes"`
	SHA256 string `json:"sha256"`
}
type liveTranscriptionProvider struct {
	ai.Provider
	transcriber ai.Transcriber
	uploads     []liveTranscriptionUpload
}

func (p *liveTranscriptionProvider) Transcribe(ctx context.Context, model string, audio ai.Audio, options ai.TextOptions) (ai.TextResult, error) {
	sum := sha256.Sum256(audio.Data)
	p.uploads = append(p.uploads, liveTranscriptionUpload{Name: audio.Name, MIME: audio.MIME, Bytes: len(audio.Data), SHA256: hex.EncodeToString(sum[:])})
	return p.transcriber.Transcribe(ctx, model, audio, options)
}

// TestLiveTranscriptionPublicMedia is an explicit, paid-service acceptance test.
// CI skips it. The operator supplies downloaded public samples; no credentials
// are copied into the disposable library, test artifacts or console output.
func TestLiveTranscriptionPublicMedia(t *testing.T) {
	source := os.Getenv("KIST_LIVE_TRANSCRIPTION_CONFIG")
	inputs := os.Getenv("KIST_LIVE_TRANSCRIPTION_INPUTS")
	if source == "" || inputs == "" {
		t.Skip("requires explicit live-service configuration and public input directory")
	}
	cfg, err := config.Load(source)
	if err != nil {
		t.Fatal(err)
	}
	number, err := strconv.ParseInt(os.Getenv("KIST_LIVE_TRANSCRIPTION_ACCOUNT"), 10, 64)
	if err != nil || cfg.AIServices == nil {
		t.Fatal("select one configured OpenAI API account explicitly")
	}
	account, ok := cfg.AIServices.Account(number)
	if !ok || account.Type != ai.AccountOpenAI || account.APIKey == "" {
		t.Fatal("selected account is not a configured OpenAI API account")
	}
	l, box := openWithBox(t)
	provider, err := l.aiAccountProvider(t.Context(), cfg, number, &ai.ModelParameters{})
	if err != nil {
		t.Fatal(err)
	}
	transcriber, ok := provider.(ai.Transcriber)
	if !ok {
		t.Fatal("account cannot transcribe")
	}
	observed := &liveTranscriptionProvider{Provider: provider, transcriber: transcriber}
	setTranscription(t, l, false)
	q := l.TranscriptionQueue()
	q.provider = func(context.Context) (ai.Provider, error) { return observed, nil }
	type sampleResult struct {
		File       string                    `json:"file"`
		Model      string                    `json:"model"`
		Seconds    float64                   `json:"elapsed_seconds"`
		Transcript string                    `json:"transcript"`
		Uploads    []liveTranscriptionUpload `json:"uploads"`
	}
	results := []sampleResult{}
	for _, sample := range []struct{ name, sha256 string }{
		{"jfk.wav", "59dfb9a4acb36fe2a2affc14bacbee2920ff435cb13cc314a08c13f66ba7860e"},
		{"conformance-evaluation.mp4", "ec419be88b5d6cf2354a366caae8012a979438decef0154a6087a9813c662dbb"},
	} {
		name := sample.name
		data, err := os.ReadFile(filepath.Join(inputs, name))
		if err != nil {
			t.Fatal(err)
		}
		digest := sha256.Sum256(data)
		if hex.EncodeToString(digest[:]) != sample.sha256 {
			t.Fatal("input does not match the pinned public sample; refusing upload")
		}
		file := importOneFile(t, l, box, name, string(data))
		before := len(observed.uploads)
		started := time.Now()
		if _, err := q.Request(t.Context(), file.ID, false); err != nil {
			t.Fatal(err)
		}
		if err := q.Step(t.Context()); err != nil {
			t.Fatal(err)
		}
		job, err := q.FileStatus(t.Context(), file.ID)
		if err != nil || job.State != "ready" {
			t.Fatalf("%s: state=%s error=%s query=%v", name, job.State, job.Error, err)
		}
		rows := metaOf(t, l, file.ID)
		text := rows["transcription:text"].Value
		if strings.TrimSpace(text) == "" || rows["transcription:model"].Value != "openai/"+ai.DefaultTranscriptionModel {
			t.Fatal("missing transcript or incorrect model provenance")
		}
		uploads := append([]liveTranscriptionUpload(nil), observed.uploads[before:]...)
		if len(uploads) != 1 {
			t.Fatalf("%s: unexpected upload count %d", name, len(uploads))
		}
		for _, upload := range uploads {
			if !strings.HasPrefix(upload.MIME, "audio/") || upload.Bytes <= 0 || upload.Bytes > ai.MaxAudioBytes {
				t.Fatal("non-audio or oversized upload")
			}
			if strings.HasSuffix(name, ".mp4") && (upload.Name != "audio.wav" || upload.MIME != "audio/wav") {
				t.Fatal("video was not converted to audio before upload")
			}
		}
		results = append(results, sampleResult{File: name, Model: rows["transcription:model"].Value, Seconds: time.Since(started).Seconds(), Transcript: text, Uploads: uploads})
		t.Logf("%s: ready, %d transcript bytes, upload=%s/%s (%d bytes), %.2fs", name, len(text), uploads[0].Name, uploads[0].MIME, uploads[0].Bytes, time.Since(started).Seconds())
		// A completed result must not incur another paid request.
		calls := len(observed.uploads)
		if _, err := q.Request(t.Context(), file.ID, false); err != nil {
			t.Fatal(err)
		}
		if err := q.Step(t.Context()); err != nil {
			t.Fatal(err)
		}
		if len(observed.uploads) != calls {
			t.Fatal("completed sample was billed twice")
		}
	}
	before := snapshotOf(t, l)
	if err := l.Rebuild(t.Context()); err != nil {
		t.Fatal(err)
	}
	if snapshotOf(t, l) != before {
		t.Fatal("real transcription changed during rebuild")
	}
	if report, err := l.Verify(t.Context(), VerifyOptions{Deep: true}); err != nil || report.Total() != 0 {
		t.Fatal("verification failed", err)
	}
	raw, err := json.MarshalIndent(results, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(inputs, "live-results.json"), raw, 0600); err != nil {
		t.Fatal(err)
	}
}
