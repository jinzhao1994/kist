package library

import (
	"errors"
	"io"
	"os"
	"testing"

	"github.com/jinzhao1994/kist/internal/derive"
)

func TestSyncDerivedImmutableDescriptorAndHistoricalContent(t *testing.T) {
	l, id, _ := mediaFixture(t)
	ctx := t.Context()
	m := l.Media()
	file, _, err := l.File(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	before, _ := l.OpCount(ctx)
	queued, err := m.DescribeSyncDerived(ctx, file.BlobKey, derive.Thumbnail, false)
	if err != nil || queued.Job.State != "queued" || queued.ContentKey != "" {
		t.Fatal(queued, err)
	}
	if err := ValidateSyncDerived(queued, file.BlobKey, derive.Thumbnail); err != nil {
		t.Fatal(err)
	}
	if err := m.work(ctx); err != nil {
		t.Fatal(err)
	}
	ready, err := m.DescribeSyncDerived(ctx, file.BlobKey, derive.Thumbnail, false)
	if err != nil || ready.Job.State != "ready" {
		t.Fatal(ready, err)
	}
	if err := ValidateSyncDerived(ready, file.BlobKey, derive.Thumbnail); err != nil {
		t.Fatal(err)
	}
	_, reader, err := m.OpenSyncDerived(ctx, file.BlobKey, derive.Thumbnail, ready.Job.Generation, ready.ContentKey)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := io.ReadAll(reader)
	reader.Close()
	if err != nil || int64(len(raw)) != ready.Job.Result.Bytes {
		t.Fatal(len(raw), err)
	}
	if _, _, err := m.OpenSyncDerived(ctx, file.BlobKey, derive.Thumbnail, "stale", ready.ContentKey); !errors.Is(err, ErrSyncConflict) {
		t.Fatal(err)
	}
	_, path, _ := m.paths(ready.Job)
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	next, err := m.DescribeSyncDerived(ctx, file.BlobKey, derive.Thumbnail, false)
	if err != nil || next.Job.Generation == ready.Job.Generation {
		t.Fatal(next, err)
	}
	if err := m.work(ctx); err != nil {
		t.Fatal(err)
	}
	if _, _, err := m.OpenSyncDerived(ctx, file.BlobKey, derive.Thumbnail, ready.Job.Generation, ready.ContentKey); !errors.Is(err, ErrSyncConflict) {
		t.Fatal(err)
	}
	if count, _ := l.OpCount(ctx); count != before {
		t.Fatal("preview transfer appended an archive operation")
	}
	report, err := l.Verify(ctx, VerifyOptions{Deep: true})
	if err != nil || report.Total() != 0 {
		t.Fatal(report, err)
	}
}
