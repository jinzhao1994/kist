package library

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"
	"github.com/jinzhao1994/kist/internal/ai"
	"github.com/jinzhao1994/kist/internal/config"
	"github.com/jinzhao1994/kist/internal/domain"
	"github.com/jinzhao1994/kist/internal/extract"
	"github.com/jinzhao1994/kist/internal/lockfile"
	"github.com/jinzhao1994/kist/internal/store"
)

// OCRJob is disposable execution state. Completed text lives only in the archive.
type OCRJob struct {
	FileID    domain.FileID `json:"file_id"`
	BlobKey   string        `json:"blob_key"`
	Name      string        `json:"name"`
	State     string        `json:"state" enum:"missing,queued,running,ready,failed"`
	Error     string        `json:"error,omitempty"`
	RequestID string        `json:"request_id"`
	Automatic bool          `json:"automatic"`
	Manual    bool          `json:"manual,omitempty"`
	Force     bool          `json:"force,omitempty"`
}
type OCRQueueStatus struct {
	Paused   bool   `json:"paused"`
	Scanning bool   `json:"scanning"`
	Queued   int    `json:"queued"`
	Failed   int    `json:"failed"`
	Running  string `json:"running"`
	Error    string `json:"error,omitempty"`
}
type ocrScan struct {
	After, Through int64
	Cursor         domain.FileID
	Retry          bool
}
type ocrQueueState struct {
	completed  []string
	Paused     bool
	Error      string
	AutoCursor int64
	AutoScan   *ocrScan
	Backfill   *ocrScan
	Jobs       []OCRJob
}

// OCRQueue uses OS locks and reloads state for every operation, including across
// processes. No archive lock is held while the provider performs inference.
type aiWorkerContextKey struct{}
type aiManualContextKey struct{}

type OCRQueue struct {
	kind          string
	library       *Library
	state, worker *lockfile.Locker
	provider      func(context.Context) (ai.Provider, error)
}

func (l *Library) LocationQueue() *OCRQueue { return l.imageTextQueue("geo") }

func (l *Library) OrientationQueue() *OCRQueue { return l.imageTextQueue("orientation") }
func (l *Library) OCRQueue() *OCRQueue         { return l.imageTextQueue("ocr") }
func (l *Library) FactsQueue() *OCRQueue       { return l.imageTextQueue("facts") }

func (l *Library) DescriptionQueue() *OCRQueue { return l.imageTextQueue("desc") }
func (l *Library) imageTextQueue(kind string) *OCRQueue {
	return &OCRQueue{kind: kind, library: l, state: lockfile.New(filepath.Join(l.layout.Root, kind+"-state.lock")), worker: lockfile.New(filepath.Join(l.layout.Root, "ocr-worker.lock")), provider: l.AIProvider}
}
func (q *OCRQueue) path() string {
	return filepath.Join(q.library.layout.Derived(), q.kind, "queue.json")
}
func queueStatus(s ocrQueueState) OCRQueueStatus {
	out := OCRQueueStatus{Paused: s.Paused, Scanning: s.Backfill != nil || s.AutoScan != nil, Error: s.Error}
	for _, j := range s.Jobs {
		switch j.State {
		case "queued":
			out.Queued++
		case "failed":
			out.Failed++
		case "running":
			out.Running = j.Name
		}
	}
	return out
}
func (q *OCRQueue) Status(ctx context.Context) (OCRQueueStatus, error) {
	release, err := q.state.Acquire(ctx)
	if err != nil {
		return OCRQueueStatus{}, err
	}
	defer release()
	db, err := q.database()
	if err != nil {
		return OCRQueueStatus{}, err
	}
	var out OCRQueueStatus
	err = db.DB.QueryRowContext(ctx, `SELECT paused,error,auto_after IS NOT NULL OR backfill_after IS NOT NULL FROM queue_controls WHERE kind=?`, q.kind).Scan(&out.Paused, &out.Error, &out.Scanning)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return out, err
	}
	rows, err := db.DB.QueryContext(ctx, `SELECT state,count(*),min(name) FROM background_jobs WHERE kind=? GROUP BY state`, q.kind)
	if err != nil {
		return out, err
	}
	defer rows.Close()
	for rows.Next() {
		var state, name string
		var count int
		if err = rows.Scan(&state, &count, &name); err != nil {
			return out, err
		}
		switch state {
		case "queued":
			out.Queued = count
		case "failed":
			out.Failed = count
		case "running":
			out.Running = name
		}
	}
	return out, rows.Err()
}
func (q *OCRQueue) Pause(ctx context.Context, paused bool) (OCRQueueStatus, error) {
	release, err := q.state.Acquire(ctx)
	if err != nil {
		return OCRQueueStatus{}, err
	}
	defer release()
	s, err := q.load()
	if err != nil {
		return OCRQueueStatus{}, err
	}
	s.Paused = paused
	if !paused {
		s.Error = ""
	}
	return queueStatus(s), q.save(s)
}
func (q *OCRQueue) Backfill(ctx context.Context, retry bool) (OCRQueueStatus, error) {
	cfg, err := q.library.OCRSettings(ctx)
	if err != nil {
		return OCRQueueStatus{}, err
	}
	if q.kind != "geo" && cfg.Model == "" {
		return OCRQueueStatus{}, errors.New("choose an AI model in Settings first")
	}
	head, err := q.library.SyncHead(ctx)
	if err != nil {
		return OCRQueueStatus{}, err
	}
	release, err := q.state.Acquire(ctx)
	if err != nil {
		return OCRQueueStatus{}, err
	}
	defer release()
	s, err := q.load()
	if err != nil {
		return OCRQueueStatus{}, err
	}
	// Completed files are absent from the missing-result scan. Explicitly
	// retry their failed refresh jobs as well, retaining the force intent.
	if retry {
		for i := range s.Jobs {
			if s.Jobs[i].Force && s.Jobs[i].State == "failed" {
				s.Jobs[i].Manual = false
				s.Jobs[i].State, s.Jobs[i].Error, s.Jobs[i].RequestID = "queued", "", uuid.NewString()
			}
		}
	}
	if s.Backfill == nil {
		s.Backfill = &ocrScan{After: -1, Through: head.OpID, Retry: retry}
	}
	return queueStatus(s), q.save(s)
}
func (q *OCRQueue) resolve(ctx context.Context, id domain.FileID) (OCRJob, error) {
	j := OCRJob{FileID: id, State: "missing"}
	err := q.library.readTx(ctx, func(tx *store.ViewTx) error {
		f, ok, err := tx.GetFile(id)
		if err != nil {
			return err
		}
		if !ok {
			return ErrNoSuchFile
		}
		j.Name = f.Name
		j.BlobKey = f.BlobKey
		versions, err := store.Extractions(tx.Queryer(), id)
		if q.kind == "orientation" {
			rows, e := store.FileMetadata(tx.Queryer(), id)
			if e != nil {
				return e
			}
			if _, known, _ := imageRotation(rows); known {
				j.State = "ready"
			}
		} else if versions[q.kind] >= 1 {
			j.State = "ready"
		}
		return err
	})
	return j, err
}
func (q *OCRQueue) FileStatus(ctx context.Context, id domain.FileID) (OCRJob, error) {
	j, err := q.resolve(ctx, id)
	if err != nil {
		return j, err
	}
	release, err := q.state.Acquire(ctx)
	if err != nil {
		return j, err
	}
	defer release()
	db, err := q.database()
	if err != nil {
		return j, err
	}
	old, err := scanOCRJob(db.DB.QueryRowContext(ctx, `SELECT file_id,blob_key,name,state,error,request_id,automatic,manual,force
	 FROM background_jobs WHERE kind=? AND job_key=?`, q.kind, id.String()+"/"+j.BlobKey))
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return j, err
	}
	if err == nil && (old.Force || j.State != "ready") {
		return old, nil
	}
	return j, nil
}
func (q *OCRQueue) eligible(j OCRJob) bool {
	path, ok := q.library.blobs.LocalPath(blobKey(j.BlobKey))
	if !ok {
		return false
	}
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()
	kind, err := extract.DetectMedia(f)
	if q.kind == "geo" {
		return err == nil && (geoExtractor{}).Wants(kind)
	}
	if err != nil {
		return false
	}
	if q.kind == "desc" || q.kind == "facts" {
		if _, ok := descriptionDocumentRecipe(j.Name, kind); ok {
			return true
		}
	}
	return (ocrExtractor{}).Wants(kind)
}
func enqueueOCR(s *ocrQueueState, j OCRJob, retry, automatic bool) OCRJob {
	for i, old := range s.Jobs {
		if old.FileID == j.FileID && old.BlobKey == j.BlobKey {
			if old.State != "failed" || !retry {
				if j.Manual {
					s.Jobs[i].Manual = true
					old.Manual = true
				}
				if !automatic {
					s.Jobs[i].Automatic = false
					old.Automatic = false
				}
				return old
			}
			s.Jobs = append(s.Jobs[:i], s.Jobs[i+1:]...)
			break
		}
	}
	j.State = "queued"
	if j.RequestID == "" {
		j.RequestID = uuid.NewString()
	}
	j.Automatic = automatic
	s.Jobs = append(s.Jobs, j)
	return j
}
func (q *OCRQueue) Request(ctx context.Context, id domain.FileID, retry bool) (OCRJob, error) {
	return q.request(ctx, id, retry, false, "")
}

// Reextract uses a stable caller key to avoid rebilling successful HTTP retries.
func (q *OCRQueue) Reextract(ctx context.Context, id domain.FileID, key string) (OCRJob, error) {
	if _, err := uuid.Parse(key); err != nil {
		return OCRJob{}, ErrInvalidRequest
	}
	return q.request(ctx, id, true, true, key)
}

func (q *OCRQueue) request(ctx context.Context, id domain.FileID, retry, force bool, key string) (OCRJob, error) {
	cfg, err := q.library.OCRSettings(ctx)
	if err != nil {
		return OCRJob{}, err
	}
	if q.kind != "geo" && cfg.Model == "" {
		return OCRJob{}, errors.New("choose an AI model in Settings first")
	}
	j, err := q.resolve(ctx, id)
	if err != nil || j.State == "ready" && !force {
		return j, err
	}
	j.Force, j.RequestID, j.Manual = force, key, true
	if force {
		if _, found, err := q.library.CheckRequest(ctx, q.jobRequest(j)); err != nil || found {
			return j, err
		}
	}
	if !q.eligible(j) {
		if q.kind == "desc" || q.kind == "facts" {
			return j, errors.New("this extraction supports images, PDF, Word and PowerPoint files")
		}
		return j, errors.New("AI extraction supports JPEG, PNG, static WebP and still HEIC/HEIF images")
	}
	release, err := q.state.Acquire(ctx)
	if err != nil {
		return j, err
	}
	defer release()
	s, err := q.load()
	if err != nil {
		return j, err
	}
	for _, old := range s.Jobs {
		if key != "" && old.RequestID == key {
			if old.FileID != id || old.BlobKey != j.BlobKey {
				return j, ErrRequestConflict
			}
			return old, nil
		}
	}
	j = enqueueOCR(&s, j, retry, false)
	return j, q.save(s)
}

// scanBatch runs under the short state lock and never performs network I/O.
func (q *OCRQueue) scanBatch(ctx context.Context, s *ocrQueueState, scan *ocrScan, automatic bool) (bool, error) {
	ids, err := store.ImageTextCandidates(q.library.db, q.kind, scan.After, scan.Through, scan.Cursor, 100)
	if err != nil {
		return false, err
	}
	for _, id := range ids {
		j, err := q.resolve(ctx, id)
		if errors.Is(err, ErrNoSuchFile) {
			scan.Cursor = id
			continue
		}
		if err != nil {
			return false, err
		}
		if j.State != "ready" && q.eligible(j) {
			enqueueOCR(s, j, scan.Retry, automatic)
		}
		scan.Cursor = id
	}
	return len(ids) < 100, nil
}

// Step owns the global worker lock across inference. Any persisted running job
// seen after acquiring it was interrupted; never automatically rebill it.
func (q *OCRQueue) Step(ctx context.Context) error {
	release, ok, err := q.worker.TryAcquire()
	if err != nil || !ok {
		return err
	}
	defer release()
	ctx = context.WithValue(ctx, aiWorkerContextKey{}, q.library)
	head, err := q.library.SyncHead(ctx)
	if err != nil {
		return err
	}
	cfg, err := config.Load(q.library.layout.ConfigPath())
	if err != nil {
		return err
	}
	auto, after := cfg.AIOptions.AutoOCR, cfg.AIOptions.AfterOp
	if q.kind == "geo" {
		auto, after = cfg.Geocoding.Automatic, cfg.Geocoding.AfterOp
	}
	if q.kind == "facts" {
		auto, after = cfg.AIOptions.AutoFacts, cfg.AIOptions.FactsAfterOp
	}
	if q.kind == "desc" {
		auto, after = cfg.AIOptions.AutoDesc, cfg.AIOptions.DescAfterOp
	}
	if q.kind == "orientation" {
		auto, after = cfg.AIOptions.AutoOrientation, cfg.AIOptions.OrientationAfterOp
	}
	unlock, err := q.state.Acquire(ctx)
	if err != nil {
		return err
	}
	s, err := q.load()
	if err != nil {
		unlock()
		return err
	}
	for i := range s.Jobs {
		if s.Jobs[i].State == "running" {
			current, e := q.resolve(ctx, s.Jobs[i].FileID)
			completed := e == nil && current.BlobKey == s.Jobs[i].BlobKey && current.State == "ready" && !s.Jobs[i].Force
			if s.Jobs[i].Force {
				_, found, err := q.library.CheckRequest(ctx, q.jobRequest(s.Jobs[i]))
				if err != nil {
					unlock()
					return err
				}
				completed = found
			}
			if completed {
				s.Jobs[i].State = "ready"
				s.completed = append(s.completed, s.Jobs[i].RequestID)
			} else {
				s.Jobs[i].State = "failed"
				s.Jobs[i].Error = "Metadata extraction was interrupted; retry explicitly"
				s.Paused = true
				s.Error = s.Jobs[i].Error
			}
		}
	}
	// Discard ready/stale jobs from the expendable queue, not from the archive.
	kept := s.Jobs[:0]
	for _, j := range s.Jobs {
		if j.State == "ready" || j.State == "queued" && j.Automatic && !auto {
			continue
		}
		if j.State == "failed" {
			current, e := q.resolve(ctx, j.FileID)
			if errors.Is(e, ErrNoSuchFile) || e == nil && (current.State == "ready" && !j.Force || current.BlobKey != j.BlobKey) {
				continue
			}
			if e != nil {
				unlock()
				return e
			}
		}
		kept = append(kept, j)
	}
	s.Jobs = kept
	// Pausing holds scans and bulk jobs, but explicit single-file requests still run.
	if !s.Paused {
		if auto {
			if s.AutoScan != nil && s.AutoScan.After < after {
				s.AutoScan = nil
			}
			if s.AutoCursor < after {
				s.AutoCursor = after
			}
			if s.AutoScan == nil && head.OpID > s.AutoCursor {
				s.AutoScan = &ocrScan{After: s.AutoCursor, Through: head.OpID}
			}
		} else {
			s.AutoScan = nil
		}
		if s.Backfill != nil {
			done, e := q.scanBatch(ctx, &s, s.Backfill, false)
			if e != nil {
				unlock()
				return e
			}
			if done {
				s.Backfill = nil
			}
		}
		if s.AutoScan != nil {
			done, e := q.scanBatch(ctx, &s, s.AutoScan, true)
			if e != nil {
				unlock()
				return e
			}
			if done {
				s.AutoCursor = s.AutoScan.Through
				s.AutoScan = nil
			}
		}
	}
	index := -1
	for i := 0; i < len(s.Jobs); {
		j := s.Jobs[i]
		if j.State != "queued" || s.Paused && !j.Manual {
			i++
			continue
		}
		current, e := q.resolve(ctx, j.FileID)
		if errors.Is(e, ErrNoSuchFile) || e == nil && (current.State == "ready" && !j.Force || current.BlobKey != j.BlobKey) {
			s.Jobs = append(s.Jobs[:i], s.Jobs[i+1:]...)
			continue
		}
		if e != nil {
			unlock()
			return e
		}
		index = i
		break
	}
	if index < 0 {
		err = q.save(s)
		unlock()
		return err
	}
	s.Jobs[index].State = "running"
	job := s.Jobs[index]
	err = q.save(s)
	unlock()
	if err != nil {
		return err
	}
	err = q.execute(context.WithValue(ctx, aiManualContextKey{}, job.Manual), job, cfg.AIOptions.Model, cfg.Library.Language)
	return q.finishJob(job, err)
}

func (q *OCRQueue) finishJob(job OCRJob, err error) error {
	// Use a bounded independent context to record shutdown/cancellation failures.
	finishCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	unlock, e := q.state.Acquire(finishCtx)
	if e != nil {
		return e
	}
	defer unlock()
	s, e := q.load()
	if e != nil {
		return e
	}
	for i, j := range s.Jobs {
		if j.RequestID == job.RequestID {
			if err == nil {
				s.completed = append(s.completed, j.RequestID)
				s.Jobs = append(s.Jobs[:i], s.Jobs[i+1:]...)
			} else {
				s.Jobs[i].State = "failed"
				s.Jobs[i].Error = err.Error()
				s.Paused = true
				s.Error = err.Error()
			}
			break
		}
	}
	return q.save(s)
}
func (q *OCRQueue) execute(ctx context.Context, j OCRJob, model, language string) error {
	ctx = q.attemptContext(ctx, j)
	if err := q.selectAttemptModel(j, model); err != nil {
		return err
	}
	if q.kind != "geo" && model == "" {
		return errors.New("choose an AI model in Settings first")
	}
	current, err := q.resolve(ctx, j.FileID)
	if err != nil {
		return err
	}
	if current.State == "ready" && !j.Force {
		return nil
	}
	if current.BlobKey != j.BlobKey {
		return ErrOCRChanged
	}
	request := q.jobRequest(j)
	if _, found, err := q.library.CheckRequest(ctx, request); err != nil || found {
		return err
	}
	// Basic extraction always precedes network metadata. Each stage has its own
	// idempotency key, so interruption does not repeat completed work.
	for _, kind := range []string{"media", "exif"} {
		prepared, err := q.library.PrepareExtraction(ctx, j.FileID, kind)
		if err != nil {
			return err
		}
		if len(prepared.payload) == 0 {
			continue
		}
		stage := q.jobRequest(j)
		stage.ID = j.RequestID + "." + kind
		if _, err = q.library.ExecuteRequest(ctx, stage, prepared.Commit); err != nil {
			return err
		}
	}
	if q.kind == "geo" {
		prepared, err := q.library.prepareLocation(ctx, j.FileID, j.BlobKey, j.Force)
		if err != nil {
			return err
		}
		_, err = q.library.ExecuteRequest(ctx, request, prepared.Commit)
		return err
	}
	provider, err := q.provider(ctx)
	if err != nil {
		return err
	}
	prepared, err := q.library.prepareImageText(ctx, j.FileID, provider, model, j.BlobKey, q.kind, language, j.Force)
	if err != nil {
		var failure *modelResponseError
		if errors.As(err, &failure) {
			if recordErr := q.recordFailedModelResponse(j, failure); recordErr != nil {
				return errors.Join(err, recordErr)
			}
		}
		return err
	}
	_, err = q.library.ExecuteRequest(ctx, request, prepared.Commit)
	return err
}

func (q *OCRQueue) jobRequest(j OCRJob) Request {
	input := j.FileID.String() + "\n" + j.BlobKey + "\n" + q.kind + ":1"
	if j.Force {
		input += ":force"
	}
	sum := sha256.Sum256([]byte(input))
	return Request{ID: j.RequestID, Digest: hex.EncodeToString(sum[:])}
}
func (q *OCRQueue) Run(ctx context.Context, companions ...*OCRQueue) {
	queues := append([]*OCRQueue{q}, companions...)
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		if ctx.Err() != nil {
			return
		}
		for _, queue := range queues {
			if ctx.Err() != nil {
				return
			}
			if err := queue.Step(ctx); err != nil && ctx.Err() == nil {
				// A state/config failure must not start network work; surface it when possible.
				unlock, e := queue.state.Acquire(ctx)
				if e == nil {
					s, e := queue.load()
					if e == nil {
						s.Paused = true
						s.Error = "Metadata task queue could not continue; check configuration and storage"
						_ = queue.save(s)
					}
					unlock()
				}
			}
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}
