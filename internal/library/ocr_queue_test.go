package library

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jinzhao1994/kist/internal/ai"
	"github.com/jinzhao1994/kist/internal/config"
	"github.com/jinzhao1994/kist/internal/domain"
)

func setOCR(t *testing.T, l *Library, auto bool) {
	t.Helper()
	s, err := l.OCRSettings(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	s.Model = "test"
	s.AutoOCR = auto
	if _, err = l.SaveOCRSettings(t.Context(), s); err != nil {
		t.Fatal(err)
	}
}
func queueFor(t *testing.T, l *Library, p ai.Provider) *OCRQueue {
	t.Helper()
	q := l.OCRQueue()
	q.provider = func(context.Context) (ai.Provider, error) { return p, nil }
	return q
}
func requireOCRState(t *testing.T, q *OCRQueue, id domain.FileID, state string) {
	t.Helper()
	j, err := q.FileStatus(t.Context(), id)
	if err != nil || j.State != state {
		t.Fatalf("state=%s want=%s err=%v", j.State, state, err)
	}
}
func TestOCRQueueNewOnlyAndExplicitBackfill(t *testing.T) {
	l := openTestLibrary(t)
	old := ocrFile(t, l)
	setOCR(t, l, true)
	fresh := ocrFile(t, l)
	p := &fakeOCR{text: "recognized"}
	q := queueFor(t, l, p)
	if err := q.Step(t.Context()); err != nil {
		t.Fatal(err)
	}
	requireOCRState(t, q, old, "missing")
	requireOCRState(t, q, fresh, "ready")
	if p.calls != 1 {
		t.Fatal(p.calls)
	}
	if _, err := q.Backfill(t.Context(), false); err != nil {
		t.Fatal(err)
	}
	if err := q.Step(t.Context()); err != nil {
		t.Fatal(err)
	}
	requireOCRState(t, q, old, "ready")
	if _, err := q.Backfill(t.Context(), false); err != nil {
		t.Fatal(err)
	}
	if err := q.Step(t.Context()); err != nil {
		t.Fatal(err)
	}
	if p.calls != 2 {
		t.Fatal("completed files were rebilled", p.calls)
	}
}

type failingOCR struct{ fakeOCR }

func (p *failingOCR) OCR(context.Context, string, ai.Image, ai.TextOptions) (ai.OCRResult, error) {
	p.calls++
	return ai.OCRResult{}, errors.New("provider unavailable")
}
func TestOCRQueueFailurePauseAndExplicitRetry(t *testing.T) {
	l := openTestLibrary(t)
	id := ocrFile(t, l)
	setOCR(t, l, false)
	p := &failingOCR{}
	q := queueFor(t, l, p)
	if _, err := q.Request(t.Context(), id, false); err != nil {
		t.Fatal(err)
	}
	if err := q.Step(t.Context()); err != nil {
		t.Fatal(err)
	}
	requireOCRState(t, q, id, "failed")
	status, _ := q.Status(t.Context())
	if !status.Paused || status.Failed != 1 {
		t.Fatal(status)
	}
	q = queueFor(t, l, p)
	q.Pause(t.Context(), false)
	q.Backfill(t.Context(), false)
	if err := q.Step(t.Context()); err != nil {
		t.Fatal(err)
	}
	if p.calls != 1 {
		t.Fatal("failure automatically retried")
	}
	good := &fakeOCR{text: "retry result"}
	q = queueFor(t, l, good)
	q.Backfill(t.Context(), true)
	if err := q.Step(t.Context()); err != nil {
		t.Fatal(err)
	}
	requireOCRState(t, q, id, "ready")
	if good.calls != 1 {
		t.Fatal(good.calls)
	}
}
func TestOCRQueueInterruptedAndDuplicateRequests(t *testing.T) {
	l := openTestLibrary(t)
	id := ocrFile(t, l)
	setOCR(t, l, false)
	p := &fakeOCR{text: "text"}
	q := queueFor(t, l, p)
	first, err := q.Request(t.Context(), id, false)
	if err != nil {
		t.Fatal(err)
	}
	again, err := q.Request(t.Context(), id, true)
	if err != nil || first.RequestID != again.RequestID {
		t.Fatal("queued job was duplicated", err)
	}
	s, _ := q.load()
	s.Jobs[0].State = "running"
	if err = q.save(s); err != nil {
		t.Fatal(err)
	}
	if err = q.Step(t.Context()); err != nil {
		t.Fatal(err)
	}
	requireOCRState(t, q, id, "failed")
	if p.calls != 0 {
		t.Fatal("interrupted request was rebilled")
	}
}
func TestOCRSettingsRevisionAndReenableBoundary(t *testing.T) {
	l := openTestLibrary(t)
	s, _ := l.OCRSettings(t.Context())
	setOCR(t, l, true)
	s.Model = "stale"
	if _, err := l.SaveOCRSettings(t.Context(), s); !errors.Is(err, config.ErrChanged) {
		t.Fatal(err)
	}
	setOCR(t, l, false)
	old := ocrFile(t, l)
	setOCR(t, l, true)
	p := &fakeOCR{}
	q := queueFor(t, l, p)
	if err := q.Step(t.Context()); err != nil {
		t.Fatal(err)
	}
	requireOCRState(t, q, old, "missing")
	if p.calls != 0 {
		t.Fatal("disabled-period image was scanned")
	}
}

type blockingOCR struct {
	fakeOCR
	started chan struct{}
	finish  chan struct{}
}

func (p *blockingOCR) OCR(ctx context.Context, model string, img ai.Image, options ai.TextOptions) (ai.OCRResult, error) {
	close(p.started)
	select {
	case <-ctx.Done():
		return ai.OCRResult{}, ctx.Err()
	case <-p.finish:
		return p.fakeOCR.OCR(ctx, model, img, options)
	}
}
func TestOCRQueueWorkerExclusionAndPauseInFlight(t *testing.T) {
	l := openTestLibrary(t)
	id := ocrFile(t, l)
	setOCR(t, l, false)
	p := &blockingOCR{started: make(chan struct{}), finish: make(chan struct{})}
	q := queueFor(t, l, p)
	if _, err := q.Request(t.Context(), id, false); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- q.Step(t.Context()) }()
	<-p.started
	// Another worker cannot duplicate inference, while ordinary archive writes proceed.
	other := queueFor(t, l, p)
	if err := other.Step(t.Context()); err != nil {
		t.Fatal(err)
	}
	second := ocrFile(t, l)
	if _, err := other.Request(t.Context(), second, false); err != nil {
		t.Fatal(err)
	}
	if _, err := other.Pause(t.Context(), true); err != nil {
		t.Fatal(err)
	}
	close(p.finish)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	requireOCRState(t, q, id, "ready")
	requireOCRState(t, q, second, "queued")
	next := &fakeOCR{text: "manual request"}
	other = queueFor(t, l, next)
	if err := other.Step(t.Context()); err != nil {
		t.Fatal(err)
	}
	requireOCRState(t, other, second, "ready")
	status, err := other.Status(t.Context())
	if err != nil || !status.Paused || next.calls != 1 {
		t.Fatalf("status=%+v calls=%d err=%v", status, next.calls, err)
	}
}

func TestOCRQueueSkipsStaleInputBeforeProvider(t *testing.T) {
	l := openTestLibrary(t)
	id := ocrFile(t, l)
	setOCR(t, l, false)
	p := &fakeOCR{text: "must not run"}
	q := queueFor(t, l, p)
	if _, err := q.Request(t.Context(), id, false); err != nil {
		t.Fatal(err)
	}
	unlock, err := q.state.Acquire(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	s, err := q.load()
	if err != nil {
		t.Fatal(err)
	}
	s.Jobs[0].BlobKey = "sha256:" + strings.Repeat("f", 64)
	s.Jobs[0].RequestID = uuid.NewString()
	if err = q.save(s); err != nil {
		t.Fatal(err)
	}
	unlock()
	if err = q.Step(t.Context()); err != nil {
		t.Fatal(err)
	}
	status, err := q.Status(t.Context())
	if err != nil || status.Paused || status.Queued != 0 || status.Failed != 0 || p.calls != 0 {
		t.Fatalf("status=%+v calls=%d err=%v", status, p.calls, err)
	}
}

func TestOCRQueueArchiveCompletionOverridesFailedRuntimeState(t *testing.T) {
	l := openTestLibrary(t)
	id := ocrFile(t, l)
	setOCR(t, l, false)
	p := &fakeOCR{text: "saved"}
	q := queueFor(t, l, p)
	job, err := q.Request(t.Context(), id, false)
	if err != nil {
		t.Fatal(err)
	}
	if err = q.Step(t.Context()); err != nil {
		t.Fatal(err)
	}
	// Simulate a runtime failure recorded after the archive write became durable.
	job.State, job.Error = "failed", "interrupted after commit"
	if err = q.save(ocrQueueState{Paused: true, Jobs: []OCRJob{job}}); err != nil {
		t.Fatal(err)
	}
	if err = q.Step(t.Context()); err != nil {
		t.Fatal(err)
	}
	status, err := q.Status(t.Context())
	if err != nil || status.Failed != 0 || !status.Paused || p.calls != 1 {
		t.Fatalf("status=%+v calls=%d err=%v", status, p.calls, err)
	}
	requireOCRState(t, q, id, "ready")
}

func TestOCRQueuePausedAllowsOnlyExplicitSingleFileRequests(t *testing.T) {
	l := openTestLibrary(t)
	setOCR(t, l, true)
	automatic, bulk, manual := ocrFile(t, l), ocrFile(t, l), ocrFile(t, l)
	p := &fakeOCR{text: "single file"}
	q := queueFor(t, l, p)
	autoJob, err := q.resolve(t.Context(), automatic)
	if err != nil {
		t.Fatal(err)
	}
	bulkJob, err := q.resolve(t.Context(), bulk)
	if err != nil {
		t.Fatal(err)
	}
	s := ocrQueueState{Paused: true}
	enqueueOCR(&s, autoJob, false, true)
	enqueueOCR(&s, bulkJob, false, false)
	if err := q.save(s); err != nil {
		t.Fatal(err)
	}
	if _, err := q.Backfill(t.Context(), false); err != nil {
		t.Fatal(err)
	}
	if _, err := q.Request(t.Context(), manual, false); err != nil {
		t.Fatal(err)
	}
	// Reload through another queue instance to prove the intent is persisted.
	q = queueFor(t, l, p)
	if err := q.Step(t.Context()); err != nil {
		t.Fatal(err)
	}
	requireOCRState(t, q, manual, "ready")
	requireOCRState(t, q, automatic, "queued")
	requireOCRState(t, q, bulk, "queued")
	if err := q.Step(t.Context()); err != nil {
		t.Fatal(err)
	}
	if p.calls != 1 {
		t.Fatalf("paused bulk work ran: %d", p.calls)
	}
	s, err = q.load()
	if err != nil || !s.Paused || s.Backfill == nil || s.Backfill.After != -1 || s.AutoScan != nil {
		t.Fatalf("paused scans changed: %+v, %v", s, err)
	}
	// A single-file click promotes an existing bulk job without duplicating it.
	before, err := q.FileStatus(t.Context(), bulk)
	if err != nil {
		t.Fatal(err)
	}
	promoted, err := q.Request(t.Context(), bulk, false)
	if err != nil || !promoted.Manual || promoted.RequestID != before.RequestID {
		t.Fatalf("promotion: %+v, %v", promoted, err)
	}
	if err := q.Step(t.Context()); err != nil {
		t.Fatal(err)
	}
	requireOCRState(t, q, bulk, "ready")
	requireOCRState(t, q, automatic, "queued")
	// Refresh also runs while paused and preserves its idempotency key.
	key := uuid.NewString()
	if _, err := q.Reextract(t.Context(), manual, key); err != nil {
		t.Fatal(err)
	}
	if err := q.Step(t.Context()); err != nil {
		t.Fatal(err)
	}
	if _, err := q.Reextract(t.Context(), manual, key); err != nil {
		t.Fatal(err)
	}
	if err := q.Step(t.Context()); err != nil {
		t.Fatal(err)
	}
	status, err := q.Status(t.Context())
	if err != nil || !status.Paused || p.calls != 3 {
		t.Fatalf("status=%+v calls=%d err=%v", status, p.calls, err)
	}
	// Resuming dispatches held background work.
	if _, err := q.Pause(t.Context(), false); err != nil {
		t.Fatal(err)
	}
	if err := q.Step(t.Context()); err != nil {
		t.Fatal(err)
	}
	requireOCRState(t, q, automatic, "ready")
}

func TestOCRQueuePausedManualFailureRequiresExplicitRetry(t *testing.T) {
	l := openTestLibrary(t)
	id := ocrFile(t, l)
	setOCR(t, l, false)
	p := &failingOCR{}
	q := queueFor(t, l, p)
	if _, err := q.Pause(t.Context(), true); err != nil {
		t.Fatal(err)
	}
	if _, err := q.Request(t.Context(), id, false); err != nil {
		t.Fatal(err)
	}
	if err := q.Step(t.Context()); err != nil {
		t.Fatal(err)
	}
	if err := q.Step(t.Context()); err != nil {
		t.Fatal(err)
	}
	requireOCRState(t, q, id, "failed")
	if p.calls != 1 {
		t.Fatal("failure was retried automatically")
	}
	good := &fakeOCR{text: "retried"}
	q = queueFor(t, l, good)
	if _, err := q.Request(t.Context(), id, true); err != nil {
		t.Fatal(err)
	}
	if err := q.Step(t.Context()); err != nil {
		t.Fatal(err)
	}
	requireOCRState(t, q, id, "ready")
	status, err := q.Status(t.Context())
	if err != nil || !status.Paused || good.calls != 1 {
		t.Fatalf("status=%+v calls=%d err=%v", status, good.calls, err)
	}
}

func TestOCRQueueBulkRetryDoesNotBypassPause(t *testing.T) {
	l := openTestLibrary(t)
	id := ocrFile(t, l)
	setOCR(t, l, false)
	good := &fakeOCR{text: "saved"}
	q := queueFor(t, l, good)
	if _, err := q.Request(t.Context(), id, false); err != nil {
		t.Fatal(err)
	}
	if err := q.Step(t.Context()); err != nil {
		t.Fatal(err)
	}
	failing := &failingOCR{}
	q = queueFor(t, l, failing)
	if _, err := q.Reextract(t.Context(), id, uuid.NewString()); err != nil {
		t.Fatal(err)
	}
	if err := q.Step(t.Context()); err != nil {
		t.Fatal(err)
	}
	if _, err := q.Backfill(t.Context(), true); err != nil {
		t.Fatal(err)
	}
	if err := q.Step(t.Context()); err != nil {
		t.Fatal(err)
	}
	job, err := q.FileStatus(t.Context(), id)
	if err != nil || job.Manual || job.State != "queued" || failing.calls != 1 {
		t.Fatalf("job=%+v calls=%d err=%v", job, failing.calls, err)
	}
}

func TestAIQueueFailurePolicyAndConfigRoundTrip(t *testing.T) {
	no, yes := false, true
	for _, tc := range []struct {
		name   string
		value  *bool
		paused bool
	}{{"default", nil, true}, {"explicit-pause", &yes, true}, {"continue", &no, false}} {
		t.Run(tc.name, func(t *testing.T) {
			l := openTestLibrary(t)
			draft, err := l.AIServicesSettings(t.Context())
			if err != nil {
				t.Fatal(err)
			}
			draft.Accounts = []AIAccountSettings{{Number: -1, ID: "Synthetic", Type: ai.AccountOpenAI, APIKey: "synthetic-key"}}
			draft.Features["ocr"] = ai.Feature{Account: -1, Model: "test", PauseOnError: tc.value}
			draft = saveServiceDraft(t, l, draft)
			cfg, err := config.Load(l.layout.ConfigPath())
			if err != nil {
				t.Fatal(err)
			}
			feature := cfg.AIServices.Features["ocr"]
			if feature.PausesOnError() != tc.paused || (feature.PauseOnError == nil) != (tc.value == nil) {
				t.Fatal("configuration lost inherited or explicit failure policy")
			}
			ocrFile(t, l)
			ocrFile(t, l)
			bad := &failingOCR{}
			q := queueFor(t, l, bad)
			if _, err := q.Backfill(t.Context(), false); err != nil {
				t.Fatal(err)
			}
			if err := q.Step(t.Context()); err != nil {
				t.Fatal(err)
			}
			status, err := q.Status(t.Context())
			if err != nil || status.Paused != tc.paused || status.Failed != 1 || status.Queued != 1 {
				t.Fatalf("status=%+v err=%v", status, err)
			}
			good := &fakeOCR{text: "recognized"}
			q.provider = func(context.Context) (ai.Provider, error) { return good, nil }
			if tc.paused {
				if err := q.Step(t.Context()); err != nil {
					t.Fatal(err)
				}
				if good.calls != 0 {
					t.Fatal("paused background job executed")
				}
				if _, err := q.Pause(t.Context(), false); err != nil {
					t.Fatal(err)
				}
			}
			if err := q.Step(t.Context()); err != nil {
				t.Fatal(err)
			}
			if _, err := q.Backfill(t.Context(), false); err != nil {
				t.Fatal(err)
			}
			if err := q.Step(t.Context()); err != nil {
				t.Fatal(err)
			}
			status, err = q.Status(t.Context())
			if err != nil || good.calls != 1 || status.Failed != 1 || status.Queued != 0 {
				t.Fatal("failed task was automatically retried or next task was blocked", status, err)
			}
			draft, err = l.AIServicesSettings(t.Context())
			if err != nil {
				t.Fatal(err)
			}
			feature = draft.Features["ocr"]
			feature.PauseOnError = nil
			draft.Features["ocr"] = feature
			draft = saveServiceDraft(t, l, draft)
			if draft.Features["ocr"].PauseOnError != nil || !draft.Features["ocr"].PausesOnError() {
				t.Fatal("restore default did not remove explicit policy")
			}
		})
	}
}
