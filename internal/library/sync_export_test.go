package library

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/jinzhao1994/kist/internal/domain"
	"github.com/jinzhao1994/kist/internal/journal"
)

func TestSyncCatalogFixedBoundaryPreservesEnvelopes(t *testing.T) {
	l := openTestLibrary(t)
	req := Request{ID: "ordinary-request", Digest: strings.Repeat("b", 64)}
	if _, err := l.ExecuteRequest(t.Context(), req, func(ctx context.Context) error {
		_, err := l.CreateBox(ctx, "ordinary", "", domain.LocalDate{}, domain.LocalDate{})
		return err
	}); err != nil {
		t.Fatal(err)
	}
	p := syncBox(t, "offline-catalog-event")
	accepted, err := l.AcceptSync(t.Context(), p)
	if err != nil {
		t.Fatal(err)
	}
	head, err := l.SyncHead(t.Context())
	if err != nil || head.OpID != accepted.OpID {
		t.Fatalf("head=%+v err=%v", head, err)
	}
	other, err := Open(l.layout.Root)
	if err != nil {
		t.Fatal(err)
	}
	defer other.Close()
	if _, err := other.CreateBox(t.Context(), "concurrent NAS edit", "", domain.LocalDate{}, domain.LocalDate{}); err != nil {
		t.Fatal(err)
	}
	// Even a one-byte page budget must let one indivisible legacy event through.
	first, err := l.SyncCatalog(t.Context(), head, 0, 256, 1)
	if err != nil || first.Next != 1 || first.Done || len(first.Lines) != 1 {
		t.Fatalf("first=%+v err=%v", first, err)
	}
	line, err := journal.UnmarshalLine(first.Lines[0])
	if err != nil || line.V != 2 || line.Sync != nil || line.Request == nil || line.Request.ID != req.ID || line.Request.Digest != req.Digest {
		t.Fatalf("ordinary envelope=%+v err=%v", line, err)
	}
	last, err := l.SyncCatalog(t.Context(), head, first.Next, 1, 4<<20)
	if err != nil || !last.Done || last.Next != head.OpID || len(last.Lines) != 1 {
		t.Fatalf("last=%+v err=%v", last, err)
	}
	line, err = journal.UnmarshalLine(last.Lines[0])
	if err != nil || line.V != 3 || line.Sum != head.Sum || line.Sync == nil || line.Sync.ID != p.ID || line.TS != journal.FormatTime(p.OccurredAt) || line.Sync.AcceptedAt != journal.FormatTime(accepted.AcceptedAt) {
		t.Fatalf("sync envelope=%+v err=%v", line, err)
	}
	if err := l.Rebuild(t.Context()); err != nil {
		t.Fatal(err)
	}
	again, err := l.SyncCatalog(t.Context(), head, first.Next, 1, 4<<20)
	if err != nil || string(again.Lines[0]) != string(last.Lines[0]) {
		t.Fatalf("rebuild changed exported event: %v", err)
	}
	bad := head
	bad.Sum = "wrong"
	if _, err := l.SyncCatalog(t.Context(), bad, 0, 1, 1024); !errors.Is(err, ErrSyncConflict) {
		t.Fatalf("changed history boundary: %v", err)
	}
	bad = head
	bad.LibraryID = domain.NewID()
	if _, err := l.SyncCatalog(t.Context(), bad, 0, 1, 1024); !errors.Is(err, ErrSyncConflict) {
		t.Fatalf("different library: %v", err)
	}
}

func TestSyncCatalogEmptyAndInvalidRanges(t *testing.T) {
	l := openTestLibrary(t)
	head, err := l.SyncHead(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	page, err := l.SyncCatalog(t.Context(), head, 0, 10, 1024)
	if err != nil || !page.Done || len(page.Lines) != 0 || page.Next != 0 {
		t.Fatalf("empty=%+v err=%v", page, err)
	}
	for _, args := range [][3]int{{-1, 1, 1}, {1, 1, 1}, {0, 0, 1}, {0, 257, 1}, {0, 1, 0}, {0, 1, 4<<20 + 1}} {
		if _, err := l.SyncCatalog(t.Context(), head, int64(args[0]), args[1], args[2]); !errors.Is(err, ErrInvalidRequest) {
			t.Fatalf("range %v: %v", args, err)
		}
	}
}
