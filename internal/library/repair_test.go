package library

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/jinzhao1994/kist/internal/config"
	"github.com/jinzhao1994/kist/internal/integrity"
)

// mirrorBlobs copies the library's blobs/ somewhere else and registers it as a
// remote, which is what `rsync -a blobs/ /backup/blobs/` does in real life
// (spec 3.11).
func mirrorBlobs(t *testing.T, l *Library, name string) string {
	t.Helper()
	dst := t.TempDir()
	src := l.Layout().Blobs()
	err := filepath.WalkDir(src, func(p string, e fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, p)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if e.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		body, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		return os.WriteFile(target, body, 0o444)
	})
	if err != nil {
		t.Fatal(err)
	}
	l.settings.Load().config.Remotes = append(l.settings.Load().config.Remotes, config.Remote{Name: name, Path: dst})
	return dst
}

// The end-to-end promise of M2a: a library whose content was damaged comes
// back from a second copy.
func TestRepairRestoresFromARemote(t *testing.T) {
	l, box := openWithBox(t)
	src := writeTree(t, map[string]string{"passport.jpg": "scan", "a.txt": "x"})
	if _, err := l.Import(t.Context(), src, ImportOptions{Box: box}); err != nil {
		t.Fatal(err)
	}
	mirrorBlobs(t, l, "backup")
	overwriteSomeBlob(t, l, "ROT!")

	before, err := l.Scrub(t.Context(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if before.Count(integrity.BlobCorrupt) != 1 {
		t.Fatalf("the fixture is not damaged:\n%s", report(t, before))
	}

	res, err := l.Repair(t.Context(), RepairOptions{Apply: true, Deep: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Repaired) != 1 {
		t.Fatalf("repaired %d, want 1 (unfixable %v)", len(res.Repaired), res.Unfixable)
	}
	if res.Repaired[0].Remote != "backup" {
		t.Errorf("came from %q, want backup", res.Repaired[0].Remote)
	}
	// The report has to name the file, not only the digest (spec 3.8).
	if len(res.Repaired[0].Files) == 0 {
		t.Error("the repaired entry does not say which file it was")
	}

	after, err := l.Scrub(t.Context(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if after.Total() != 0 {
		t.Errorf("the library is still damaged:\n%s", report(t, after))
	}
}

// The test that makes "you may point kist at a drive you do not fully trust"
// true. A remote serving the wrong bytes must change nothing.
func TestRepairRejectsARemoteThatServesWrongContent(t *testing.T) {
	l, box := openWithBox(t)
	src := writeTree(t, map[string]string{"passport.jpg": "scan"})
	if _, err := l.Import(t.Context(), src, ImportOptions{Box: box}); err != nil {
		t.Fatal(err)
	}
	remote := mirrorBlobs(t, l, "liar")

	// The remote has something under the right key, with the wrong content.
	err := filepath.WalkDir(remote, func(p string, e fs.DirEntry, err error) error {
		if err != nil || e.IsDir() {
			return err
		}
		if err := os.Chmod(p, 0o644); err != nil {
			return err
		}
		return os.WriteFile(p, []byte("HAHA"), 0o444)
	})
	if err != nil {
		t.Fatal(err)
	}
	overwriteSomeBlob(t, l, "ROT!")
	damaged := someBlobPath(t, l)

	res, err := l.Repair(t.Context(), RepairOptions{Apply: true, Deep: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Repaired) != 0 {
		t.Errorf("repaired %v from a remote serving wrong content", res.Repaired)
	}
	if len(res.Unfixable) != 1 {
		t.Errorf("unfixable = %+v, want the one damaged blob", res.Unfixable)
	}
	if len(res.Unfixable) == 1 && len(res.Unfixable[0].Files) != 1 {
		t.Errorf("the unfixable entry does not name the file it belongs to: %+v", res.Unfixable[0])
	}
	body, err := os.ReadFile(damaged)
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != "ROT!" {
		t.Errorf("the local blob was overwritten with %q", body)
	}
}

// A dry run must not write. It is the default, so it is what almost every
// invocation does.
func TestRepairDryRunWritesNothing(t *testing.T) {
	l, box := openWithBox(t)
	src := writeTree(t, map[string]string{"passport.jpg": "scan"})
	if _, err := l.Import(t.Context(), src, ImportOptions{Box: box}); err != nil {
		t.Fatal(err)
	}
	mirrorBlobs(t, l, "backup")
	overwriteSomeBlob(t, l, "ROT!")
	damaged := someBlobPath(t, l)

	res, err := l.Repair(t.Context(), RepairOptions{Deep: true})
	if err != nil {
		t.Fatal(err)
	}
	if !res.DryRun {
		t.Error("the result does not say it was a dry run")
	}
	if len(res.Repaired) != 1 {
		t.Errorf("a dry run must still report what it could fix: %+v", res)
	}
	body, err := os.ReadFile(damaged)
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != "ROT!" {
		t.Errorf("the dry run wrote to blobs/: %q", body)
	}
}

// No remotes is the state every new library is in. It must be a clear message
// rather than "repaired 0", which reads like good news.
func TestRepairWithNoRemotesSaysSo(t *testing.T) {
	l, box := openWithBox(t)
	src := writeTree(t, map[string]string{"passport.jpg": "scan"})
	if _, err := l.Import(t.Context(), src, ImportOptions{Box: box}); err != nil {
		t.Fatal(err)
	}
	overwriteSomeBlob(t, l, "ROT!")

	_, err := l.Repair(t.Context(), RepairOptions{Deep: true})
	if !errors.Is(err, ErrNoRemotes) {
		t.Errorf("err = %v, want ErrNoRemotes", err)
	}
}

// A healthy library needs no remotes at all, so having none must not be an
// error when there is nothing to fix.
func TestRepairOfAHealthyLibraryIsQuiet(t *testing.T) {
	l, box := openWithBox(t)
	src := writeTree(t, map[string]string{"a.txt": "x"})
	if _, err := l.Import(t.Context(), src, ImportOptions{Box: box}); err != nil {
		t.Fatal(err)
	}

	res, err := l.Repair(t.Context(), RepairOptions{Deep: true})
	if err != nil {
		t.Fatalf("a healthy library with no remotes should be fine: %v", err)
	}
	if len(res.Broken) != 0 {
		t.Errorf("broken = %v", res.Broken)
	}
}

// An unplugged drive must not stop the repair — another remote may have it.
func TestRepairKeepsGoingWhenARemoteIsUnreachable(t *testing.T) {
	l, box := openWithBox(t)
	src := writeTree(t, map[string]string{"passport.jpg": "scan"})
	if _, err := l.Import(t.Context(), src, ImportOptions{Box: box}); err != nil {
		t.Fatal(err)
	}
	l.settings.Load().config.Remotes = append(l.settings.Load().config.Remotes, config.Remote{
		Name: "unplugged", Path: filepath.Join(t.TempDir(), "not-mounted"),
	})
	mirrorBlobs(t, l, "backup")
	overwriteSomeBlob(t, l, "ROT!")

	res, err := l.Repair(t.Context(), RepairOptions{Apply: true, Deep: true})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := res.Unreachable["unplugged"]; !ok {
		t.Error("the unreachable remote was not reported")
	}
	if len(res.Repaired) != 1 {
		t.Errorf("repaired %d, want 1 — the working remote had it", len(res.Repaired))
	}
}
