package library

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jinzhao1994/kist/internal/domain"
	"github.com/jinzhao1994/kist/internal/oplog"
)

func syncBox(t *testing.T, id string) SyncProposal {
	t.Helper()
	raw, err := oplog.Marshal(oplog.CreateBoxPayload{BoxID: domain.NewBoxID(), RootFolderID: domain.NewFolderID(), BoxFields: oplog.BoxFields{Name: id}})
	if err != nil {
		t.Fatal(err)
	}
	return SyncProposal{Version: 1, ID: id, OccurredAt: time.Date(2026, 10, 20, 1, 2, 3, 0, time.UTC), Action: string(oplog.ActionCreateBox), Origin: "user", Payload: raw}
}

func TestSyncActionCoverage(t *testing.T) {
	actual := SyncActions()
	expected := []string{}
	for _, a := range oplog.AllActions() {
		if a != oplog.ActionRequestReceipt {
			expected = append(expected, string(a))
		}
	}
	slices.Sort(expected)
	if !slices.Equal(actual, expected) {
		t.Fatalf("sync=%v registered=%v", actual, expected)
	}
}

func TestSyncAcceptanceRetryUndoAndRestart(t *testing.T) {
	l := openTestLibrary(t)
	p := syncBox(t, "pending-create-box")
	first, err := l.AcceptSync(t.Context(), p)
	if err != nil {
		t.Fatal(err)
	}
	undo := SyncProposal{Version: 1, ID: "pending-undo-box", OccurredAt: p.OccurredAt.Add(time.Minute), Origin: "user", Undo: &SyncReference{LocalID: p.ID}, Requires: []string{p.ID}}
	undone, err := l.AcceptSync(t.Context(), undo)
	if err != nil || undone.OpID != first.OpID+1 {
		t.Fatalf("undo=%v err=%v", undone, err)
	}
	retried, err := l.AcceptSync(t.Context(), p)
	if err != nil || retried != first {
		t.Fatalf("retry=%v err=%v", retried, err)
	}
	root := l.Layout().Root
	l.Close()
	l, err = Open(root)
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	if err := l.Rebuild(t.Context()); err != nil {
		t.Fatal(err)
	}
	retried, err = l.AcceptSync(t.Context(), undo)
	if err != nil || retried != undone {
		t.Fatalf("undo retry=%v err=%v", retried, err)
	}
	changed := p
	changed.OccurredAt = changed.OccurredAt.Add(time.Second)
	if _, err := l.AcceptSync(t.Context(), changed); !errors.Is(err, ErrRequestConflict) {
		t.Fatalf("identity reused: %v", err)
	}
	status, found, err := l.SyncStatus(t.Context(), p.ID)
	if err != nil || !found || status != first {
		t.Fatalf("status=%v found=%v err=%v", status, found, err)
	}
	if count, _ := l.OpCount(t.Context()); count != 2 {
		t.Fatalf("count=%d", count)
	}
}

func TestSyncAcceptsOnceAcrossConnections(t *testing.T) {
	l := openTestLibrary(t)
	other, err := Open(l.Layout().Root)
	if err != nil {
		t.Fatal(err)
	}
	defer other.Close()
	p := syncBox(t, "concurrent-pending")
	var wg sync.WaitGroup
	for i := range 8 {
		wg.Go(func() {
			db := l
			if i%2 == 1 {
				db = other
			}
			r, err := db.AcceptSync(t.Context(), p)
			if err != nil || r.OpID != 1 {
				t.Errorf("accept=%v err=%v", r, err)
			}
		})
	}
	wg.Wait()
	if count, _ := l.OpCount(t.Context()); count != 1 {
		t.Fatalf("count=%d", count)
	}
}

func TestSyncLostAcknowledgementRecovers(t *testing.T) {
	l := openTestLibrary(t)
	p := syncBox(t, "lost-acknowledgement")
	l.hooks.afterAppend = func() error { return errors.New("power cut") }
	r, err := l.AcceptSync(t.Context(), p)
	var outcome *WriteOutcomeError
	if !errors.As(err, &outcome) || r.OpID != 1 {
		t.Fatalf("write=%v err=%v", r, err)
	}
	l.hooks.afterAppend = nil
	r, err = l.AcceptSync(t.Context(), p)
	if err != nil || r.OpID != 1 || r.AcceptedAt.IsZero() {
		t.Fatalf("recovered=%v err=%v", r, err)
	}
}

func TestSyncDependenciesAndStalePatch(t *testing.T) {
	l := openTestLibrary(t)
	p := syncBox(t, "pending-dependency")
	p.Requires = []string{"not-yet-accepted"}
	if _, err := l.AcceptSync(t.Context(), p); !errors.Is(err, ErrSyncDependency) {
		t.Fatalf("dependency=%v", err)
	}
	p.Requires = nil
	if _, err := l.AcceptSync(t.Context(), p); err != nil {
		t.Fatal(err)
	}
	created, err := oplog.Payload[oplog.CreateBoxPayload](p.Payload)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := l.UpdateBox(t.Context(), created.BoxID, oplog.BoxFields{Name: "NAS changed"}); err != nil {
		t.Fatal(err)
	}
	// Deleting the original box cannot erase a concurrent rename.
	p.ID = "stale-box-delete"
	p.Action = string(oplog.ActionDeleteBox)
	if _, err := l.AcceptSync(t.Context(), p); !errors.Is(err, ErrSyncConflict) {
		t.Fatalf("stale=%v", err)
	}
	if _, found, err := l.SyncStatus(t.Context(), p.ID); err != nil || found {
		t.Fatalf("rejected operation has receipt: %v %v", found, err)
	}
}

func TestSyncRequiresVerifiedOriginalBeforeCommit(t *testing.T) {
	l := openTestLibrary(t)
	b, err := l.CreateBox(t.Context(), "NAS", "", domain.LocalDate{}, domain.LocalDate{})
	if err != nil {
		t.Fatal(err)
	}
	content := "offline photo"
	sum := sha256.Sum256([]byte(content))
	digest := hex.EncodeToString(sum[:])
	f := oplog.ImportedFile{FileID: domain.NewFileID(), FolderID: b.RootFolderID, Name: "photo.jpg", BlobKey: "sha256:" + digest, Sha256: digest, Size: int64(len(content))}
	raw, _ := oplog.Marshal(oplog.ImportFilesPayload{BoxID: b.ID, Files: []oplog.ImportedFile{f}})
	p := SyncProposal{Version: 1, ID: "pending-new-photo", OccurredAt: time.Now().UTC(), Action: string(oplog.ActionImportFiles), Origin: "user", Payload: raw}
	if _, err := l.AcceptSync(t.Context(), p); err == nil {
		t.Fatal("accepted missing original")
	}
	if n, _ := l.OpCount(t.Context()); n != 1 {
		t.Fatalf("missing blob wrote op: %d", n)
	}
	if _, err := l.blobs.Put(t.Context(), strings.NewReader(content)); err != nil {
		t.Fatal(err)
	}
	f.Size++
	bad, _ := oplog.Marshal(oplog.ImportFilesPayload{BoxID: b.ID, Files: []oplog.ImportedFile{f}})
	p.Payload = bad
	if _, err := l.AcceptSync(t.Context(), p); !errors.Is(err, ErrSyncConflict) {
		t.Fatalf("wrong size=%v", err)
	}
	// A later correct upsert must not hide an earlier incorrect declaration.
	second := f
	second.FileID = domain.NewFileID()
	second.Name = "another.jpg"
	second.Size--
	p.Payload, _ = oplog.Marshal(oplog.ImportFilesPayload{BoxID: b.ID, Files: []oplog.ImportedFile{f, second}})
	if _, err := l.AcceptSync(t.Context(), p); !errors.Is(err, ErrSyncConflict) {
		t.Fatalf("inconsistent duplicate blob declarations=%v", err)
	}
	if n, _ := l.OpCount(t.Context()); n != 1 {
		t.Fatalf("invalid declarations wrote op: %d", n)
	}
	p.Payload = raw
	r, err := l.AcceptSync(t.Context(), p)
	if err != nil || r.OpID != 2 {
		t.Fatalf("valid import=%v err=%v", r, err)
	}
	before := snapshotOf(t, l)
	if err := l.Rebuild(t.Context()); err != nil {
		t.Fatal(err)
	}
	if before != snapshotOf(t, l) {
		t.Fatal("import rebuild differs")
	}
	metadata, _ := oplog.Marshal(oplog.ExtractMetadataPayload{Extractor: "media", Version: 1, Files: []oplog.FileExtraction{{FileID: f.FileID}}})
	extraction := SyncProposal{Version: 1, ID: "pending-extraction", OccurredAt: p.OccurredAt, Action: string(oplog.ActionExtractMetadata), Origin: "extractor", Payload: metadata}
	if _, err := l.AcceptSync(t.Context(), extraction); !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("extraction omitted content precondition: %v", err)
	}
	extraction.Files = []SyncFileState{{ID: f.FileID, BlobKey: "sha256:" + strings.Repeat("0", 64)}}
	if _, err := l.AcceptSync(t.Context(), extraction); !errors.Is(err, ErrSyncConflict) {
		t.Fatalf("extraction used another original: %v", err)
	}
	extraction.Files[0].BlobKey = f.BlobKey
	if _, err := l.AcceptSync(t.Context(), extraction); err != nil {
		t.Fatal(err)
	}
}

func TestSyncGroupingAndInvalidReferences(t *testing.T) {
	l := openTestLibrary(t)
	first := syncBox(t, "group-root-local")
	root, err := l.AcceptSync(t.Context(), first)
	if err != nil {
		t.Fatal(err)
	}
	member := syncBox(t, "group-member-local")
	member.Group = &SyncReference{LocalID: first.ID}
	member.Requires = []string{first.ID}
	r, err := l.AcceptSync(t.Context(), member)
	if err != nil {
		t.Fatal(err)
	}
	entry, err := l.HistoryOp(t.Context(), r.OpID)
	if err != nil || entry.RootOpID == nil || *entry.RootOpID != root.OpID {
		t.Fatalf("group=%+v err=%v", entry, err)
	}
	for _, ref := range []SyncReference{{}, {OpID: -1}, {OpID: root.OpID, LocalID: first.ID}, {LocalID: member.ID}} {
		p := syncBox(t, "invalid-group-local")
		p.Group = &ref
		if _, err := l.AcceptSync(t.Context(), p); !errors.Is(err, ErrInvalidRequest) {
			t.Fatalf("ref=%+v err=%v", ref, err)
		}
	}
}

func TestSyncRestoreRejectsConflictingBlobClaims(t *testing.T) {
	l := openTestLibrary(t)
	box, err := l.CreateBox(t.Context(), "restore target", "", domain.LocalDate{}, domain.LocalDate{})
	if err != nil {
		t.Fatal(err)
	}
	key := "sha256:" + strings.Repeat("a", 64)
	files := []domain.ArchivedFile{
		{File: domain.File{ID: domain.NewFileID(), BlobKey: key}, SHA256: strings.Repeat("a", 64), Size: 1},
		{File: domain.File{ID: domain.NewFileID(), BlobKey: key}, SHA256: strings.Repeat("a", 64), Size: 2},
	}
	for _, tc := range []struct {
		action oplog.Action
		value  any
	}{
		{oplog.ActionRestoreBoxContents, domain.BoxContents{Files: files}},
		{oplog.ActionRestoreEntryContents, domain.EntryContents{BoxID: box.ID, Parent: box.RootFolderID, Ancestors: []domain.FolderID{box.RootFolderID}, Files: files}},
	} {
		raw, err := oplog.Marshal(tc.value)
		if err != nil {
			t.Fatal(err)
		}
		p := SyncProposal{Version: 1, ID: "conflicting-restore", OccurredAt: time.Now().UTC(), Action: string(tc.action), Origin: "user", Payload: raw}
		if _, err := l.AcceptSync(t.Context(), p); !errors.Is(err, ErrSyncConflict) || !strings.Contains(err.Error(), "inconsistent declarations") {
			t.Fatalf("%s: %v", tc.action, err)
		}
	}
	if n, _ := l.OpCount(t.Context()); n != 1 {
		t.Fatalf("invalid restore wrote %d operations", n)
	}
}

func TestSyncProtocolValidationDoesNotAppend(t *testing.T) {
	l := openTestLibrary(t)
	base := syncBox(t, "invalid-proposal")
	cases := []func(*SyncProposal){
		func(p *SyncProposal) { p.Version = 2 },
		func(p *SyncProposal) { p.OccurredAt = time.Time{} },
		func(p *SyncProposal) { p.OccurredAt = time.Date(3000, 1, 1, 0, 0, 0, 0, time.UTC) },
		func(p *SyncProposal) { p.Origin = "sync" },
		func(p *SyncProposal) { p.Origin = "extractor" },
		func(p *SyncProposal) { p.Action = "future_action" },
		func(p *SyncProposal) { p.Action = "request_receipt" },
		func(p *SyncProposal) { p.ID = "short" },
		func(p *SyncProposal) { p.Undo = &SyncReference{OpID: 1} },
	}
	for i, change := range cases {
		p := base
		change(&p)
		if _, err := l.AcceptSync(t.Context(), p); err == nil {
			t.Errorf("case %d accepted", i)
		}
	}
	if n, _ := l.OpCount(t.Context()); n != 0 {
		t.Fatalf("invalid proposals appended %d operations", n)
	}
}
