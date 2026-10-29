package library

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"os"
	"slices"
	"time"

	"github.com/google/uuid"
	"github.com/jinzhao1994/kist/internal/blob"
	"github.com/jinzhao1994/kist/internal/domain"
	"github.com/jinzhao1994/kist/internal/runtimestore"
)

func (q *OCRQueue) database() (*runtimestore.Store, error) {
	db, err := q.library.runtimeStore()
	if err != nil {
		return nil, err
	}
	err = db.Import(context.Background(), "queue/"+q.kind, func(tx *sql.Tx) error {
		f, err := os.Open(q.path())
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		if err != nil {
			return err
		}
		defer f.Close()
		var s ocrQueueState
		if err = json.NewDecoder(io.LimitReader(f, 32<<20)).Decode(&s); err != nil {
			return err
		}
		return q.saveTx(tx, s)
	})
	return db, err
}

func (q *OCRQueue) load() (ocrQueueState, error) {
	var s ocrQueueState
	db, err := q.database()
	if err != nil {
		return s, err
	}
	var aa, at, ar, ba, bt, br sql.NullInt64
	var af, bf sql.NullString
	err = db.DB.QueryRow(`SELECT paused,error,auto_cursor,auto_after,auto_through,auto_file,auto_retry,
	 backfill_after,backfill_through,backfill_file,backfill_retry FROM queue_controls WHERE kind=?`, q.kind).
		Scan(&s.Paused, &s.Error, &s.AutoCursor, &aa, &at, &af, &ar, &ba, &bt, &bf, &br)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return s, err
	}
	decode := func(a, t, r sql.NullInt64, f sql.NullString) (*ocrScan, error) {
		if !a.Valid {
			return nil, nil
		}
		id, err := domain.ParseID(f.String)
		if err != nil {
			return nil, err
		}
		return &ocrScan{After: a.Int64, Through: t.Int64, Cursor: domain.FileID{ID: id}, Retry: r.Int64 != 0}, nil
	}
	if s.AutoScan, err = decode(aa, at, ar, af); err != nil {
		return s, err
	}
	if s.Backfill, err = decode(ba, bt, br, bf); err != nil {
		return s, err
	}
	rows, err := db.DB.Query(`SELECT file_id,blob_key,name,state,error,request_id,automatic,manual,force FROM background_jobs WHERE kind=? ORDER BY position,job_key`, q.kind)
	if err != nil {
		return s, err
	}
	defer rows.Close()
	for rows.Next() {
		j, err := scanOCRJob(rows)
		if err != nil {
			return s, err
		}
		s.Jobs = append(s.Jobs, j)
	}
	return s, rows.Err()
}

type rowScanner interface{ Scan(...any) error }

func scanOCRJob(row rowScanner) (OCRJob, error) {
	var j OCRJob
	var id string
	err := row.Scan(&id, &j.BlobKey, &j.Name, &j.State, &j.Error, &j.RequestID, &j.Automatic, &j.Manual, &j.Force)
	if err != nil {
		return j, err
	}
	parsed, parseErr := domain.ParseID(id)
	j.FileID, err = domain.FileID{ID: parsed}, parseErr
	return j, err
}

func (q *OCRQueue) save(s ocrQueueState) error {
	db, err := q.database()
	if err != nil {
		return err
	}
	return db.Transaction(context.Background(), func(tx *sql.Tx) error { return q.saveTx(tx, s) })
}

func (q *OCRQueue) saveTx(tx *sql.Tx, s ocrQueueState) error {
	args := []any{q.kind, s.Paused, s.Error, s.AutoCursor}
	for _, scan := range []*ocrScan{s.AutoScan, s.Backfill} {
		if scan == nil {
			args = append(args, nil, nil, nil, nil)
		} else {
			args = append(args, scan.After, scan.Through, scan.Cursor.String(), scan.Retry)
		}
	}
	_, err := tx.Exec(`INSERT INTO queue_controls VALUES(?,?,?,?,?,?,?,?,?,?,?,?) ON CONFLICT(kind) DO UPDATE SET
	 paused=excluded.paused,error=excluded.error,auto_cursor=excluded.auto_cursor,auto_after=excluded.auto_after,
	 auto_through=excluded.auto_through,auto_file=excluded.auto_file,auto_retry=excluded.auto_retry,
	 backfill_after=excluded.backfill_after,backfill_through=excluded.backfill_through,backfill_file=excluded.backfill_file,backfill_retry=excluded.backfill_retry WHERE (queue_controls.paused,queue_controls.error,queue_controls.auto_cursor,queue_controls.auto_after,queue_controls.auto_through,queue_controls.auto_file,queue_controls.auto_retry,queue_controls.backfill_after,queue_controls.backfill_through,queue_controls.backfill_file,queue_controls.backfill_retry) IS NOT (excluded.paused,excluded.error,excluded.auto_cursor,excluded.auto_after,excluded.auto_through,excluded.auto_file,excluded.auto_retry,excluded.backfill_after,excluded.backfill_through,excluded.backfill_file,excluded.backfill_retry)`, args...)
	if err != nil {
		return err
	}
	rows, err := tx.Query(`SELECT job_key,request_id,state,position FROM background_jobs WHERE kind=?`, q.kind)
	if err != nil {
		return err
	}
	type previous struct {
		key, request, state string
		position            int64
	}
	var old []previous
	for rows.Next() {
		var p previous
		if err = rows.Scan(&p.key, &p.request, &p.state, &p.position); err != nil {
			rows.Close()
			return err
		}
		old = append(old, p)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	kept := map[string]bool{}
	prior := map[string]previous{}
	var nextPosition int64
	for _, p := range old {
		prior[p.key] = p
		if p.position >= nextPosition {
			nextPosition = p.position + 1
		}
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	for _, j := range s.Jobs {
		if j.FileID.IsZero() {
			return ErrInvalidRequest
		}
		if _, err := blob.ParseKey(j.BlobKey); err != nil {
			return err
		}
		if _, err := uuid.Parse(j.RequestID); err != nil {
			return ErrInvalidRequest
		}
		if !slices.Contains([]string{"queued", "running", "failed", "ready"}, j.State) {
			return ErrInvalidRequest
		}
		key := j.FileID.String() + "/" + j.BlobKey
		var conflict int
		if err := tx.QueryRow(`SELECT count(*) FROM job_attempts WHERE request_id=? AND (kind<>? OR job_key<>?)`, j.RequestID, q.kind, key).Scan(&conflict); err != nil {
			return err
		}
		if conflict != 0 {
			return ErrRequestConflict
		}

		if kept[key] {
			return errors.New("duplicate persisted extraction job")
		}
		kept[key] = true
		position := nextPosition
		if previous, ok := prior[key]; ok && previous.request == j.RequestID {
			position = previous.position
		} else {
			nextPosition++
		}
		_, err = tx.Exec(`INSERT INTO background_jobs(kind,job_key,file_id,blob_key,name,state,error,request_id,automatic,manual,force,position)
		 VALUES(?,?,?,?,?,?,?,?,?,?,?,?) ON CONFLICT(kind,job_key) DO UPDATE SET name=excluded.name,state=excluded.state,
		 error=excluded.error,request_id=excluded.request_id,automatic=excluded.automatic,manual=excluded.manual,force=excluded.force,position=excluded.position WHERE (background_jobs.name,background_jobs.state,background_jobs.error,background_jobs.request_id,background_jobs.automatic,background_jobs.manual,background_jobs.force,background_jobs.position) IS NOT (excluded.name,excluded.state,excluded.error,excluded.request_id,excluded.automatic,excluded.manual,excluded.force,excluded.position)`,
			q.kind, key, j.FileID.String(), j.BlobKey, j.Name, j.State, j.Error, j.RequestID, j.Automatic, j.Manual, j.Force, position)
		if err != nil {
			return err
		}
		started, finished := "", ""
		if j.State == "running" {
			started = now
		}
		if j.State == "failed" {
			finished = now
		}
		_, err = tx.Exec(`INSERT INTO job_attempts(request_id,kind,job_key,file_id,blob_key,state,started_at,finished_at,error)
		 VALUES(?,?,?,?,?,?,?,?,?) ON CONFLICT(request_id) DO UPDATE SET state=excluded.state,error=excluded.error,
		 started_at=CASE WHEN job_attempts.started_at='' THEN excluded.started_at ELSE job_attempts.started_at END,
		 finished_at=CASE WHEN job_attempts.finished_at='' THEN excluded.finished_at ELSE job_attempts.finished_at END
		 WHERE job_attempts.kind=excluded.kind AND job_attempts.job_key=excluded.job_key AND (job_attempts.state<>excluded.state OR job_attempts.error<>excluded.error OR job_attempts.started_at='' AND excluded.started_at<>'' OR job_attempts.finished_at='' AND excluded.finished_at<>'')`,
			j.RequestID, q.kind, key, j.FileID.String(), j.BlobKey, j.State, started, finished, j.Error)
		if err != nil {
			return err
		}
	}
	for _, p := range old {
		if kept[p.key] {
			continue
		}
		if p.state == "queued" || p.state == "running" {
			state := "cancelled"
			if slices.Contains(s.completed, p.request) {
				state = "succeeded"
			}
			if _, err = tx.Exec(`UPDATE job_attempts SET state=?,finished_at=? WHERE request_id=?`, state, now, p.request); err != nil {
				return err
			}
		}
		if _, err = tx.Exec(`DELETE FROM background_jobs WHERE kind=? AND job_key=?`, q.kind, p.key); err != nil {
			return err
		}
	}
	return nil
}
