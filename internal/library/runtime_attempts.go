package library

import (
	"context"
	"database/sql"
	"errors"

	"github.com/google/uuid"
	"github.com/jinzhao1994/kist/internal/domain"
)

// ExtractionAttempt is local execution history, not archive metadata. Lists
// deliberately omit model output; details require an explicit request identity.
type ExtractionAttempt struct {
	Sequence    int64  `json:"sequence"`
	RequestID   string `json:"request_id"`
	Kind        string `json:"kind"`
	FileID      string `json:"file_id"`
	BlobKey     string `json:"blob_key"`
	State       string `json:"state"`
	StartedAt   string `json:"started_at"`
	FinishedAt  string `json:"finished_at"`
	Error       string `json:"error,omitempty"`
	Provider    string `json:"provider,omitempty"`
	Model       string `json:"model,omitempty"`
	OutputBytes int    `json:"output_bytes"`
	Truncated   bool   `json:"truncated"`
}
type ExtractionAttemptDetail struct {
	ExtractionAttempt
	OutputText string `json:"output_text"`
}

const attemptColumns = `sequence,request_id,kind,file_id,blob_key,state,started_at,finished_at,error,provider,model,output_bytes,truncated`

func attemptTargets(a *ExtractionAttempt) []any {
	return []any{&a.Sequence, &a.RequestID, &a.Kind, &a.FileID, &a.BlobKey, &a.State, &a.StartedAt, &a.FinishedAt, &a.Error, &a.Provider, &a.Model, &a.OutputBytes, &a.Truncated}
}

func (q *OCRQueue) Attempts(ctx context.Context, id domain.FileID, before int64) ([]ExtractionAttempt, error) {
	if before < 0 {
		return nil, ErrInvalidRequest
	}
	if _, err := q.resolve(ctx, id); err != nil {
		return nil, err
	}
	unlock, err := q.state.Acquire(ctx)
	if err != nil {
		return nil, err
	}
	db, err := q.database()
	unlock()
	if err != nil {
		return nil, err
	}
	upper := before
	if upper == 0 {
		upper = 1<<63 - 1
	}
	rows, err := db.DB.QueryContext(ctx, `SELECT `+attemptColumns+` FROM job_attempts WHERE file_id=? AND kind=? AND sequence<? ORDER BY sequence DESC LIMIT 100`, id.String(), q.kind, upper)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []ExtractionAttempt{}
	for rows.Next() {
		var a ExtractionAttempt
		if err = rows.Scan(attemptTargets(&a)...); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}
func (q *OCRQueue) Attempt(ctx context.Context, id domain.FileID, request string) (ExtractionAttemptDetail, error) {
	var out ExtractionAttemptDetail
	if _, err := uuid.Parse(request); err != nil {
		return out, ErrInvalidRequest
	}
	if _, err := q.resolve(ctx, id); err != nil {
		return out, err
	}
	unlock, err := q.state.Acquire(ctx)
	if err != nil {
		return out, err
	}
	db, err := q.database()
	unlock()
	if err != nil {
		return out, err
	}
	targets := append(attemptTargets(&out.ExtractionAttempt), &out.OutputText)
	err = db.DB.QueryRowContext(ctx, `SELECT `+attemptColumns+`,output_text FROM job_attempts WHERE request_id=? AND file_id=? AND kind=?`, request, id.String(), q.kind).Scan(targets...)
	if errors.Is(err, sql.ErrNoRows) {
		return out, ErrNoSuchOp
	}
	return out, err
}
