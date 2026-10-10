package library

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/jinzhao1994/kist/internal/domain"
	"github.com/jinzhao1994/kist/internal/extract"
)

func TestHistoricalPrefixesMatchOriginalStates(t *testing.T) {
	l := openTestLibrary(t)
	ctx := t.Context()
	now := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	l.now = func() time.Time { now = now.Add(time.Second); return now }
	snapshots := map[int64]string{}
	checkpoint := func() int64 {
		t.Helper()
		n, err := l.OpCount(ctx)
		if err != nil {
			t.Fatal(err)
		}
		snapshots[n] = snapshotOf(t, l)
		return n
	}
	b1, err := l.CreateBox(ctx, "first", "", domain.LocalDate{}, domain.LocalDate{})
	if err != nil {
		t.Fatal(err)
	}
	checkpoint()
	b2, err := l.CreateBox(ctx, "second", "", domain.LocalDate{}, domain.LocalDate{})
	if err != nil {
		t.Fatal(err)
	}
	checkpoint()
	f1 := importOneFile(t, l, b1.ID, "nested/a.txt", "original bytes")
	imported := checkpoint()
	f2 := importOneFile(t, l, b2.ID, "b.txt", "second bytes")
	checkpoint()
	tag, err := l.CreateTag(ctx, "shared", nil)
	if err != nil {
		t.Fatal(err)
	}
	checkpoint()
	if _, err := l.TagFiles(ctx, tag.ID, []domain.FileID{f1.ID, f2.ID}); err != nil {
		t.Fatal(err)
	}
	tagged := checkpoint()
	if err := l.RenameTag(ctx, tag.ID, "renamed globally"); err != nil {
		t.Fatal(err)
	}
	checkpoint()
	e := &fakeExtractor{id: "fake", version: 1, ns: []string{"fake"}, tags: []extract.KV{text("fake:description", "historical metadata")}}
	extractWith(t, l, e)
	extracted := checkpoint()
	if _, err := l.UpdateBox(ctx, b1.ID, BoxFields{Name: "renamed box"}); err != nil {
		t.Fatal(err)
	}
	checkpoint()
	if _, err := l.Undo(ctx, tagged); err != nil {
		t.Fatal(err)
	}
	checkpoint()
	if _, err := l.Undo(ctx, extracted); err != nil {
		t.Fatal(err)
	}
	checkpoint()
	deleted, err := l.Undo(ctx, imported)
	if err != nil {
		t.Fatal(err)
	}
	checkpoint()
	if _, err := l.Undo(ctx, deleted); err != nil {
		t.Fatal(err)
	}
	checkpoint()
	live := snapshotOf(t, l)
	journalBefore := historyJournalHashes(t, l)
	for cutoff, expected := range snapshots {
		err := l.ViewAt(ctx, cutoff, func(view *HistoricalView) error {
			actual := snapshotOf(t, view.historicalReader.(*Library))
			if actual != expected {
				t.Errorf("prefix %d differs from its original state", cutoff)
			}
			return nil
		})
		if err != nil {
			t.Fatalf("prefix %d: %v", cutoff, err)
		}
	}
	if err := l.ViewAt(ctx, imported, func(view *HistoricalView) error {
		_, content, err := view.OpenContent(ctx, f1.ID)
		if err != nil {
			return err
		}
		defer content.Close()
		data, err := io.ReadAll(content)
		if string(data) != "original bytes" {
			t.Errorf("historical content=%q", data)
		}
		return err
	}); err != nil {
		t.Fatal(err)
	}
	if err := l.ViewAt(ctx, deleted, func(view *HistoricalView) error {
		_, _, err := view.OpenContent(ctx, f1.ID)
		if !errors.Is(err, ErrNoSuchFile) {
			t.Errorf("deleted content: %v", err)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if snapshotOf(t, l) != live || !reflect.DeepEqual(journalBefore, historyJournalHashes(t, l)) {
		t.Fatal("historical reads changed the live state or authoritative journal")
	}
	assertHistoryClean(t, l)
}

func TestHistoricalCleanupAndCutoffValidation(t *testing.T) {
	l, _ := openWithBox(t)
	for _, target := range []int64{0, -1, 100} {
		err := l.ViewAt(t.Context(), target, func(*HistoricalView) error { t.Fatal("invalid cutoff ran callback"); return nil })
		if err == nil {
			t.Fatalf("accepted cutoff %d", target)
		}
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if err := l.ViewAt(ctx, 1, func(*HistoricalView) error { t.Fatal("cancelled callback"); return nil }); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	boom := errors.New("callback failed")
	if err := l.ViewAt(t.Context(), 1, func(*HistoricalView) error { return boom }); !errors.Is(err, boom) {
		t.Fatal(err)
	}
	ctx, cancel = context.WithCancel(t.Context())
	if err := l.ViewAt(ctx, 1, func(*HistoricalView) error { cancel(); return nil }); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	assertHistoryClean(t, l)
}

func TestHistoricalIgnoresFutureTailAndRejectsDamagedPrefix(t *testing.T) {
	l, _ := openWithBox(t)
	paths, err := filepath.Glob(filepath.Join(l.layout.Ops(), "*", "*", "*.jsonl"))
	if err != nil || len(paths) != 1 {
		t.Fatalf("journal paths=%v err=%v", paths, err)
	}
	original, err := os.ReadFile(paths[0])
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(paths[0], append(original, []byte(`{"unfinished":`)...), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := l.ViewAt(t.Context(), 1, func(*HistoricalView) error { return nil }); err != nil {
		t.Fatalf("future tail affected an earlier prefix: %v", err)
	}
	if err := os.WriteFile(paths[0], []byte("invalid prefix\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := l.ViewAt(t.Context(), 1, func(*HistoricalView) error { t.Fatal("damaged history ran callback"); return nil }); err == nil {
		t.Fatal("accepted damaged prefix")
	}
	assertHistoryClean(t, l)
}

func TestHistoricalViewsHaveIndependentLifetimes(t *testing.T) {
	l, box := openWithBox(t)
	if _, err := l.UpdateBox(t.Context(), box, BoxFields{Name: "later"}); err != nil {
		t.Fatal(err)
	}
	err := l.ViewAt(t.Context(), 1, func(first *HistoricalView) error {
		before, found, err := first.Box(t.Context(), box)
		if err != nil || !found {
			t.Fatalf("first: %v %v", found, err)
		}
		if err := l.ViewAt(t.Context(), 2, func(second *HistoricalView) error {
			after, found, err := second.Box(t.Context(), box)
			if err != nil || !found || after.Name != "later" {
				t.Fatalf("second: %+v %v %v", after, found, err)
			}
			return nil
		}); err != nil {
			return err
		}
		still, _, err := first.Box(t.Context(), box)
		if still != before {
			t.Fatal("second view changed the first")
		}
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	assertHistoryClean(t, l)
}

func historyJournalHashes(t *testing.T, l *Library) map[string][32]byte {
	t.Helper()
	hashes := map[string][32]byte{}
	err := filepath.WalkDir(l.layout.Ops(), func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		hashes[path] = sha256.Sum256(data)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return hashes
}

func assertHistoryClean(t *testing.T, l *Library) {
	t.Helper()
	paths, err := filepath.Glob(filepath.Join(l.layout.Root, ".kist-history-*"))
	if err != nil || len(paths) != 0 {
		t.Fatalf("history residue=%v err=%v", paths, err)
	}
}

// The fixture mixes one large import with many small operations. Setup is not
// timed: this measures the cost paid by each historical page request.
func BenchmarkHistoricalPrefix(b *testing.B) {
	root := b.TempDir()
	if _, err := Create(b.Context(), root); err != nil {
		b.Fatal(err)
	}
	l, err := Open(root)
	if err != nil {
		b.Fatal(err)
	}
	defer l.Close()
	box, err := l.CreateBox(b.Context(), "benchmark", "", domain.LocalDate{}, domain.LocalDate{})
	if err != nil {
		b.Fatal(err)
	}
	source := b.TempDir()
	for i := range 4000 {
		if err := os.WriteFile(filepath.Join(source, fmt.Sprintf("%04d.txt", i)), []byte("benchmark"), 0o600); err != nil {
			b.Fatal(err)
		}
	}
	if _, err := l.Import(b.Context(), source, ImportOptions{Box: box.ID}); err != nil {
		b.Fatal(err)
	}
	for i := range 500 {
		if _, err := l.UpdateBox(b.Context(), box.ID, BoxFields{Name: fmt.Sprintf("box-%d", i)}); err != nil {
			b.Fatal(err)
		}
	}
	target, err := l.OpCount(b.Context())
	if err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		if err := l.ViewAt(b.Context(), target, func(view *HistoricalView) error {
			_, _, err := view.Box(b.Context(), box.ID)
			return err
		}); err != nil {
			b.Fatal(err)
		}
	}
}
