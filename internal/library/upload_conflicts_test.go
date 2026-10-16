package library

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/jinzhao1994/kist/internal/domain"
	"github.com/jinzhao1994/kist/internal/extract"
)

func TestUploadOverwritePreservesIdentityUndoRebuildAndHistory(t *testing.T) {
	l, box := openWithBox(t)
	ctx := t.Context()
	first, err := l.Import(ctx, writeTree(t, map[string]string{"a.txt": "old"}), ImportOptions{Box: box})
	if err != nil {
		t.Fatal(err)
	}
	original := listAll(t, l, box)[0]
	tag, err := l.CreateTag(ctx, "manual", nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = l.TagFiles(ctx, tag.ID, []domain.FileID{original.ID}); err != nil {
		t.Fatal(err)
	}
	if _, err = l.RemoveMetadata(ctx, original.ID, "file:mtime"); err != nil {
		t.Fatal(err)
	}
	extractWith(t, l, &fakeExtractor{id: "test", version: 1, ns: []string{"test"}, tags: []extract.KV{text("test:old", "old metadata")}})
	before, _, err := l.entrySnapshot(ctx, box, original.ID.ID, false)
	if err != nil {
		t.Fatal(err)
	}
	opt := ImportOptions{Box: box, ConflictMode: "overwrite"}
	replaced, err := l.Import(ctx, writeTree(t, map[string]string{"a.txt": "new", "b.txt": "added"}), opt)
	if err != nil {
		t.Fatal(err)
	}
	current, ok, err := l.File(ctx, original.ID)
	if err != nil || !ok || current.BlobKey == original.BlobKey || current.Name != original.Name {
		t.Fatalf("replacement: %+v %v", current, err)
	}
	after, _, err := l.entrySnapshot(ctx, box, original.ID.ID, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(after.Files[0].Extractions) != 0 {
		t.Fatal("old extraction state retained")
	}
	for _, m := range after.Files[0].Metadata {
		if m.Source == "test" {
			t.Fatal("old extracted metadata retained")
		}
	}
	if !reflect.DeepEqual(after.Files[0].Tags, before.Files[0].Tags) || after.Files[0].Metadata[0].Value != nil {
		t.Fatal("manual annotations lost", after)
	}
	checkRebuilt(t, l)
	if err := l.ViewAt(ctx, first.OpID, func(v *HistoricalView) error {
		f, found, err := v.File(ctx, original.ID)
		if err != nil || !found || f.BlobKey != original.BlobKey {
			t.Fatalf("historical content: %+v %v", f, err)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := l.ViewAt(ctx, replaced.OpID, func(v *HistoricalView) error {
		f, found, err := v.File(ctx, original.ID)
		if err != nil || !found || f.BlobKey != current.BlobKey {
			t.Fatalf("replacement snapshot: %+v %v", f, err)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	undone, err := l.Undo(ctx, replaced.OpID)
	if err != nil {
		t.Fatal(err)
	}
	restored := listAll(t, l, box)
	if len(restored) != 1 || restored[0].ID != original.ID || restored[0].BlobKey != original.BlobKey {
		t.Fatal(restored)
	}
	checkRebuilt(t, l)
	if _, err = l.Undo(ctx, undone); err != nil {
		t.Fatal(err)
	}
	checkRebuilt(t, l)
	report, err := l.Verify(ctx, VerifyOptions{Deep: true})
	if err != nil || report.Total() != 0 {
		t.Fatalf("verify: %+v %v", report, err)
	}
}

func TestUploadKeepBothAlsoKeepsIdenticalContent(t *testing.T) {
	l, box := openWithBox(t)
	ctx := t.Context()
	src := writeTree(t, map[string]string{"a.txt": "same"})
	if _, err := l.Import(ctx, src, ImportOptions{Box: box}); err != nil {
		t.Fatal(err)
	}
	if _, err := l.Import(ctx, src, ImportOptions{Box: box, ConflictMode: "keep_both"}); err != nil {
		t.Fatal(err)
	}
	if got := fileNames(listAll(t, l, box)); !reflect.DeepEqual(got, []string{"a (2).txt", "a.txt"}) {
		t.Fatal(got)
	}
	checkRebuilt(t, l)
}
func TestPreparedOverwriteRefusesChangedFile(t *testing.T) {
	l, box := openWithBox(t)
	ctx := t.Context()
	src := writeTree(t, map[string]string{"a.txt": "old"})
	if _, err := l.Import(ctx, src, ImportOptions{Box: box}); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "a.txt"), []byte("new"), 0600); err != nil {
		t.Fatal(err)
	}
	prepared, err := l.PrepareImport(ctx, src, ImportOptions{Box: box, ConflictMode: "overwrite"})
	if err != nil {
		t.Fatal(err)
	}
	original := listAll(t, l, box)[0]
	if _, err := l.RenameEntry(ctx, box, original.ID.ID, false, "a.txt", "changed.txt"); err != nil {
		t.Fatal(err)
	}
	if _, err := prepared.Commit(ctx); err == nil {
		t.Fatal("overwrote changed file")
	}
	current, _, _ := l.File(ctx, original.ID)
	if current.BlobKey != original.BlobKey {
		t.Fatal("content changed")
	}
}

func TestUploadFolderCollisionOnlyAllowsKeepBoth(t *testing.T) {
	l, box := openWithBox(t)
	ctx := t.Context()
	if _, err := l.Import(ctx, writeTree(t, map[string]string{"a.txt/child": "old"}), ImportOptions{Box: box}); err != nil {
		t.Fatal(err)
	}
	b, _, _ := l.Box(ctx, box)
	preview, err := l.PreviewUpload(ctx, box, b.RootFolderID, []string{"a.txt"})
	if err != nil {
		t.Fatal(err)
	}
	if preview.CanOverwrite || len(preview.Paths) != 1 {
		t.Fatal(preview)
	}
	src := writeTree(t, map[string]string{"a.txt": "new"})
	if _, err = l.Import(ctx, src, ImportOptions{Box: box, ConflictMode: "overwrite"}); err == nil {
		t.Fatal("replaced folder")
	}
	if _, err = l.Import(ctx, src, ImportOptions{Box: box, ConflictMode: "keep_both"}); err != nil {
		t.Fatal(err)
	}
	if got := fileNames(listAll(t, l, box)); !reflect.DeepEqual(got, []string{"a (2).txt", "child"}) {
		t.Fatal(got)
	}
	checkRebuilt(t, l)
}

func TestUploadUndoRefusesLaterAnnotation(t *testing.T) {
	l, box := openWithBox(t)
	ctx := t.Context()
	if _, err := l.Import(ctx, writeTree(t, map[string]string{"a": "old"}), ImportOptions{Box: box}); err != nil {
		t.Fatal(err)
	}
	result, err := l.Import(ctx, writeTree(t, map[string]string{"a": "new"}), ImportOptions{Box: box, ConflictMode: "overwrite"})
	if err != nil {
		t.Fatal(err)
	}
	f := listAll(t, l, box)[0]
	if _, err := l.RemoveMetadata(ctx, f.ID, "file:mtime"); err != nil {
		t.Fatal(err)
	}
	if _, err := l.Undo(ctx, result.OpID); err == nil {
		t.Fatal("undo discarded a later annotation")
	}
}
