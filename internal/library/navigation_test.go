package library

import (
	"context"
	"errors"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jinzhao1994/kist/internal/domain"
	"github.com/jinzhao1994/kist/internal/query"
)

type navigationReader interface {
	Navigation(context.Context, NavigationTarget) (NavigationLocation, error)
}

func TestNavigationLogicalPathsAndStableIdentity(t *testing.T) {
	l, box := openWithBox(t)
	ctx := t.Context()
	date, _ := domain.ParseLocalDate("2026-10")
	if _, err := l.UpdateBox(ctx, box, BoxFields{Name: "旅途 #?%", StartAt: date}); err != nil {
		t.Fatal(err)
	}
	// Archive names are logical paths; staging names must be valid on every host.
	prepared, err := l.PrepareFiles(ctx, []ImportSource{
		clientImportSource(t, "第一天/西湖 #?%2F.jpg", "image"),
		clientImportSource(t, "第二天/other.txt", "other"),
	}, ImportOptions{Box: box})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := prepared.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	files, _, _ := l.Files(ctx, query.Query{})
	var file domain.File
	for _, f := range files {
		if strings.HasPrefix(f.Name, "西湖") {
			file = f
		}
	}
	client := newSyncClient(t, l)
	target := NavigationTarget{File: file.ID.ID}
	original, err := l.Navigation(ctx, target)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(original.Path, "2026-10_") || !strings.Contains(original.Path, "%252F") {
		t.Fatal(original)
	}
	for _, reader := range []navigationReader{l, client} {
		got, err := reader.Navigation(ctx, NavigationTarget{Path: original.Path})
		if err != nil || got != original {
			t.Fatalf("round trip: %+v %v", got, err)
		}
	}
	// Paths resolve from logical state even when the readable disk projection is absent.
	if err := os.Rename(filepath.Join(l.layout.Root, "boxes"), filepath.Join(l.layout.Root, "boxes-hidden")); err != nil {
		t.Fatal(err)
	}
	if got, err := l.Navigation(ctx, target); err != nil || got != original {
		t.Fatalf("projection dependent: %+v %v", got, err)
	}
	if _, err := l.RenameEntry(ctx, box, file.ID.ID, false, file.Name, "renamed.jpg"); err != nil {
		t.Fatal(err)
	}
	after, err := l.Navigation(ctx, target)
	if err != nil || after.File != original.File || after.Path == original.Path {
		t.Fatalf("rename: %+v %v", after, err)
	}
	if _, err := l.Navigation(ctx, NavigationTarget{Path: original.Path}); !errors.Is(err, ErrNoSuchFile) {
		t.Fatalf("old alias retained: %v", err)
	}
	prepared, err = l.PrepareFiles(ctx, []ImportSource{
		clientImportSource(t, "第一天/西湖 #?%2F.jpg", "replacement"),
	}, ImportOptions{Box: box})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := prepared.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	replacement, err := l.Navigation(ctx, NavigationTarget{Path: original.Path})
	if err != nil || replacement.File == original.File {
		t.Fatalf("reused path: %+v %v", replacement, err)
	}
	for _, bad := range []string{"/boxes/%", "/boxes/%2e%2e", "/boxes/x/%2F", "/boxes/x//y", "/boxes/x/%00", "/boxes/x/../y"} {
		if _, err := l.Navigation(ctx, NavigationTarget{Path: bad}); !errors.Is(err, ErrInvalidRequest) {
			t.Fatalf("unsafe path %q: %v", bad, err)
		}
	}
	// Exact decoding: the literal %2F in the original filename is not a separator.
	if _, err := client.Navigation(ctx, NavigationTarget{Path: strings.Replace(original.Path, "%252F", "%2F", 1)}); !errors.Is(err, ErrInvalidRequest) {
		t.Fatal(err)
	}
}

func TestNavigationDuplicateBoxesAndNestedFilesets(t *testing.T) {
	l, box := openWithBox(t)
	ctx := t.Context()
	b, _, _ := l.Box(ctx, box)
	second, err := l.CreateBox(ctx, b.Name, "", b.StartAt, b.EndAt)
	if err != nil {
		t.Fatal(err)
	}
	a, err := l.Navigation(ctx, NavigationTarget{Box: box})
	if err != nil {
		t.Fatal(err)
	}
	bnav, err := l.Navigation(ctx, NavigationTarget{Box: second.ID})
	if err != nil || a.Path == bnav.Path {
		t.Fatalf("duplicate: %+v %v", bnav, err)
	}
	for _, expected := range []NavigationLocation{a, bnav} {
		got, err := l.Navigation(ctx, NavigationTarget{Path: expected.Path})
		if err != nil || got.Box != expected.Box {
			t.Fatalf("ambiguous: %+v %v", got, err)
		}
	}
	if _, err := l.Import(ctx, writeTree(t, map[string]string{"outer/inner/page.jpg": "page"}), ImportOptions{Box: box}); err != nil {
		t.Fatal(err)
	}
	folders, _ := l.Folders(ctx, box)
	var inner, outer domain.FolderID
	for _, f := range folders {
		if f.Name == "inner" {
			inner = f.ID
		}
		if f.Name == "outer" {
			outer = f.ID
		}
	}
	files, _, _ := l.Files(ctx, query.Query{})
	child := domain.Fileset{FolderID: inner, Kind: "document_pages", Layout: "flat", Members: []domain.FilesetMember{{FileID: files[0].ID, Role: "page"}}}
	if _, err := l.SetFileset(ctx, box, inner, nil, &child); err != nil {
		t.Fatal(err)
	}
	parent := domain.Fileset{FolderID: outer, Kind: "document_pages", Layout: "flat", Members: []domain.FilesetMember{{FolderID: inner, Role: "page"}}}
	if _, err := l.SetFileset(ctx, box, outer, nil, &parent); err != nil {
		t.Fatal(err)
	}
	client := newSyncClient(t, l)
	for _, reader := range []navigationReader{l, client} {
		for _, id := range []domain.FolderID{outer, inner} {
			set, err := reader.Navigation(ctx, NavigationTarget{File: id.ID})
			if err != nil || set.Kind != "fileset" {
				t.Fatalf("fileset: %+v %v", set, err)
			}
			got, err := reader.Navigation(ctx, NavigationTarget{Path: set.Path})
			if err != nil || got != set {
				t.Fatalf("fileset path: %+v %v", got, err)
			}
			folder, err := reader.Navigation(ctx, NavigationTarget{Path: set.Path, FolderView: true})
			if err != nil || folder.Folder != id.String() || folder.File != "" || folder.View != "folder" {
				t.Fatalf("folder view: %+v %v", folder, err)
			}
		}
	}
	// A directory named id inside a box is ordinary content.
	if _, err := l.CreateFolder(ctx, box, b.RootFolderID, "id"); err != nil {
		t.Fatal(err)
	}
	if got, err := l.Navigation(ctx, NavigationTarget{Path: a.Path + "/" + url.PathEscape("id")}); err != nil || got.Kind != "folder" {
		t.Fatalf("reserved child: %+v %v", got, err)
	}
}

func TestNavigationIdentityDoesNotDependOnAncestors(t *testing.T) {
	l, box := openWithBox(t)
	ctx := t.Context()
	file := importOneFile(t, l, box, "document.txt", "bytes")
	original, err := l.Navigation(ctx, NavigationTarget{File: file.ID.ID})
	if err != nil {
		t.Fatal(err)
	}
	other, err := l.CreateBox(ctx, "Destination", "", domain.LocalDate{}, domain.LocalDate{})
	if err != nil {
		t.Fatal(err)
	}
	// Simulate a relocated logical object in the disposable view. URL resolution
	// must derive its current ancestors, even if an old link includes stale scope.
	if _, err := l.db.Exec(`UPDATE files SET folder_id=? WHERE id=?`, other.RootFolderID, file.ID); err != nil {
		t.Fatal(err)
	}
	moved, err := l.Navigation(ctx, NavigationTarget{Box: box, Folder: file.FolderID, File: file.ID.ID})
	if err != nil || moved.Box != other.ID.String() || moved.File != file.ID.String() || moved.Path == original.Path {
		t.Fatalf("moved: %+v %v", moved, err)
	}
	if _, err := l.Navigation(ctx, NavigationTarget{Path: original.Path}); !errors.Is(err, ErrNoSuchFile) {
		t.Fatalf("old location: %v", err)
	}
	if err := l.Rebuild(ctx); err != nil {
		t.Fatal(err)
	}
	restored, err := l.Navigation(ctx, NavigationTarget{File: file.ID.ID})
	if err != nil || restored != original {
		t.Fatalf("rebuild: %+v %v", restored, err)
	}
}

func TestNavigationAncestorRenameAndBoxDateChange(t *testing.T) {
	l, box := openWithBox(t)
	ctx := t.Context()
	if _, err := l.Import(ctx, writeTree(t, map[string]string{"before/report.txt": "report"}), ImportOptions{Box: box}); err != nil {
		t.Fatal(err)
	}
	files, _, err := l.Files(ctx, query.Query{})
	if err != nil || len(files) != 1 {
		t.Fatalf("files: %v %v", files, err)
	}
	file := files[0]
	original, err := l.Navigation(ctx, NavigationTarget{File: file.ID.ID})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := l.RenameEntry(ctx, box, file.FolderID.ID, true, "before", "after"); err != nil {
		t.Fatal(err)
	}
	renamed, err := l.Navigation(ctx, NavigationTarget{File: file.ID.ID})
	if err != nil || renamed.File != original.File || !strings.HasSuffix(renamed.Path, "/after/report.txt") {
		t.Fatalf("ancestor rename: %+v %v", renamed, err)
	}
	if _, err := l.Navigation(ctx, NavigationTarget{Path: original.Path}); !errors.Is(err, ErrNoSuchFile) {
		t.Fatalf("old ancestor alias: %v", err)
	}
	date, _ := domain.ParseLocalDate("2025")
	if _, err := l.UpdateBox(ctx, box, BoxFields{Name: "new box", StartAt: date}); err != nil {
		t.Fatal(err)
	}
	changed, err := l.Navigation(ctx, NavigationTarget{File: file.ID.ID})
	if err != nil || changed.File != original.File || !strings.Contains(changed.Path, "2025_new%20box/") {
		t.Fatalf("box rename/date: %+v %v", changed, err)
	}
	if _, err := l.Navigation(ctx, NavigationTarget{Path: renamed.Path}); !errors.Is(err, ErrNoSuchBox) {
		t.Fatalf("old box alias: %v", err)
	}
	client := newSyncClient(t, l)
	if got, err := client.Navigation(ctx, NavigationTarget{Path: changed.Path}); err != nil || got != changed {
		t.Fatalf("client: %+v %v", got, err)
	}
}
