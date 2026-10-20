package library

import (
	"github.com/jinzhao1994/kist/internal/domain"
	"github.com/jinzhao1994/kist/internal/integrity"
	"github.com/jinzhao1994/kist/internal/query"
	"github.com/jinzhao1994/kist/internal/store"
	"reflect"
	"testing"
)

func TestFilesetLifecycleAndAtomicOrder(t *testing.T) {
	l, box := openWithBox(t)
	ctx := t.Context()
	_, err := l.Import(ctx, writeTree(t, map[string]string{"document/a.jpg": "a", "document/b.jpg": "bb", "document/page/c.jpg": "ccc", "document/page/c.mov": "video"}), ImportOptions{Box: box})
	if err != nil {
		t.Fatal(err)
	}
	folders, _ := l.Folders(ctx, box)
	byName := map[string]domain.FolderID{}
	for _, f := range folders {
		byName[f.Name] = f.ID
	}
	files, _, err := l.Files(ctx, query.Query{})
	if err != nil {
		t.Fatal(err)
	}
	byFile := map[string]domain.FileID{}
	for _, f := range files {
		byFile[f.Name] = f.ID
	}
	child := domain.Fileset{FolderID: byName["page"], Kind: "live_photo", Layout: "flat", Members: []domain.FilesetMember{{FileID: byFile["c.jpg"], Role: "image"}, {FileID: byFile["c.mov"], Role: "video"}}}
	if _, err = l.SetFileset(ctx, box, child.FolderID, nil, &child); err != nil {
		t.Fatal(err)
	}
	parent := domain.Fileset{FolderID: byName["document"], Kind: "document_pages", Layout: "flat", Members: []domain.FilesetMember{{FileID: byFile["a.jpg"], Role: "page"}, {FolderID: child.FolderID, Role: "page"}, {FileID: byFile["b.jpg"], Role: "page"}}}
	create, err := l.SetFileset(ctx, box, parent.FolderID, nil, &parent)
	if err != nil {
		t.Fatal(err)
	}
	after := parent
	after.Members = append([]domain.FilesetMember{}, parent.Members...)
	after.Members[0], after.Members[1] = after.Members[1], after.Members[0]
	reorder, err := l.SetFileset(ctx, box, parent.FolderID, &parent, &after)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = l.SetFileset(ctx, box, parent.FolderID, &parent, &parent); err == nil {
		t.Fatal("stale update succeeded")
	}
	sets, _ := l.Filesets(ctx)
	m := map[domain.FolderID]domain.Fileset{}
	for _, s := range sets {
		m[s.FolderID] = s
	}
	if got := domain.FilesetPrimary(parent.FolderID, m); got != byFile["c.jpg"] {
		t.Fatal(got)
	}
	if _, err = l.Undo(ctx, create); err == nil {
		t.Fatal("undo discarded subsequent order")
	}
	if _, err = l.Undo(ctx, reorder); err != nil {
		t.Fatal(err)
	}
	// A member deletion and its role removal are one undoable operation.
	preview, err := l.PreviewEntryDeletion(ctx, box, byFile["b.jpg"].ID, false)
	if err != nil {
		t.Fatal(err)
	}
	deletion, err := l.DeleteEntry(ctx, box, byFile["b.jpg"].ID, false, preview.Revision)
	if err != nil {
		t.Fatal(err)
	}
	state, _ := store.FilesetByID(l.db, parent.FolderID)
	if len(state.Members) != 2 {
		t.Fatal(state)
	}
	if _, err = l.Undo(ctx, deletion); err != nil {
		t.Fatal(err)
	}
	// Whole-subtree trash restores nested configurations and references.
	preview, err = l.PreviewEntryDeletion(ctx, box, parent.FolderID.ID, true)
	if err != nil {
		t.Fatal(err)
	}
	deletion, err = l.DeleteEntry(ctx, box, parent.FolderID.ID, true, preview.Revision)
	if err != nil {
		t.Fatal(err)
	}
	sets, _ = l.Filesets(ctx)
	if len(sets) != 0 {
		t.Fatal(sets)
	}
	if _, err = l.Undo(ctx, deletion); err != nil {
		t.Fatal(err)
	}
	before, err := store.Snapshot(l.db)
	if err != nil {
		t.Fatal(err)
	}
	if err = l.Rebuild(ctx); err != nil {
		t.Fatal(err)
	}
	replayed, err := store.Snapshot(l.db)
	if err != nil || before != replayed {
		t.Fatalf("rebuild drift: %v", err)
	}
	report, err := l.Verify(ctx, VerifyOptions{Deep: true})
	if err != nil || report.Count(integrity.ViewDrift) != 0 {
		t.Fatalf("verify: %+v %v", report, err)
	}
	state, _ = store.FilesetByID(l.db, parent.FolderID)
	if !reflect.DeepEqual(state.Members, parent.Members) {
		t.Fatal(state)
	}
	// Clearing a nested set still referenced by a page must fail before journaling.
	op := lastOpID(t, l)
	if _, err = l.SetFileset(ctx, box, child.FolderID, &child, nil); err == nil {
		t.Fatal("removed referenced set")
	}
	if lastOpID(t, l) != op {
		t.Fatal("invalid operation journaled")
	}
}
