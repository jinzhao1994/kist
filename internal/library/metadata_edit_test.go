package library

import (
	"errors"
	"strings"
	"testing"

	"github.com/jinzhao1994/kist/internal/extract"
	"github.com/jinzhao1994/kist/internal/oplog"
)

func TestMetadataEditProtectsAndReplays(t *testing.T) {
	l, box := openWithBox(t)
	f := importOneFile(t, l, box, "test.txt", "synthetic")
	e := &fakeExtractor{id: "fake", version: 1, ns: []string{"ocr", "exif"}, tags: []extract.KV{text("ocr:text", "old"), text("exif:DateTimeOriginal", "2020:01:02 12:00:00")}}
	extractWith(t, l, e)
	edit := MetadataEdit{BlobKey: f.BlobKey, Key: "ocr:text", BeforeValue: "old", BeforeSource: "fake", BeforeType: "text", Value: strings.Repeat("line\n", 500)}
	if err := l.EditMetadata(t.Context(), f.ID, edit); err != nil {
		t.Fatal(err)
	}
	row := metaOf(t, l, f.ID)[edit.Key]
	if row.Source != "user" || row.Value != edit.Value {
		t.Fatal(row)
	}
	if err := l.EditMetadata(t.Context(), f.ID, edit); !errors.Is(err, oplog.ErrStaleBefore) {
		t.Fatal(err)
	}
	e.version++
	extractWith(t, l, e)
	if metaOf(t, l, f.ID)[edit.Key].Value != edit.Value {
		t.Fatal("extraction replaced human text")
	}
	old, _, _ := l.File(t.Context(), f.ID)
	date := MetadataEdit{BlobKey: f.BlobKey, Key: "exif:DateTimeOriginal", BeforeValue: "2020:01:02 12:00:00", BeforeSource: "fake", BeforeType: "text", Value: "2021:02:03 13:00:00"}
	if err := l.EditMetadata(t.Context(), f.ID, date); err != nil {
		t.Fatal(err)
	}
	current, _, _ := l.File(t.Context(), f.ID)
	if current.CapturedAt == old.CapturedAt {
		t.Fatal("capture time did not update")
	}
	head, _, _ := l.ops.Last()
	before := snapshotOf(t, l)
	if err := l.Rebuild(t.Context()); err != nil {
		t.Fatal(err)
	}
	if snapshotOf(t, l) != before {
		t.Fatal("replay drift")
	}
	if _, err := l.Undo(t.Context(), head.ID); err != nil {
		t.Fatal(err)
	}
	row = metaOf(t, l, f.ID)[date.Key]
	if row.Source != "fake" || row.Value != date.BeforeValue {
		t.Fatal("undo lost provenance", row)
	}
	report, err := l.Verify(t.Context(), VerifyOptions{Deep: true})
	if err != nil || report.Total() != 0 {
		t.Fatal(report, err)
	}
}

func TestMetadataEditValidatesSnapshotAndType(t *testing.T) {
	l, box := openWithBox(t)
	f := importOneFile(t, l, box, "test.txt", "synthetic")
	row := metaOf(t, l, f.ID)["file:size"]
	base := MetadataEdit{BlobKey: f.BlobKey, Key: row.Key, BeforeValue: row.Value, BeforeSource: row.Source, BeforeType: string(row.Type), Value: "100"}
	for _, mutate := range []func(*MetadataEdit){
		func(e *MetadataEdit) { e.BlobKey = "changed" },
		func(e *MetadataEdit) { e.BeforeSource = "user" },
		func(e *MetadataEdit) { e.BeforeType = "text" },
	} {
		e := base
		mutate(&e)
		if err := l.EditMetadata(t.Context(), f.ID, e); !errors.Is(err, oplog.ErrStaleBefore) {
			t.Fatal(err)
		}
	}
	base.Value = "not a number"
	if err := l.EditMetadata(t.Context(), f.ID, base); !errors.Is(err, ErrInvalidRequest) {
		t.Fatal(err)
	}
	base.Value = row.Value
	head, _, _ := l.ops.Last()
	if err := l.EditMetadata(t.Context(), f.ID, base); err != nil {
		t.Fatal(err)
	}
	after, _, _ := l.ops.Last()
	if head.ID != after.ID {
		t.Fatal("unchanged edit wrote an op")
	}
}

func TestClientMetadataEditSync(t *testing.T) {
	nas, box := openWithBox(t)
	f := importOneFile(t, nas, box, "test.txt", "synthetic")
	row := metaOf(t, nas, f.ID)["file:mtime"]
	c := newSyncClient(t, nas)
	edit := MetadataEdit{BlobKey: f.BlobKey, Key: row.Key, BeforeValue: row.Value, BeforeSource: row.Source, BeforeType: string(row.Type), Value: "1600000000000000000"}
	if err := c.EditMetadata(t.Context(), f.ID, edit); err != nil {
		t.Fatal(err)
	}
	result, err := c.Synchronize(t.Context(), &testSyncPeer{nas: nas})
	if err != nil || result.Accepted != 1 {
		t.Fatal(result, err)
	}
	got := metaOf(t, nas, f.ID)[row.Key]
	if got.Value != edit.Value || got.Source != "user" {
		t.Fatal(got)
	}
	edit.BeforeValue, edit.BeforeSource, edit.Remove = got.Value, got.Source, true
	if err := c.EditMetadata(t.Context(), f.ID, edit); err != nil {
		t.Fatal(err)
	}
	result, err = c.Synchronize(t.Context(), &testSyncPeer{nas: nas})
	if err != nil || result.Accepted != 1 {
		t.Fatal(result, err)
	}
	if got := metaOf(t, nas, f.ID)[row.Key]; !got.Tombstone || got.Source != "user" {
		t.Fatal(got)
	}
}

func TestMetadataRemovalSnapshotAndUndo(t *testing.T) {
	l, box := openWithBox(t)
	f := importOneFile(t, l, box, "test.txt", "synthetic")
	e := &fakeExtractor{id: "fake", version: 1, ns: []string{"exif"}, tags: []extract.KV{text("exif:DateTimeOriginal", "2020:01:02 12:00:00")}}
	extractWith(t, l, e)
	row := metaOf(t, l, f.ID)["exif:DateTimeOriginal"]
	edit := MetadataEdit{BlobKey: f.BlobKey, Key: row.Key, BeforeValue: row.Value, BeforeSource: row.Source, BeforeType: string(row.Type), Remove: true}
	for _, mutate := range []func(*MetadataEdit){
		func(e *MetadataEdit) { e.BlobKey = "changed" },
		func(e *MetadataEdit) { e.BeforeValue = "changed" },
		func(e *MetadataEdit) { e.BeforeSource = "user" },
		func(e *MetadataEdit) { e.BeforeType = "int" },
	} {
		stale := edit
		mutate(&stale)
		if err := l.EditMetadata(t.Context(), f.ID, stale); !errors.Is(err, oplog.ErrStaleBefore) {
			t.Fatal(err)
		}
	}
	before, _, _ := l.File(t.Context(), f.ID)
	if err := l.EditMetadata(t.Context(), f.ID, edit); err != nil {
		t.Fatal(err)
	}
	head, _, _ := l.ops.Last()
	after, _, _ := l.File(t.Context(), f.ID)
	if before.CapturedAt == after.CapturedAt {
		t.Fatal("deletion did not recalculate capture time")
	}
	e.version++
	extractWith(t, l, e)
	if got := metaOf(t, l, f.ID)[row.Key]; !got.Tombstone || got.Source != "user" {
		t.Fatal("extraction restored deleted value", got)
	}
	snapshot := snapshotOf(t, l)
	if err := l.Rebuild(t.Context()); err != nil {
		t.Fatal(err)
	}
	if snapshotOf(t, l) != snapshot {
		t.Fatal("removal replay drift")
	}
	if _, err := l.Undo(t.Context(), head.ID); err != nil {
		t.Fatal(err)
	}
	if got := metaOf(t, l, f.ID)[row.Key]; got != row {
		t.Fatal("undo did not restore value and source", got)
	}
	report, err := l.Verify(t.Context(), VerifyOptions{Deep: true})
	if err != nil || report.Total() != 0 {
		t.Fatal(report, err)
	}
}
