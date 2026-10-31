package library

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jinzhao1994/kist/internal/ai"
	"github.com/jinzhao1994/kist/internal/extract"
)

type fakeTranscriber struct {
	fakeOCR
	audioCalls int
	hint       string
	failure    bool
}

func (f *fakeTranscriber) Transcribe(_ context.Context, model string, audio ai.Audio, options ai.TextOptions) (ai.TextResult, error) {
	f.audioCalls++
	f.hint = options.Supplement
	if f.failure {
		return ai.TextResult{}, errors.New("synthetic transcription failure")
	}
	return ai.TextResult{Text: f.text, Provider: "synthetic", Model: model}, nil
}

func syntheticWAV() string {
	data := make([]byte, 44+320)
	copy(data, "RIFF")
	binary.LittleEndian.PutUint32(data[4:], uint32(len(data)-8))
	copy(data[8:], "WAVEfmt ")
	binary.LittleEndian.PutUint32(data[16:], 16)
	binary.LittleEndian.PutUint16(data[20:], 1)
	binary.LittleEndian.PutUint16(data[22:], 1)
	binary.LittleEndian.PutUint32(data[24:], 16000)
	binary.LittleEndian.PutUint32(data[28:], 32000)
	binary.LittleEndian.PutUint16(data[32:], 2)
	binary.LittleEndian.PutUint16(data[34:], 16)
	copy(data[36:], "data")
	binary.LittleEndian.PutUint32(data[40:], 320)
	return string(data)
}

func setTranscription(t *testing.T, l *Library, automatic bool) {
	t.Helper()
	draft, err := l.AIServicesSettings(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if len(draft.Accounts) == 0 {
		draft.Accounts = []AIAccountSettings{{Number: -1, ID: "测试 API", Type: ai.AccountOpenAI, APIKey: "synthetic-key"}}
		draft.Features["transcription"] = ai.Feature{Account: -1, Model: "gpt-transcribe", Automatic: automatic}
	} else {
		f := draft.Features["transcription"]
		f.Automatic = automatic
		draft.Features["transcription"] = f
	}
	saveServiceDraft(t, l, draft)
}

func TestTranscriptionMetadataReplayProtectionAndNoRepeat(t *testing.T) {
	l, box := openWithBox(t)
	file := importOneFile(t, l, box, "sample.wav", syntheticWAV())
	setTranscription(t, l, false)
	p := &fakeTranscriber{fakeOCR: fakeOCR{text: strings.Repeat("Synthetic transcript.\n", 200)}}
	q := l.TranscriptionQueue()
	q.provider = func(context.Context) (ai.Provider, error) { return p, nil }
	if _, err := q.Request(t.Context(), file.ID, false); err != nil {
		t.Fatal(err)
	}
	if err := q.Step(t.Context()); err != nil {
		t.Fatal(err)
	}
	requireOCRState(t, q, file.ID, "ready")
	rows := metaOf(t, l, file.ID)
	if rows["transcription:text"].Value != p.text || rows["transcription:text"].Source != "transcription" || rows["transcription:model"].Value != "synthetic/gpt-transcribe" {
		t.Fatal("missing transcription output")
	}
	if _, err := q.Backfill(t.Context(), false); err != nil {
		t.Fatal(err)
	}
	if err := q.Step(t.Context()); err != nil {
		t.Fatal(err)
	}
	if p.audioCalls != 1 {
		t.Fatal("completed audio transcribed again")
	}
	before := snapshotOf(t, l)
	if err := l.Rebuild(t.Context()); err != nil {
		t.Fatal(err)
	}
	if snapshotOf(t, l) != before {
		t.Fatal("transcription changed during rebuild")
	}
	if report, err := l.Verify(t.Context(), VerifyOptions{Deep: true}); err != nil || report.Total() != 0 {
		t.Fatal(report, err)
	}
	row := rows["transcription:text"]
	edit := MetadataEdit{BlobKey: file.BlobKey, Key: row.Key, BeforeValue: row.Value, BeforeSource: row.Source, BeforeType: string(row.Type), Value: "Human transcript"}
	if err := l.EditMetadata(t.Context(), file.ID, edit); err != nil {
		t.Fatal(err)
	}
	if _, err := q.Reextract(t.Context(), file.ID, uuid.NewString()); err != nil {
		t.Fatal(err)
	}
	if err := q.Step(t.Context()); err != nil {
		t.Fatal(err)
	}
	if p.audioCalls != 1 || metaOf(t, l, file.ID)[row.Key].Value != "Human transcript" {
		t.Fatal("human correction was overwritten or billed")
	}
}

func TestTranscriptionNewOnlyFailureAndExplicitRetry(t *testing.T) {
	l, box := openWithBox(t)
	old := importOneFile(t, l, box, "old.wav", syntheticWAV())
	setTranscription(t, l, true)
	fresh := importOneFile(t, l, box, "fresh.wav", syntheticWAV()+"different synthetic data")
	p := &fakeTranscriber{fakeOCR: fakeOCR{text: "Synthetic text"}, failure: true}
	q := l.TranscriptionQueue()
	q.provider = func(context.Context) (ai.Provider, error) { return p, nil }
	if err := q.Step(t.Context()); err != nil {
		t.Fatal(err)
	}
	requireOCRState(t, q, old.ID, "missing")
	requireOCRState(t, q, fresh.ID, "failed")
	status, err := q.Status(t.Context())
	if err != nil || !status.Paused {
		t.Fatal(status, err)
	}
	if err := q.Step(t.Context()); err != nil {
		t.Fatal(err)
	}
	if p.audioCalls != 1 {
		t.Fatal("failed audio retried automatically")
	}
	p.failure = false
	if _, err := q.Request(t.Context(), fresh.ID, true); err != nil {
		t.Fatal(err)
	}
	if err := q.Step(t.Context()); err != nil {
		t.Fatal(err)
	}
	requireOCRState(t, q, fresh.ID, "ready")
	if p.audioCalls != 2 {
		t.Fatal("manual retry did not run while background paused")
	}
	if _, err := q.Request(t.Context(), ocrFile(t, l), false); err == nil {
		t.Fatal("image accepted for transcription")
	}
}

func TestTranscriptionOutputBoundary(t *testing.T) {
	l, box := openWithBox(t)
	file := importOneFile(t, l, box, "sample.wav", syntheticWAV())
	p := &fakeTranscriber{fakeOCR: fakeOCR{text: strings.Repeat("x", ai.MaxTextBytes+1)}}
	if _, err := l.prepareTranscription(t.Context(), file.ID, p, "synthetic", file.BlobKey, false); err == nil {
		t.Fatal("oversized transcript accepted")
	}
	if _, err := l.prepareTranscription(t.Context(), file.ID, p, "synthetic", "different-blob", false); !errors.Is(err, ErrOCRChanged) {
		t.Fatal("stale input accepted", err)
	}
	if metaOf(t, l, file.ID)["transcription:text"].Type == extract.TypeText {
		t.Fatal("invalid result reached archive")
	}
}

func TestTranscriptionUndoRedoAndReplacedAudio(t *testing.T) {
	l, box := openWithBox(t)
	file := importOneFile(t, l, box, "sample.wav", syntheticWAV())
	provider := &fakeTranscriber{fakeOCR: fakeOCR{text: strings.Repeat("Synthetic long transcript.\n", 200)}}
	p, err := l.prepareTranscription(t.Context(), file.ID, provider, "gpt-transcribe", "", false)
	if err != nil {
		t.Fatal(err)
	}
	receipt, err := l.ExecuteRequest(t.Context(), requestFor("transcription-undo-0001", "transcription"), p.Commit)
	if err != nil {
		t.Fatal(err)
	}
	undo, err := l.Undo(t.Context(), receipt.OpID)
	if err != nil {
		t.Fatal(err)
	}
	if _, found := metaOf(t, l, file.ID)["transcription:text"]; found {
		t.Fatal("undo left transcription metadata")
	}
	if _, err := l.Undo(t.Context(), undo); err != nil {
		t.Fatal(err)
	}
	if metaOf(t, l, file.ID)["transcription:text"].Value != provider.text || provider.audioCalls != 1 {
		t.Fatal("redo did not restore the saved transcript without inference")
	}
	p, err = l.prepareTranscription(t.Context(), file.ID, provider, "gpt-transcribe", "", true)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := l.Import(t.Context(), writeTree(t, map[string]string{"sample.wav": syntheticWAV() + "replacement"}), ImportOptions{Box: box, ConflictMode: "overwrite"}); err != nil {
		t.Fatal(err)
	}
	if _, err := l.ExecuteRequest(t.Context(), requestFor("transcription-stale-0001", "transcription"), p.Commit); !errors.Is(err, ErrOCRChanged) {
		t.Fatal("stale transcript accepted after audio replacement", err)
	}
	if _, found := metaOf(t, l, file.ID)["transcription:text"]; found {
		t.Fatal("replacement retained the old audio's AI transcript")
	}
	before := snapshotOf(t, l)
	if err := l.Rebuild(t.Context()); err != nil || snapshotOf(t, l) != before {
		t.Fatal("replacement/undo history changed during replay", err)
	}
	if report, err := l.Verify(t.Context(), VerifyOptions{Deep: true}); err != nil || report.Total() != 0 {
		t.Fatal(report, err)
	}
}

type segmentAudioRenderer struct {
	duration        float64
	starts, seconds []float64
	outputs         []string
}

func (r *segmentAudioRenderer) TranscriptionDuration(context.Context, string) (float64, error) {
	return r.duration, nil
}
func (r *segmentAudioRenderer) RenderTranscriptionAudio(ctx context.Context, input, output string, start, seconds float64) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	r.starts = append(r.starts, start)
	r.seconds = append(r.seconds, seconds)
	r.outputs = append(r.outputs, output)
	return os.WriteFile(output, []byte(syntheticWAV()), 0600)
}

type segmentTranscriber struct {
	fakeOCR
	calls, failAt int
}

func (p *segmentTranscriber) Transcribe(ctx context.Context, model string, audio ai.Audio, options ai.TextOptions) (ai.TextResult, error) {
	p.calls++
	if p.calls == p.failAt {
		return ai.TextResult{}, errors.New("synthetic segment failure")
	}
	if audio.MIME != "audio/wav" || audio.Name != "audio.wav" || string(audio.Data[:4]) != "RIFF" {
		return ai.TextResult{}, errors.New("video frames or malformed audio uploaded")
	}
	return ai.TextResult{Text: fmt.Sprintf("part %d", p.calls), Provider: "synthetic", Model: model}, nil
}
func TestTranscriptionSegmentsInOrderWithoutSavingPartialResult(t *testing.T) {
	for _, failAt := range []int{0, 2} {
		t.Run(fmt.Sprint(failAt), func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "video.mov")
			if err := os.WriteFile(path, []byte("\x00\x00\x00\x14ftypqt  0000qt  "), 0600); err != nil {
				t.Fatal(err)
			}
			renderer := &segmentAudioRenderer{duration: 1201.5}
			provider := &segmentTranscriber{failAt: failAt}
			var lastDiagnostic string
			ctx := context.WithValue(t.Context(), modelAttemptKey{}, modelAttempt{kind: "transcription", record: func(result ai.TextResult) error { lastDiagnostic = result.Text; return nil }})
			result, err := transcribeMedia(ctx, provider, ai.DefaultTranscriptionModel, path, "video.mov", "", renderer)
			if failAt == 0 {
				if err != nil || result.Text != "part 1\npart 2\npart 3" || !slices.Equal(renderer.starts, []float64{0, 600, 1200}) || !slices.Equal(renderer.seconds, []float64{600, 600, 1.5}) {
					t.Fatal(result.Text, renderer.starts, renderer.seconds, err)
				}
			} else if err == nil || result.Text != "" || provider.calls != 2 || lastDiagnostic != "part 1" {
				t.Fatal("partial transcript returned as success or lost failure evidence", result.Text, lastDiagnostic, err)
			}
			for _, output := range renderer.outputs {
				if _, err := os.Stat(output); !errors.Is(err, os.ErrNotExist) {
					t.Fatal("temporary audio retained", err)
				}
			}
		})
	}
}
