package library

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jinzhao1994/kist/internal/domain"
	"github.com/jinzhao1994/kist/internal/extract"
)

const metadataTaskTimeout = time.Minute

// MetadataQueue shares persistent task controls and the extraction worker.
func (l *Library) MetadataQueue() *OCRQueue { return l.imageTextQueue("metadata") }

// Priority belongs to the task type, not to arrival order or a viewing tab.
func (q *OCRQueue) Priority() int {
	if q.kind == "metadata" {
		return 100
	}
	return 0
}

func localExtractionVersions() map[string]int {
	out := map[string]int{}
	for _, e := range extract.All() {
		out[e.ID()] = e.Version()
	}
	return out
}

// RequestExtraction optionally refreshes one local extractor. Ordinary requests
// complete every missing registered extractor without forcing existing results.
func (q *OCRQueue) RequestExtraction(ctx context.Context, id domain.FileID, retry, force bool, key, extractor string) (OCRJob, error) {
	if q.kind != "metadata" {
		return OCRJob{}, ErrInvalidRequest
	}
	if extractor != "" {
		if !force {
			return OCRJob{}, ErrInvalidRequest
		}
		if _, err := extract.Lookup(extractor); err != nil {
			return OCRJob{}, ErrInvalidRequest
		}
	}
	if force {
		if _, err := uuid.Parse(key); err != nil {
			return OCRJob{}, ErrInvalidRequest
		}
	}
	return q.requestLocal(ctx, id, retry, force, key, extractor)
}

func (q *OCRQueue) requestLocal(ctx context.Context, id domain.FileID, retry, force bool, key, extractor string) (OCRJob, error) {
	j, err := q.resolve(ctx, id)
	if err != nil || j.State == "ready" && !force {
		return j, err
	}
	j.Force, j.RequestID, j.Manual, j.Extractor = force, key, true, extractor
	if force {
		if _, found, err := q.library.CheckRequest(ctx, q.jobRequest(j)); err != nil || found {
			return j, err
		}
	}
	unlock, err := q.state.Acquire(ctx)
	if err != nil {
		return j, err
	}
	defer unlock()
	s, err := q.load()
	if err != nil {
		return j, err
	}
	for _, old := range s.Jobs {
		if key != "" && old.RequestID == key {
			if old.FileID != id || old.BlobKey != j.BlobKey || old.Extractor != extractor {
				return j, ErrRequestConflict
			}
			return old, nil
		}
		if old.FileID == id && old.BlobKey == j.BlobKey && old.State != "failed" && force && (old.Extractor != extractor || !old.Force) {
			return j, ErrRequestConflict
		}
	}
	j = enqueueOCR(&s, j, retry, false)
	return j, q.save(s)
}

func (q *OCRQueue) executeMetadata(ctx context.Context, j OCRJob) error {
	if j.Extractor != "" {
		if _, err := extract.Lookup(j.Extractor); err != nil {
			return ErrInvalidRequest
		}
	}
	ctx, cancel := context.WithTimeout(ctx, metadataTaskTimeout)
	defer cancel()
	return q.executeLocalStages(ctx, j)
}

func (q *OCRQueue) executeLocalStages(ctx context.Context, j OCRJob) error {
	var failures []error
	for _, e := range extract.All() {
		if j.Extractor != "" && j.Extractor != e.ID() {
			continue
		}
		file, found, err := q.library.File(ctx, j.FileID)
		if err != nil {
			return err
		}
		if !found {
			return ErrNoSuchFile
		}
		if file.BlobKey != j.BlobKey {
			return ErrOCRChanged
		}
		prepare := q.library.PrepareExtraction
		if j.Force {
			prepare = q.library.PrepareReExtraction
		}
		p, err := prepare(ctx, j.FileID, e.ID())
		if err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			failures = append(failures, err)
			continue
		}
		if p.file != nil && p.file.BlobKey != j.BlobKey {
			return ErrOCRChanged
		}
		stage := q.jobRequest(j)
		stage.ID = j.RequestID + "." + e.ID()
		if _, err = q.library.ExecuteRequest(ctx, stage, p.Commit); err != nil {
			return err
		}
	}
	if len(failures) > 0 {
		return errors.Join(failures...)
	}
	_, err := q.library.ExecuteRequest(ctx, q.jobRequest(j), func(ctx context.Context) error {
		file, found, err := q.library.File(ctx, j.FileID)
		if err != nil {
			return err
		}
		if !found {
			return ErrNoSuchFile
		}
		if file.BlobKey != j.BlobKey {
			return ErrOCRChanged
		}
		return ctx.Err()
	})
	return err
}

// A dependency is queued under its own controls; pausing local extraction holds
// automatic AI work instead of silently performing the same parsing inline.
func (q *OCRQueue) metadataReady(ctx context.Context, j OCRJob) (bool, error) {
	if q.kind == "metadata" {
		return true, nil
	}
	local := q.library.MetadataQueue()
	current, err := local.resolve(ctx, j.FileID)
	if err != nil {
		return false, err
	}
	if current.State == "ready" {
		return true, nil
	}
	current.Manual = j.Manual
	unlock, err := local.state.Acquire(ctx)
	if err != nil {
		return false, err
	}
	defer unlock()
	s, err := local.load()
	if err != nil {
		return false, err
	}
	enqueueOCR(&s, current, false, false)
	return false, local.save(s)
}
