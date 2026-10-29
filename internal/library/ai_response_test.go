package library

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jinzhao1994/kist/internal/ai"
)

func TestFactsFailureResponseSurvivesQueueReloadAndRetry(t *testing.T) {
	l := openTestLibrary(t)
	id := ocrFile(t, l)
	setOCR(t, l, false)
	const raw = `{"fields":[{"key":"name","label":"Name","type":"text","value":"SYNTHETIC-REJECTED-OUTPUT"},{"key":"name","label":"Name","type":"text","value":"Another example"}]}`
	p := &factsProvider{fakeOCR: fakeOCR{text: raw}}
	q := l.FactsQueue()
	q.provider = func(context.Context) (ai.Provider, error) { return p, nil }
	first, err := q.Request(t.Context(), id, false)
	if err != nil {
		t.Fatal(err)
	}
	if err := q.Step(t.Context()); err != nil {
		t.Fatal(err)
	}
	// Recreate the queue to prove status and diagnostics do not depend on memory.
	job, err := l.FactsQueue().FileStatus(t.Context(), id)
	if err != nil || job.State != "failed" || job.RequestID != first.RequestID {
		t.Fatalf("failure was not persisted: %v", err)
	}
	db, err := q.database()
	if err != nil {
		t.Fatal(err)
	}
	read := func(request string) failedModelResponse {
		t.Helper()
		var r failedModelResponse
		var fileID, finished string
		err := db.DB.QueryRow(`SELECT request_id,kind,file_id,blob_key,finished_at,model,output_text,output_bytes,truncated FROM job_attempts WHERE request_id=?`, request).Scan(&r.RequestID, &r.Kind, &fileID, &r.BlobKey, &finished, &r.Model, &r.OutputText, &r.OutputBytes, &r.Truncated)
		if err != nil {
			t.Fatal(err)
		}
		if err = r.FileID.UnmarshalText([]byte(fileID)); err != nil {
			t.Fatal(err)
		}
		r.RecordedAt, err = time.Parse(time.RFC3339Nano, finished)
		if err != nil {
			t.Fatal(err)
		}
		return r
	}
	record := read(first.RequestID)
	if record.OutputText != raw || record.Truncated || record.OutputBytes != len(raw) || record.FileID != id || record.BlobKey != job.BlobKey || record.RequestID != job.RequestID || record.Kind != "facts" || record.RecordedAt.IsZero() || record.Model != "facts-model" {
		t.Fatal("diagnostic lost its output or task identity")
	}
	if runtime.GOOS != "windows" {
		for _, path := range []string{filepath.Join(l.layout.Root, "runtime.db")} {
			info, err := os.Stat(path)
			if err != nil || info.Mode().Perm()&0077 != 0 {
				t.Fatal("diagnostic permissions are not private")
			}
		}
	}
	if strings.Contains(job.Error, "SYNTHETIC-REJECTED-OUTPUT") || strings.Contains(snapshotOf(t, l), "SYNTHETIC-REJECTED-OUTPUT") {
		t.Fatal("rejected model output leaked into queue or archive")
	}
	if _, found := metaOf(t, l, id)["facts:data"]; found {
		t.Fatal("rejected facts were saved as metadata")
	}
	if err := q.Step(t.Context()); err != nil || p.calls != 1 {
		t.Fatal("failure was automatically rebilled")
	}
	second, err := q.Request(t.Context(), id, true)
	if err != nil || second.RequestID == first.RequestID {
		t.Fatal("retry did not receive a new identity")
	}
	if err := q.Step(t.Context()); err != nil {
		t.Fatal(err)
	}
	p.text = syntheticFacts
	if _, err := q.Request(t.Context(), id, true); err != nil {
		t.Fatal(err)
	}
	if err := q.Step(t.Context()); err != nil {
		t.Fatal(err)
	}
	requireOCRState(t, q, id, "ready")
	var failures int
	if err = db.DB.QueryRow(`SELECT count(*) FROM job_attempts WHERE kind='facts' AND state='failed' AND output_bytes>0`).Scan(&failures); err != nil || failures != 2 {
		t.Fatal("retry lost failure history", err)
	}
	if after := read(first.RequestID); after != record {
		t.Fatal("retry overwrote original failure")
	}
	attempts, err := q.Attempts(t.Context(), id, 0)
	if err != nil || len(attempts) != 3 || attempts[0].State != "succeeded" {
		t.Fatal("attempt listing lost ordered history", err)
	}
	older, err := q.Attempts(t.Context(), id, attempts[1].Sequence)
	if err != nil || len(older) != 1 || older[0].RequestID != first.RequestID {
		t.Fatal("attempt cursor did not preserve ordering", err)
	}
	detail, err := q.Attempt(t.Context(), id, first.RequestID)
	if err != nil || detail.OutputText != raw {
		t.Fatal("explicit attempt detail lost model output", err)
	}
	if _, err = l.OCRQueue().Attempt(t.Context(), id, first.RequestID); !errors.Is(err, ErrNoSuchOp) {
		t.Fatal("attempt detail escaped its extraction kind", err)
	}
}

func TestFailedModelResponseBoundsAndWriteFailure(t *testing.T) {
	l := openTestLibrary(t)
	q := l.FactsQueue()
	job := OCRJob{RequestID: uuid.NewString()}
	raw := strings.Repeat("界", maxFailedModelTextBytes/3+1)
	failure := &modelResponseError{cause: errors.New("synthetic validation failure"), result: ai.TextResult{Text: raw}}
	if strings.Contains(failure.Error(), "界") {
		t.Fatal("raw output appeared in error text")
	}
	if err := q.recordFailedModelResponse(job, failure); err != nil {
		t.Fatal(err)
	}
	db, err := q.database()
	if err != nil {
		t.Fatal(err)
	}
	var record failedModelResponse
	if err = db.DB.QueryRow(`SELECT output_text,output_bytes,truncated FROM job_attempts WHERE request_id=?`, job.RequestID).Scan(&record.OutputText, &record.OutputBytes, &record.Truncated); err != nil {
		t.Fatal(err)
	}
	if !record.Truncated || record.OutputBytes != len(raw) || len(record.OutputText) > maxFailedModelTextBytes || !utf8.ValidString(record.OutputText) || !strings.HasPrefix(raw, record.OutputText) {
		t.Fatal("diagnostic truncation is inaccurate")
	}
	job.RequestID = "../../escape"
	if err := q.recordFailedModelResponse(job, failure); !errors.Is(err, ErrInvalidRequest) {
		t.Fatal("unsafe diagnostic path was accepted")
	}
	job.RequestID = uuid.NewString()
	if err = db.DB.Close(); err != nil {
		t.Fatal(err)
	}
	if err = q.recordFailedModelResponse(job, failure); err == nil {
		t.Fatal("diagnostic write failure was hidden")
	}
}
