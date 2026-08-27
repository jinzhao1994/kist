package library

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/jinzhao1994/kist/internal/domain"
	"github.com/jinzhao1994/kist/internal/store"
)

// openWithBox opens a fresh library holding one empty box.
func openWithBox(t *testing.T) (*Library, domain.BoxID) {
	t.Helper()
	l := openTestLibrary(t)
	b, err := l.CreateBox(t.Context(), "云南", "", domain.LocalDate{}, domain.LocalDate{})
	if err != nil {
		t.Fatal(err)
	}
	return l, b.ID
}

// writeTree lays a map of slash paths onto a fresh temp directory.
func writeTree(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	for rel, content := range files {
		p := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func listAll(t *testing.T, l *Library, box domain.BoxID) []domain.File {
	t.Helper()
	got, _, err := store.ListFiles(l.db, store.ListFilesOptions{Box: &box})
	if err != nil {
		t.Fatal(err)
	}
	return got
}

func fileNames(files []domain.File) []string {
	out := make([]string, len(files))
	for i, f := range files {
		out[i] = f.Name
	}
	slices.Sort(out)
	return out
}

func describe(r *ImportResult) string {
	var b strings.Builder
	for _, f := range r.Files {
		fmt.Fprintf(&b, "\n  %-10s %s", f.Status, f.Rel)
		if f.Err != nil {
			fmt.Fprintf(&b, " (%v)", f.Err)
		}
	}
	return b.String()
}

// Re-running an interrupted import must converge, not pile up copies.
//
// This is the whole reason spec 3.7.1 ② exists: with only "rename on conflict"
// from spec 1.5, a Ctrl-C at 3000 of 5000 followed by a re-run gives every one
// of those 3000 files a " (2)" twin.
func TestImportIsIdempotent(t *testing.T) {
	l, box := openWithBox(t)
	src := writeTree(t, map[string]string{
		"a.txt":      "alpha",
		"day1/b.txt": "bravo",
	})

	first, err := l.Import(t.Context(), src, ImportOptions{Box: box})
	if err != nil {
		t.Fatal(err)
	}
	if n := first.Count(FileImported); n != 2 {
		t.Fatalf("first run imported %d files, want 2: %s", n, describe(first))
	}

	second, err := l.Import(t.Context(), src, ImportOptions{Box: box})
	if err != nil {
		t.Fatal(err)
	}
	if n := second.Count(FileSkipped); n != 2 {
		t.Errorf("second run skipped %d files, want 2: %s", n, describe(second))
	}
	if n := second.Count(FileImported); n != 0 {
		t.Errorf("second run imported %d files; nothing changed on disk", n)
	}
	if second.OpID != 0 {
		t.Error("a run that changed nothing still wrote an op")
	}

	if got := listAll(t, l, box); len(got) != 2 {
		t.Errorf("library holds %d files after two identical imports: %v", len(got), fileNames(got))
	}
	// The folder must be reused too, not recreated.
	folders, err := store.FoldersOfBox(l.db, box)
	if err != nil {
		t.Fatal(err)
	}
	if len(folders) != 2 {
		t.Errorf("%d folders, want root + day1", len(folders))
	}
}

// Same name, different content is the case that does get numbered.
func TestImportNumbersOnlyOnDifferentContent(t *testing.T) {
	l, box := openWithBox(t)

	src1 := writeTree(t, map[string]string{"发票.jpg": "one"})
	if _, err := l.Import(t.Context(), src1, ImportOptions{Box: box}); err != nil {
		t.Fatal(err)
	}
	src2 := writeTree(t, map[string]string{"发票.jpg": "two"})
	if _, err := l.Import(t.Context(), src2, ImportOptions{Box: box}); err != nil {
		t.Fatal(err)
	}

	got := fileNames(listAll(t, l, box))
	want := []string{"发票 (2).jpg", "发票.jpg"}
	slices.Sort(want)
	if !slices.Equal(got, want) {
		t.Errorf("names = %v, want %v", got, want)
	}
}

// Two files with the same name arriving in one batch must not collide with
// each other either — the taken set has to include what this batch just added,
// because the database will not see it until the transaction at the end.
func TestImportNumbersWithinOneBatch(t *testing.T) {
	l, box := openWithBox(t)
	src := writeTree(t, map[string]string{
		"a/发票.jpg": "one",
		"b/发票.jpg": "two",
	})

	// Import each subdirectory into the box root, so both names land in the
	// same folder within a single library.
	if _, err := l.Import(t.Context(), filepath.Join(src, "a"), ImportOptions{Box: box}); err != nil {
		t.Fatal(err)
	}
	res, err := l.Import(t.Context(), filepath.Join(src, "b"), ImportOptions{Box: box})
	if err != nil {
		t.Fatal(err)
	}
	if res.Count(FileImported) != 1 {
		t.Fatalf("second import: %s", describe(res))
	}
	if got := listAll(t, l, box); len(got) != 2 {
		t.Errorf("%d files, want both: %v", len(got), fileNames(got))
	}
}

// A cancelled context is a graceful stop: what was finished is committed as
// one op, and that op is undoable like any other.
//
// The trap this test exists to catch is committing on the cancelled context,
// which fails at BeginTx and turns "stop here" into "lose everything".
func TestImportInterruptedCommitsWhatItFinished(t *testing.T) {
	l, box := openWithBox(t)
	src := writeTree(t, map[string]string{
		"01.txt": "a", "02.txt": "b", "03.txt": "c", "04.txt": "d", "05.txt": "e",
	})

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	n := 0
	l.hooks.afterFile = func() {
		if n++; n == 2 {
			cancel()
		}
	}

	res, err := l.Import(ctx, src, ImportOptions{Box: box})
	if err != nil {
		t.Fatalf("an interrupted import must not fail: %v", err)
	}
	if !res.Interrupted {
		t.Error("Interrupted is false")
	}
	if res.Remaining != 3 {
		t.Errorf("Remaining = %d, want 3", res.Remaining)
	}
	if res.OpID == 0 {
		t.Fatal("no op was written; the finished files were lost")
	}
	if got := listAll(t, l, box); len(got) != 2 {
		t.Errorf("%d files in the library, want the 2 that finished: %v", len(got), fileNames(got))
	}

	// And it undoes as a unit, like any other op.
	if _, err := l.Undo(t.Context(), res.OpID); err != nil {
		t.Fatalf("undo: %v", err)
	}
	if got := listAll(t, l, box); len(got) != 0 {
		t.Errorf("%d files left after undoing the interrupted import", len(got))
	}
}

// One unreadable file must not take the batch with it, and it must not enter
// the payload — that is what keeps the op atomic even though the batch is not
// (spec 3.7.1 ③).
func TestImportKeepsGoingAfterOneBadFile(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("chmod 0 does not stop a read on Windows")
	}
	l, box := openWithBox(t)
	src := writeTree(t, map[string]string{
		"good1.txt": "a",
		"locked":    "b",
		"good2.txt": "c",
	})
	if err := os.Chmod(filepath.Join(src, "locked"), 0); err != nil {
		t.Skip("cannot make a file unreadable here")
	}
	if os.Geteuid() == 0 {
		t.Skip("running as root; mode 0 does not stop a read")
	}

	res, err := l.Import(t.Context(), src, ImportOptions{Box: box})
	if err != nil {
		t.Fatalf("one bad file must not fail the batch: %v", err)
	}
	if got := res.Count(FileImported); got != 2 {
		t.Errorf("%d imported, want 2: %s", got, describe(res))
	}
	if got := res.Count(FileFailed); got != 1 {
		t.Errorf("%d failed, want 1: %s", got, describe(res))
	}
	if got := listAll(t, l, box); len(got) != 2 {
		t.Errorf("%d files in the library, want 2: %v", len(got), fileNames(got))
	}
}

// A name that cannot be projected identically on every platform is a per-file
// failure with an actionable message, not a silent rename (spec 3.4).
func TestImportRejectsAnUnprojectableName(t *testing.T) {
	l, box := openWithBox(t)
	src := writeTree(t, map[string]string{
		"fine.txt":   "a",
		"trailing. ": "b",
	})

	res, err := l.Import(t.Context(), src, ImportOptions{Box: box})
	if err != nil {
		t.Fatal(err)
	}
	if got := res.Count(FileFailed); got != 1 {
		t.Errorf("%d failed, want 1: %s", got, describe(res))
	}
	if got := res.Count(FileImported); got != 1 {
		t.Errorf("%d imported, want the other one: %s", got, describe(res))
	}
}

// Dot-files are operating-system droppings by default, archive material only
// on request.
func TestImportSkipsHiddenFilesUnlessAsked(t *testing.T) {
	l, box := openWithBox(t)
	src := writeTree(t, map[string]string{
		"photo.jpg": "a",
		".DS_Store": "junk",
		".git/HEAD": "junk",
	})

	res, err := l.Import(t.Context(), src, ImportOptions{Box: box})
	if err != nil {
		t.Fatal(err)
	}
	if got := len(res.Files); got != 1 {
		t.Errorf("%d candidates, want only the photo: %s", got, describe(res))
	}

	l2, box2 := openWithBox(t)
	res, err = l2.Import(t.Context(), src, ImportOptions{Box: box2, Hidden: true})
	if err != nil {
		t.Fatal(err)
	}
	if got := res.Count(FileImported); got != 3 {
		t.Errorf("%d imported with --hidden, want 3: %s", got, describe(res))
	}
}

// Importing a single file is the same operation with one candidate.
func TestImportASingleFile(t *testing.T) {
	l, box := openWithBox(t)
	src := writeTree(t, map[string]string{"one.txt": "a"})

	res, err := l.Import(t.Context(), filepath.Join(src, "one.txt"), ImportOptions{Box: box})
	if err != nil {
		t.Fatal(err)
	}
	if res.Count(FileImported) != 1 {
		t.Errorf("%s", describe(res))
	}
	if got := listAll(t, l, box); len(got) != 1 || got[0].Name != "one.txt" {
		t.Errorf("files = %v", fileNames(got))
	}
}

// Two files with identical content share one blob but still get their own row.
func TestImportDeduplicatesContentNotRows(t *testing.T) {
	l, box := openWithBox(t)
	src := writeTree(t, map[string]string{"a.txt": "same", "b.txt": "same"})

	if _, err := l.Import(t.Context(), src, ImportOptions{Box: box}); err != nil {
		t.Fatal(err)
	}
	if got := listAll(t, l, box); len(got) != 2 {
		t.Fatalf("%d files, want 2", len(got))
	}
	var blobs int
	if err := l.db.QueryRow(`SELECT count(*) FROM blobs`).Scan(&blobs); err != nil {
		t.Fatal(err)
	}
	if blobs != 1 {
		t.Errorf("%d blobs, want 1 — identical content is stored once", blobs)
	}
}

func TestImportRefusesAnUnknownBox(t *testing.T) {
	l := openTestLibrary(t)
	src := writeTree(t, map[string]string{"a.txt": "a"})
	if _, err := l.Import(t.Context(), src, ImportOptions{Box: domain.NewBoxID()}); err == nil {
		t.Error("importing into a box that does not exist was accepted")
	}
}

// The file in flight when the context is cancelled is not a failure.
//
// Found by interrupting a real 4000-file import: the summary said
// "imported 116, skipped 0, failed 1" with a "failed: context canceled" line
// and a non-zero exit. The user pressed Ctrl-C — nothing went wrong with their
// file, and saying otherwise makes "I stopped it" look like "my data is bad".
func TestImportDoesNotReportTheInterruptedFileAsFailed(t *testing.T) {
	l, box := openWithBox(t)
	files := map[string]string{}
	for i := range 20 {
		files[fmt.Sprintf("f%02d.txt", i)] = fmt.Sprintf("content %d", i)
	}
	src := writeTree(t, files)

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	n := 0
	l.hooks.afterFile = func() {
		if n++; n == 3 {
			cancel()
		}
	}

	res, err := l.Import(ctx, src, ImportOptions{Box: box})
	if err != nil {
		t.Fatal(err)
	}
	if !res.Interrupted {
		t.Fatal("Interrupted is false")
	}
	if got := res.Count(FileFailed); got != 0 {
		t.Errorf("%d files reported as failed: %s", got, describe(res))
	}
	if got := res.Count(FileImported); got != 3 {
		t.Errorf("%d imported, want the 3 that finished: %s", got, describe(res))
	}
	// The one that was cut short counts as not looked at, not as done.
	if res.Remaining != 17 {
		t.Errorf("Remaining = %d, want 17", res.Remaining)
	}
}
