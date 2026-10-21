package library

import (
	"database/sql"
	"fmt"

	"github.com/jinzhao1994/kist/internal/journal"
	"github.com/jinzhao1994/kist/internal/oplog"
	"github.com/jinzhao1994/kist/internal/store"
)

// applyOp writes one op into the database: the ops row, the materialized
// views, the index tables, and the updated_at propagation.
//
// Three callers share it and that is the point — the live write path, the
// catch-up after a crash, and Rebuild must all produce the same rows from the
// same line. When they were three copies, adding op_blobs updated two of them
// and the third stayed green (the bug verify --deep caught on 2026-09-02).
//
// It does not append to the journal. The write path first runs it in a transaction
// that is always rolled back to validate the event. Only after the line is
// durable may a caller commit its effects (invariant 2, spec 2.2.2). Keeping
// the append out of here also stops a replay from writing the log again.
func applyOp(tx *sql.Tx, row store.OpRow) (oplog.Targets, error) {
	return applyOpChecked(tx, row, row.SyncID != "")
}

func applyOpChecked(tx *sql.Tx, row store.OpRow, checkClaims bool) (oplog.Targets, error) {
	action := oplog.Action(row.Action)
	handler, err := oplog.Lookup(action)
	if err != nil {
		return oplog.Targets{}, fmt.Errorf("op %d: %w", row.ID, err)
	}
	targets, err := handler.Targets(row.Payload)
	if err != nil {
		return targets, fmt.Errorf("op %d: extract targets for %s: %w", row.ID, action, err)
	}
	if err := store.InsertOp(tx, row); err != nil {
		return targets, err
	}

	appliedAt := row.TS
	if row.SyncID != "" {
		appliedAt = row.AcceptedAt
	}
	vtx := store.NewViewTx(tx, appliedAt)

	// The folder chain has to be sampled on both sides of Apply.
	//
	// An import's files do not exist before Apply, and a delete's files do not
	// exist after it, so either sample alone misses one direction. Taking the
	// union also gets a future move-file op right for free: the first sample
	// names the old chain, the second the new one, and both need bumping.
	chainBefore, err := vtx.FolderChainOfFiles(targets.Files)
	if err != nil {
		return targets, err
	}
	var applyTx oplog.Tx = vtx
	if checkClaims {
		applyTx = &syncBlobClaims{Tx: vtx, claims: make(map[string]syncBlobClaim)}
	}
	if err := handler.Apply(applyTx, row.Payload); err != nil {
		return targets, fmt.Errorf("apply op %d (%s): %w", row.ID, action, err)
	}
	// Deferred references must be checked during the pre-append trial too.
	var violations int
	if err := tx.QueryRow(`SELECT count(*) FROM pragma_foreign_key_check WHERE "table" IN ('filesets','fileset_members')`).Scan(&violations); err != nil {
		return targets, err
	}
	if violations != 0 {
		return targets, fmt.Errorf("fileset still references a removed member: %w", oplog.ErrStaleBefore)
	}
	if err := store.InsertOpFilesets(tx, row.ID, targets.Filesets); err != nil {
		return targets, err
	}
	if err := store.InsertOpIndexes(tx, row.ID, targets.Boxes, targets.Files, targets.Blobs); err != nil {
		return targets, err
	}

	// Same rule for every caller — literally the same function, with this op's
	// own ts. That is why replayed updated_at values come out identical
	// instead of all being the moment of the rebuild (spec 1.5.1 ①).
	if err := touchAfter(vtx, action, oplog.Origin(row.Origin), targets, chainBefore); err != nil {
		return targets, err
	}
	return targets, nil
}

// rowFromLine converts a journal line into the shape store writes.
//
// The two structs are deliberately separate: journal may import nothing
// (spec 2.6) and store must not import journal, so library owns the
// translation — the same arrangement as store.OpRow and oplog's types.
func rowFromLine(l journal.Line) (store.OpRow, error) {
	ts, err := l.Time()
	if err != nil {
		return store.OpRow{}, err
	}
	row := store.OpRow{
		ID:        l.ID,
		TS:        ts,
		Action:    l.Action,
		Origin:    l.Origin,
		Payload:   l.Payload,
		RevertsID: l.RevertsID,
		RootOpID:  l.RootOpID,
	}
	if l.Sync != nil {
		row.SyncID, row.SyncDigest = l.Sync.ID, l.Sync.Digest
		row.AcceptedAt, err = journal.ParseTime(l.Sync.AcceptedAt)
		if err != nil {
			return store.OpRow{}, err
		}
	}
	if l.Request != nil {
		row.RequestID = l.Request.ID
		row.RequestDigest = l.Request.Digest
	}
	return row, nil
}

// lineFromRow is the other direction.
func lineFromRow(r store.OpRow) journal.Line {
	line := journal.Line{
		ID:        r.ID,
		TS:        journal.FormatTime(r.TS),
		Action:    r.Action,
		Origin:    r.Origin,
		RevertsID: r.RevertsID,
		RootOpID:  r.RootOpID,
		Payload:   r.Payload,
	}
	if r.SyncID != "" {
		line.Sync = &journal.Sync{ID: r.SyncID, Digest: r.SyncDigest, AcceptedAt: journal.FormatTime(r.AcceptedAt)}
	}
	if r.RequestID != "" {
		line.Request = &journal.Request{ID: r.RequestID, Digest: r.RequestDigest}
	}
	return line
}
