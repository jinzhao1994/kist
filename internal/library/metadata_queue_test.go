package library

import (
	"context"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/jinzhao1994/kist/internal/domain"
)

func TestMetadataQueueAutomaticAndPriority(t *testing.T) {
	l, a, b := mediaFixture(t)
	ctx := t.Context()
	meta := l.MetadataQueue()
	setOCR(t, l, false)
	provider := &fakeOCR{text: "recognized"}
	ocr := queueFor(t, l, provider)
	if _, err := ocr.Backfill(ctx, false); err != nil {
		t.Fatal(err)
	}
	queues := []*OCRQueue{meta, ocr}
	cursors := map[int]int{}
	for range 2 {
		ran, err := meta.dispatch(ctx, queues, cursors)
		if err != nil || !ran {
			t.Fatal(ran, err)
		}
		if provider.calls != 0 {
			t.Fatal("AI ran before local metadata backlog")
		}
	}
	for _, id := range []domain.FileID{a, b} {
		info, err := l.MediaInfo(ctx, id)
		if err != nil || len(info.Pending) != 0 {
			t.Fatal(info, err)
		}
	}
	if ran, err := meta.dispatch(ctx, queues, cursors); err != nil || !ran || provider.calls != 1 {
		t.Fatal("AI did not follow local metadata", ran, provider.calls, err)
	}
	// New imports regain priority at the next job boundary, even during an AI batch.
	fresh := ocrFile(t, l)
	if ran, err := meta.dispatch(ctx, queues, cursors); err != nil || !ran || provider.calls != 1 {
		t.Fatal("new metadata lost priority", ran, provider.calls, err)
	}
	if info, err := l.MediaInfo(ctx, fresh); err != nil || len(info.Pending) != 0 {
		t.Fatal(info, err)
	}
	if report, err := l.Verify(ctx, VerifyOptions{Deep: true}); err != nil || report.Total() != 0 {
		t.Fatal(report, err)
	}
	if err := l.Rebuild(ctx); err != nil {
		t.Fatal(err)
	}
	if job, err := l.MetadataQueue().FileStatus(ctx, a); err != nil || job.State != "ready" {
		t.Fatal("rebuild lost local extraction completion", job, err)
	}
}

func TestMetadataQueueAutomaticallyProcessesReplacement(t *testing.T) {
	l, a, _ := mediaFixture(t)
	ctx := t.Context()
	meta := l.MetadataQueue()
	for range 2 {
		if err := meta.Step(ctx); err != nil {
			t.Fatal(err)
		}
	}
	file, _, err := l.File(ctx, a)
	if err != nil {
		t.Fatal(err)
	}
	box, _, err := l.FileLocation(ctx, file)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = l.Import(ctx, writeTree(t, map[string]string{file.Name: "replacement text"}), ImportOptions{Box: box, ConflictMode: "overwrite"}); err != nil {
		t.Fatal(err)
	}
	if err = meta.Step(ctx); err != nil {
		t.Fatal(err)
	}
	info, err := l.MediaInfo(ctx, a)
	if err != nil || len(info.Pending) != 0 || info.Kind != "document" {
		t.Fatal(info, err)
	}
}

func TestMetadataQueuePauseDependenciesAndRefresh(t *testing.T) {
	l, a, _ := mediaFixture(t)
	ctx := t.Context()
	meta := l.MetadataQueue()
	setOCR(t, l, false)
	provider := &fakeOCR{}
	ocr := queueFor(t, l, provider)
	meta.Pause(ctx, true)
	ocr.Backfill(ctx, false)
	queues := []*OCRQueue{meta, ocr}
	cursors := map[int]int{}
	if ran, err := meta.dispatch(ctx, queues, cursors); err != nil || ran || provider.calls != 0 {
		t.Fatal("paused dependency was bypassed", ran, provider.calls, err)
	}
	meta.Pause(ctx, false)
	for range 2 {
		if err := meta.Step(ctx); err != nil {
			t.Fatal(err)
		}
	}
	meta.Pause(ctx, true)
	key := uuid.NewString()
	first, err := meta.RequestExtraction(ctx, a, false, true, key, "media")
	if err != nil {
		t.Fatal(err)
	}
	again, err := l.MetadataQueue().RequestExtraction(ctx, a, false, true, key, "media")
	if err != nil || first.RequestID != again.RequestID || again.Extractor != "media" {
		t.Fatal("lost refresh scope", again, err)
	}
	if err = meta.Step(ctx); err != nil {
		t.Fatal(err)
	}
	job, err := meta.FileStatus(ctx, a)
	if err != nil || job.State != "ready" {
		t.Fatal("manual refresh blocked by bulk pause", job, err)
	}
	if _, found, err := l.CheckRequest(ctx, meta.jobRequest(first)); err != nil || !found {
		t.Fatal("refresh completion not durable", found, err)
	}
}

func TestMetadataQueueFailureIsolationAndRetry(t *testing.T) {
	l, a, b := mediaFixture(t)
	ctx := t.Context()
	meta := l.MetadataQueue()
	file, _, err := l.File(ctx, a)
	if err != nil {
		t.Fatal(err)
	}
	path, _ := l.blobs.LocalPath(blobKey(file.BlobKey))
	// Both synthetic files share this blob. Restore it before the second job.
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.Rename(path, path+".saved"); err != nil {
		t.Fatal(err)
	}
	if err = meta.Step(ctx); err != nil {
		t.Fatal(err)
	}
	if state, err := meta.Status(ctx); err != nil || state.Paused || state.Failed != 1 {
		t.Fatal(state, err)
	}
	if err = os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	if err = meta.Step(ctx); err != nil {
		t.Fatal(err)
	}
	first, _ := meta.FileStatus(ctx, a)
	second, _ := meta.FileStatus(ctx, b)
	if first.State != "failed" && second.State != "failed" {
		t.Fatal("failed item was silently retried")
	}
	if first.State != "ready" && second.State != "ready" {
		t.Fatal("bad file blocked later job")
	}
	meta.Backfill(ctx, true)
	if err = meta.Step(ctx); err != nil {
		t.Fatal(err)
	}
	for _, id := range []domain.FileID{a, b} {
		if job, err := meta.FileStatus(ctx, id); err != nil || job.State != "ready" {
			t.Fatal(job, err)
		}
	}
}

func TestMetadataReaderHonorsCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	file, err := os.CreateTemp(t.TempDir(), "reader")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	reader := contextReadSeeker{context: ctx, ReadSeeker: file}
	if _, err = reader.Read(make([]byte, 1)); err != context.Canceled {
		t.Fatal(err)
	}
	if _, err = reader.Seek(0, 0); err != context.Canceled {
		t.Fatal(err)
	}
}

func TestMetadataDependencyFailureDoesNotBlockOtherAIJobs(t *testing.T) {
	l, a, _ := mediaFixture(t)
	ctx := t.Context()
	meta := l.MetadataQueue()
	failed, err := meta.resolve(ctx, a)
	if err != nil {
		t.Fatal(err)
	}
	failed.State = "failed"
	failed.RequestID = uuid.NewString()
	failed.Error = "synthetic parse failure"
	if err = meta.save(ocrQueueState{Jobs: []OCRJob{failed}}); err != nil {
		t.Fatal(err)
	}
	setOCR(t, l, false)
	provider := &fakeOCR{}
	ocr := queueFor(t, l, provider)
	ocr.Backfill(ctx, false)
	queues := []*OCRQueue{meta, ocr}
	cursors := map[int]int{}
	for range 2 {
		if _, err = meta.dispatch(ctx, queues, cursors); err != nil {
			t.Fatal(err)
		}
	}
	if provider.calls != 1 {
		t.Fatal("blocked dependency stalled unrelated AI work", provider.calls)
	}
}

func TestMetadataQueueResumesInterruptedLocalWork(t *testing.T) {
	l, a, _ := mediaFixture(t)
	ctx := t.Context()
	q := l.MetadataQueue()
	first, err := q.Request(ctx, a, false)
	if err != nil {
		t.Fatal(err)
	}
	s, err := q.load()
	if err != nil {
		t.Fatal(err)
	}
	s.Jobs[0].State = "running"
	if err = q.save(s); err != nil {
		t.Fatal(err)
	}
	q = l.MetadataQueue()
	if err = q.finishJobWithPolicy(first, context.Canceled, false); err != nil {
		t.Fatal(err)
	}
	resumed, err := q.FileStatus(ctx, a)
	if err != nil || resumed.State != "queued" || resumed.RequestID != first.RequestID {
		t.Fatal(resumed, err)
	}
	s, err = q.load()
	if err != nil {
		t.Fatal(err)
	}
	s.Jobs[0].State = "running"
	if err = q.save(s); err != nil {
		t.Fatal(err)
	}
	if err = q.Step(ctx); err != nil {
		t.Fatal(err)
	}
	if job, err := q.FileStatus(ctx, a); err != nil || job.State != "ready" {
		t.Fatal(job, err)
	}
}
