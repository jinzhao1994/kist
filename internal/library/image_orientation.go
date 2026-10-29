package library

import (
	"context"
	"errors"
	"reflect"
	"strconv"

	"github.com/google/uuid"
	"github.com/jinzhao1994/kist/internal/ai"
	"github.com/jinzhao1994/kist/internal/domain"
	"github.com/jinzhao1994/kist/internal/extract"
	"github.com/jinzhao1994/kist/internal/oplog"
	"github.com/jinzhao1994/kist/internal/store"
)

// imageRotation reads the current content version's orientation. Replacement
// drops the angle in its payload; no input checksum is part of the result.
func imageRotation(rows []store.MetadataRow) (degrees int, known, manual bool) {
	var angle *store.MetadataRow
	for i := range rows {
		r := &rows[i]
		if r.Tombstone {
			continue
		}
		if r.Key == "orientation:clockwise_degrees" {
			angle = r
		}
	}
	if angle == nil || angle.Type != extract.TypeInt {
		return 0, false, false
	}
	n, err := strconv.Atoi(angle.Value)
	if err != nil || (n != 0 && n != 90 && n != 180 && n != 270) {
		return 0, false, false
	}
	if angle.Source != extract.UserSource && angle.Source != "orientation" {
		return 0, false, false
	}
	return n, true, angle.Source == extract.UserSource
}

func (l *Library) ensureImageOrientation(ctx context.Context, id domain.FileID, provider ai.Provider, model, blob string) error {
	q := l.OrientationQueue()
	// Dependencies share the same durable queue and worker lock as standalone
	// checks, so another task cannot retry a failed or interrupted inference.
	if ctx.Value(aiWorkerContextKey{}) != l {
		release, err := q.worker.Acquire(ctx)
		if err != nil {
			return err
		}
		defer release()
	}
	job, err := q.resolve(ctx, id)
	if err != nil {
		return err
	}
	if job.BlobKey != blob {
		return ErrOCRChanged
	}
	if job.State == "ready" {
		return nil
	}
	unlock, err := q.state.Acquire(ctx)
	if err != nil {
		return err
	}
	state, err := q.load()
	if err != nil {
		unlock()
		return err
	}
	manual, _ := ctx.Value(aiManualContextKey{}).(bool)
	if state.Paused && !manual {
		unlock()
		return errors.New("image orientation queue is paused; resume it in AI settings")
	}
	index := -1
	for i, previous := range state.Jobs {
		if previous.FileID != id || previous.BlobKey != blob {
			continue
		}
		if previous.State == "failed" || previous.State == "running" {
			unlock()
			return errors.New("image orientation failed or was interrupted; retry it explicitly in AI settings")
		}
		if previous.State == "queued" {
			index = i
			job = previous
			break
		}
	}
	if index < 0 {
		job.RequestID = uuid.NewString()
		state.Jobs = append(state.Jobs, job)
		index = len(state.Jobs) - 1
	}
	job.State, job.Automatic, job.Manual = "running", false, manual
	state.Jobs[index] = job
	err = q.save(state)
	unlock()
	if err != nil {
		return err
	}
	if err := q.selectAttemptModel(job, model); err != nil {
		return errors.Join(err, q.finishJob(job, err))
	}
	prepared, err := l.prepareImageText(q.attemptContext(ctx, job), id, provider, model, blob, "orientation", "", job.Force)
	if err == nil {
		_, err = l.ExecuteRequest(ctx, q.jobRequest(job), prepared.Commit)
	}
	if err == nil {
		err = l.readTx(ctx, func(tx *store.ViewTx) error {
			file, found, e := tx.GetFile(id)
			if e != nil {
				return e
			}
			if !found || file.BlobKey != blob {
				return ErrOCRChanged
			}
			rows, e := store.FileMetadata(tx.Queryer(), id)
			if e != nil {
				return e
			}
			if _, known, _ := imageRotation(rows); !known {
				return errors.New("image orientation metadata is protected; restore automatic orientation before retrying")
			}
			return nil
		})
	}
	return errors.Join(err, q.finishJob(job, err))
}

// SetImageRotation records an explicit human override, or releases it when nil.
// Existing metadata actions provide strict undo and deterministic replay.
func (l *Library) SetImageRotation(ctx context.Context, id domain.FileID, blob string, degrees *int) error {
	if degrees != nil && *degrees != 0 && *degrees != 90 && *degrees != 180 && *degrees != 270 {
		return ErrInvalidRequest
	}
	var change oplog.FileMetadataChange
	err := l.readTx(ctx, func(tx *store.ViewTx) error {
		f, ok, err := tx.GetFile(id)
		if err != nil {
			return err
		}
		if !ok {
			return ErrNoSuchFile
		}
		if f.BlobKey != blob {
			return ErrOCRChanged
		}
		if degrees != nil && !l.OrientationQueue().eligible(OCRJob{BlobKey: blob}) {
			return ErrInvalidRequest
		}
		rows, err := store.FileMetadata(tx.Queryer(), id)
		if err != nil {
			return err
		}
		change = oplog.FileMetadataChange{FileID: id, BlobKey: blob, CapturedAt: f.CapturedAt, CapturedWas: f.CapturedAt}
		const key = "orientation:clockwise_degrees"
		var before, after *oplog.MetadataState
		for _, row := range rows {
			if row.Key == key {
				before = &oplog.MetadataState{Value: row.Value, Type: row.Type, Source: row.Source, Tombstone: row.Tombstone}
				break
			}
		}
		if degrees == nil {
			if before == nil || before.Source != extract.UserSource {
				return nil
			}
		} else {
			after = &oplog.MetadataState{Value: strconv.Itoa(*degrees), Type: extract.TypeInt, Source: extract.UserSource}
		}
		if !reflect.DeepEqual(before, after) {
			change.Keys = append(change.Keys, oplog.KeyChange{Key: key, Before: before, After: after})
		}
		return nil
	})
	if err != nil || len(change.Keys) == 0 {
		return err
	}
	raw, err := oplog.Marshal(oplog.SetMetadataPayload{Files: []oplog.FileMetadataChange{change}})
	if err != nil {
		return err
	}
	_, err = l.write(ctx, oplog.ActionSetMetadata, raw, oplog.OriginUser, writeOptions{})
	return err
}
