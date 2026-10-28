package library

import (
	"errors"
	"slices"
	"testing"

	"github.com/jinzhao1994/kist/internal/domain"
	"github.com/jinzhao1994/kist/internal/extract"
	"github.com/jinzhao1994/kist/internal/oplog"
	"github.com/jinzhao1994/kist/internal/store"
)

func TestOrientationReplacementClearsAngleAndUndoRestoresVersion(t *testing.T) {
	for _, manual := range []bool{false, true} {
		t.Run(map[bool]string{false: "automatic", true: "manual"}[manual], func(t *testing.T) {
			l := openTestLibrary(t)
			ctx := t.Context()
			id := rotationFile(t, l)
			file, _, _ := l.File(ctx, id)
			box, _, err := l.FileLocation(ctx, file)
			if err != nil {
				t.Fatal(err)
			}
			p := &uprightProvider{fakeOCR: fakeOCR{text: "synthetic"}}
			pending, err := l.PrepareOCR(ctx, id, p, "test")
			if err != nil {
				t.Fatal(err)
			}
			if manual {
				angle := 90
				if err := l.SetImageRotation(ctx, id, file.BlobKey, &angle); err != nil {
					t.Fatal(err)
				}
			}
			oldHead, _, _ := l.ops.Last()
			client := newSyncClient(t, l)
			replaced, err := l.Import(ctx, writeTree(t, map[string]string{file.Name: "different synthetic content"}), ImportOptions{Box: box, ConflictMode: "overwrite"})
			if err != nil {
				t.Fatal(err)
			}
			current, _, _ := l.File(ctx, id)
			info, err := l.MediaInfo(ctx, id)
			if err != nil || info.Rotation != 0 || info.RotationManual || info.RotationBlob != current.BlobKey {
				t.Fatal("replacement reused old rotation", info, err)
			}
			if _, exists := metaOf(t, l, id)["orientation:clockwise_degrees"]; exists {
				t.Fatal("replacement kept the old angle")
			}
			candidates, err := store.ImageTextCandidates(l.db, "orientation", -1, replaced.OpID, domain.FileID{}, 100)
			if err != nil || !slices.Contains(candidates, id) {
				t.Fatal("replacement is not eligible for orientation backfill", candidates, err)
			}
			if _, err := l.ExecuteRequest(ctx, requestFor("replaced-ocr-stale", "ocr"), pending.Commit); !errors.Is(err, ErrOCRChanged) {
				t.Fatal("inflight OCR accepted replacement", err)
			}
			if _, err := client.Synchronize(ctx, &testSyncPeer{nas: l}); err != nil {
				t.Fatal(err)
			}
			remoteInfo, err := client.MediaInfo(ctx, id)
			if err != nil || remoteInfo.Rotation != 0 || remoteInfo.RotationBlob != current.BlobKey {
				t.Fatal("client retained old orientation", remoteInfo, err)
			}
			if err := l.ViewAt(ctx, oldHead.ID, func(v *HistoricalView) error {
				info, err := v.MediaInfo(ctx, id)
				if err != nil || info.Rotation != 90 || info.RotationManual != manual || info.RotationBlob != file.BlobKey {
					t.Fatal("historical rotation lost", info, err)
				}
				return nil
			}); err != nil {
				t.Fatal(err)
			}
			checkRebuilt(t, l)
			if _, err := l.Undo(ctx, replaced.OpID); err != nil {
				t.Fatal(err)
			}
			info, err = l.MediaInfo(ctx, id)
			if err != nil || info.Rotation != 90 || info.RotationManual != manual || info.RotationBlob != file.BlobKey {
				t.Fatal("undo did not restore content and orientation together", info, err)
			}
			if _, err := client.Synchronize(ctx, &testSyncPeer{nas: l}); err != nil {
				t.Fatal(err)
			}
			remoteInfo, err = client.MediaInfo(ctx, id)
			if err != nil || remoteInfo.Rotation != 90 || remoteInfo.RotationManual != manual || remoteInfo.RotationBlob != file.BlobKey {
				t.Fatal("client did not restore rotation after undo", remoteInfo, err)
			}
			checkRebuilt(t, l)
		})
	}
}

func TestOrientationFirstWriteRejectsReplacedContent(t *testing.T) {
	l := openTestLibrary(t)
	ctx := t.Context()
	id := rotationFile(t, l)
	f, _, _ := l.File(ctx, id)
	box, _, _ := l.FileLocation(ctx, f)
	// A first manual rotation has no previous angle. Slot checks alone cannot
	// detect a replacement between preparation and append.
	raw, err := oplog.Marshal(oplog.SetMetadataPayload{Files: []oplog.FileMetadataChange{{
		FileID: id, BlobKey: f.BlobKey,
		Keys: []oplog.KeyChange{{Key: "orientation:clockwise_degrees", After: &oplog.MetadataState{Value: "90", Type: extract.TypeInt, Source: extract.UserSource}}},
	}}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := l.Import(ctx, writeTree(t, map[string]string{f.Name: "replacement"}), ImportOptions{Box: box, ConflictMode: "overwrite"}); err != nil {
		t.Fatal(err)
	}
	head, _, _ := l.ops.Last()
	if _, err := l.write(ctx, oplog.ActionSetMetadata, raw, oplog.OriginUser, writeOptions{}); !errors.Is(err, oplog.ErrStaleBefore) {
		t.Fatal("stale first rotation accepted", err)
	}
	after, _, _ := l.ops.Last()
	if after.ID != head.ID {
		t.Fatal("rejected rotation appended an operation")
	}
}

func TestOrientationGenericEditAndInverseCarryContentGuard(t *testing.T) {
	l := openTestLibrary(t)
	id := rotationFile(t, l)
	f, _, _ := l.File(t.Context(), id)
	angle := 90
	if err := l.SetImageRotation(t.Context(), id, f.BlobKey, &angle); err != nil {
		t.Fatal(err)
	}
	var change *oplog.FileMetadataChange
	err := l.readTx(t.Context(), func(tx *store.ViewTx) error {
		var err error
		change, err = editMetadataChange(tx, id, MetadataEdit{BlobKey: f.BlobKey, Key: "orientation:clockwise_degrees", BeforeValue: "90", BeforeType: "int", BeforeSource: "user", Value: "180"}, l.zone())
		return err
	})
	if err != nil || change == nil || change.BlobKey != f.BlobKey {
		t.Fatal("generic edit lost content guard", change, err)
	}
	raw, _ := oplog.Marshal(oplog.SetMetadataPayload{Files: []oplog.FileMetadataChange{*change}})
	h, _ := oplog.Lookup(oplog.ActionSetMetadata)
	_, inverse, err := h.Inverse(raw)
	if err != nil {
		t.Fatal(err)
	}
	payload, err := oplog.Payload[oplog.SetMetadataPayload](inverse)
	if err != nil || payload.Files[0].BlobKey != f.BlobKey {
		t.Fatal("undo lost content guard", payload, err)
	}
	if err := l.readTx(t.Context(), func(tx *store.ViewTx) error {
		removed, err := removeMetadataChange(tx, id, "orientation:clockwise_degrees", l.zone())
		if err != nil || removed == nil || removed.BlobKey != f.BlobKey {
			t.Fatal("removal lost content guard", removed, err)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}
