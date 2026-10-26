package library

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
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
	Paused     bool
	Error      string
	AutoCursor int64
	AutoScan   *ocrScan
	Backfill   *ocrScan
	Jobs       []OCRJob
}

// OCRQueue uses OS locks and reloads state for every operation, including across
// processes. No archive lock is held while the provider performs inference.
type OCRQueue struct {
	kind          string
	library       *Library
	state, worker *lockfile.Locker
	provider      func(context.Context) (ai.Provider, error)
}

func (l *Library) OCRQueue() *OCRQueue         { return l.imageTextQueue("ocr") }
func (l *Library) DescriptionQueue() *OCRQueue { return l.imageTextQueue("desc") }
func (l *Library) imageTextQueue(kind string) *OCRQueue {
	return &OCRQueue{kind: kind, library: l, state: lockfile.New(filepath.Join(l.layout.Root, kind+"-state.lock")), worker: lockfile.New(filepath.Join(l.layout.Root, "ocr-worker.lock")), provider: l.AIProvider}
}
func (q *OCRQueue) path() string {
	return filepath.Join(q.library.layout.Derived(), q.kind, "queue.json")
}
func (q *OCRQueue) load() (ocrQueueState, error) {
	var s ocrQueueState
	f, err := os.Open(q.path())
	if errors.Is(err, os.ErrNotExist) {
		return s, nil
	}
	if err != nil {
		return s, err
	}
	defer f.Close()
	err = json.NewDecoder(io.LimitReader(f, 32<<20)).Decode(&s)
	if err != nil {
		return s, errors.New("AI task queue state is unreadable")
	}
	return s, nil
}
func (q *OCRQueue) save(s ocrQueueState) error {
	if err := os.MkdirAll(filepath.Dir(q.path()), 0700); err != nil {
		return err
	}
	raw, err := json.Marshal(s)
	if err != nil {
		return err
	}
	if old, err := os.ReadFile(q.path()); err == nil && bytes.Equal(old, raw) {
		return nil
	}
	return config.WriteAtomic(q.path(), raw)
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
	s, err := q.load()
	return queueStatus(s), err
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
	if cfg.Model == "" {
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
		if versions[q.kind] >= 1 {
			j.State = "ready"
		}
		return err
	})
	return j, err
}
func (q *OCRQueue) FileStatus(ctx context.Context, id domain.FileID) (OCRJob, error) {
	j, err := q.resolve(ctx, id)
	if err != nil || j.State == "ready" {
		return j, err
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
		if old.FileID == id && old.BlobKey == j.BlobKey {
			return old, nil
		}
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
	return err == nil && (ocrExtractor{}).Wants(kind)
}
func enqueueOCR(s *ocrQueueState, j OCRJob, retry, automatic bool) OCRJob {
	for i, old := range s.Jobs {
		if old.FileID == j.FileID && old.BlobKey == j.BlobKey {
			if old.State != "failed" || !retry {
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
	j.RequestID = uuid.NewString()
	j.Automatic = automatic
	s.Jobs = append(s.Jobs, j)
	return j
}
func (q *OCRQueue) Request(ctx context.Context, id domain.FileID, retry bool) (OCRJob, error) {
	cfg, err := q.library.OCRSettings(ctx)
	if err != nil {
		return OCRJob{}, err
	}
	if cfg.Model == "" {
		return OCRJob{}, errors.New("choose an AI model in Settings first")
	}
	j, err := q.resolve(ctx, id)
	if err != nil || j.State == "ready" {
		return j, err
	}
	if !q.eligible(j) {
		return j, errors.New("AI extraction supports JPEG, PNG and static WebP images")
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
	head, err := q.library.SyncHead(ctx)
	if err != nil {
		return err
	}
	cfg, err := config.Load(q.library.layout.ConfigPath())
	if err != nil {
		return err
	}
	auto, after := cfg.AIOptions.AutoOCR, cfg.AIOptions.AfterOp
	if q.kind == "desc" {
		auto, after = cfg.AIOptions.AutoDesc, cfg.AIOptions.DescAfterOp
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
			if e == nil && current.State == "ready" {
				s.Jobs[i].State = "ready"
			} else {
				s.Jobs[i].State = "failed"
				s.Jobs[i].Error = "AI extraction was interrupted; retry explicitly"
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
			if errors.Is(e, ErrNoSuchFile) || e == nil && (current.State == "ready" || current.BlobKey != j.BlobKey) {
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
	if s.Paused {
		err = q.save(s)
		unlock()
		return err
	}
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
	index := -1
	for i := 0; i < len(s.Jobs); {
		j := s.Jobs[i]
		if j.State != "queued" {
			i++
			continue
		}
		current, e := q.resolve(ctx, j.FileID)
		if errors.Is(e, ErrNoSuchFile) || e == nil && (current.State == "ready" || current.BlobKey != j.BlobKey) {
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
	err = q.execute(ctx, job, cfg.AIOptions.Model, cfg.Library.Language)
	// Use a bounded independent context to record shutdown/cancellation failures.
	finishCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	unlock, e := q.state.Acquire(finishCtx)
	if e != nil {
		return e
	}
	defer unlock()
	s, e = q.load()
	if e != nil {
		return e
	}
	for i, j := range s.Jobs {
		if j.RequestID == job.RequestID {
			if err == nil {
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
	if model == "" {
		return errors.New("choose an AI model in Settings first")
	}
	current, err := q.resolve(ctx, j.FileID)
	if err != nil {
		return err
	}
	if current.State == "ready" {
		return nil
	}
	if current.BlobKey != j.BlobKey {
		return ErrOCRChanged
	}
	provider, err := q.provider(ctx)
	if err != nil {
		return err
	}
	prepared, err := q.library.prepareImageText(ctx, j.FileID, provider, model, j.BlobKey, q.kind, language)
	if err != nil {
		return err
	}
	sum := sha256.Sum256([]byte(j.FileID.String() + "\n" + j.BlobKey + "\n" + q.kind + ":1"))
	_, err = q.library.ExecuteRequest(ctx, Request{ID: j.RequestID, Digest: hex.EncodeToString(sum[:])}, prepared.Commit)
	return err
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
						s.Error = "AI task queue could not continue; check configuration and storage"
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
