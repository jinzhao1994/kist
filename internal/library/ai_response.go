package library

import (
	"context"
	"fmt"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jinzhao1994/kist/internal/ai"
	"github.com/jinzhao1994/kist/internal/domain"
)

// modelResponseError carries rejected output only to the local diagnostic
// writer. Error strings, queue polling and ordinary logs never contain it.
type modelResponseError struct {
	cause  error
	result ai.TextResult
}

func (e *modelResponseError) Error() string { return e.cause.Error() }
func (e *modelResponseError) Unwrap() error { return e.cause }

const maxFailedModelTextBytes = 8 << 20

// failedModelResponse is persistent, instance-local diagnostic data, not an
// archive observation. It is neither replayed nor synchronized.
type failedModelResponse struct {
	Version     int           `json:"version"`
	Kind        string        `json:"kind"`
	FileID      domain.FileID `json:"file_id"`
	BlobKey     string        `json:"blob_key"`
	RequestID   string        `json:"request_id"`
	RecordedAt  time.Time     `json:"recorded_at"`
	Error       string        `json:"error"`
	Provider    string        `json:"provider"`
	Model       string        `json:"model"`
	OutputText  string        `json:"output_text"`
	OutputBytes int           `json:"output_bytes"`
	Truncated   bool          `json:"truncated"`
}

func (q *OCRQueue) recordFailedModelResponse(job OCRJob, failure *modelResponseError) error {
	// Reject malformed persisted request identities before recording output.
	id, err := uuid.Parse(job.RequestID)
	if err != nil {
		return ErrInvalidRequest
	}
	text := failure.result.Text
	originalBytes := len(text)
	if len(text) > maxFailedModelTextBytes {
		end := maxFailedModelTextBytes
		for end > 0 && !utf8.RuneStart(text[end]) {
			end--
		}
		text = text[:end]
	}
	record := failedModelResponse{
		Version: 1, Kind: q.kind, FileID: job.FileID, BlobKey: job.BlobKey,
		RequestID: job.RequestID, RecordedAt: time.Now().UTC(),
		Error: failure.Error(), Provider: failure.result.Provider, Model: failure.result.Model,
		OutputText: text, OutputBytes: originalBytes, Truncated: len(text) != originalBytes,
	}
	db, err := q.database()
	if err != nil {
		return err
	}
	_, err = db.DB.Exec(`INSERT INTO job_attempts(request_id,kind,job_key,file_id,blob_key,state,error,provider,model,output_text,output_bytes,truncated)
	 VALUES(?,?,?,?,?,'failed',?,?,?,?,?,?) ON CONFLICT(request_id) DO UPDATE SET
	 provider=excluded.provider,model=excluded.model,output_text=excluded.output_text,output_bytes=excluded.output_bytes,truncated=excluded.truncated`,
		id.String(), q.kind, job.FileID.String()+"/"+job.BlobKey, job.FileID.String(), job.BlobKey, record.Error, record.Provider, record.Model, record.OutputText, record.OutputBytes, record.Truncated)
	if err != nil {
		return fmt.Errorf("record model response: %w", err)
	}
	return nil
}

type modelAttemptKey struct{}
type modelAttempt struct {
	kind   string
	record func(ai.TextResult) error
}

func (q *OCRQueue) attemptContext(ctx context.Context, job OCRJob) context.Context {
	return context.WithValue(ctx, modelAttemptKey{}, modelAttempt{kind: q.kind, record: func(result ai.TextResult) error { return q.recordModelResult(job, result) }})
}
func captureModelResult(ctx context.Context, kind string, result ai.TextResult) error {
	if attempt, ok := ctx.Value(modelAttemptKey{}).(modelAttempt); ok && attempt.kind == kind {
		return attempt.record(result)
	}
	return nil
}
func (q *OCRQueue) recordModelResult(job OCRJob, result ai.TextResult) error {
	text := result.Text
	size := len(text)
	if size > maxFailedModelTextBytes {
		end := maxFailedModelTextBytes
		for end > 0 && !utf8.RuneStart(text[end]) {
			end--
		}
		text = text[:end]
	}
	db, err := q.database()
	if err != nil {
		return err
	}
	_, err = db.DB.Exec(`UPDATE job_attempts SET provider=?,model=?,output_text=?,output_bytes=?,truncated=? WHERE request_id=? AND kind=?`, result.Provider, result.Model, text, size, len(text) != size, job.RequestID, q.kind)
	return err
}

func (q *OCRQueue) selectAttemptModel(job OCRJob, model string) error {
	db, err := q.database()
	if err != nil {
		return err
	}
	_, err = db.DB.Exec(`UPDATE job_attempts SET model=? WHERE request_id=?`, model, job.RequestID)
	return err
}
