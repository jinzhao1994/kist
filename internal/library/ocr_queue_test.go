package library

import (
	"context"
	"errors"
	"testing"

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

func (p *failingOCR) OCR(context.Context, string, ai.Image) (ai.OCRResult, error) {
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

func (p *blockingOCR) OCR(ctx context.Context, model string, img ai.Image) (ai.OCRResult, error) {
	close(p.started)
	select {
	case <-ctx.Done():
		return ai.OCRResult{}, ctx.Err()
	case <-p.finish:
		return p.fakeOCR.OCR(ctx, model, img)
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
	if err := other.Step(t.Context()); err != nil {
		t.Fatal(err)
	}
	if p.calls != 1 {
		t.Fatal("paused queue continued", p.calls)
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
	s.Jobs[0].BlobKey = "superseded-input"
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
