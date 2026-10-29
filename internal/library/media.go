package library

import (
	"context"
	"crypto/rand"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"time"

	"github.com/jinzhao1994/kist/internal/blob"
	"github.com/jinzhao1994/kist/internal/derive"
	"github.com/jinzhao1994/kist/internal/domain"
	"github.com/jinzhao1994/kist/internal/lockfile"
)

// MediaJob is disposable runtime state, never an archive fact. Missing means
// no request has been made. Equal blob keys share jobs across file references.
type MediaJob struct {
	BlobKey    string        `json:"blob_key"`
	Recipe     string        `json:"recipe"`
	State      string        `json:"state" enum:"missing,queued,running,ready,blocked,failed,cancelled"`
	Generation string        `json:"generation"`
	Reason     string        `json:"reason,omitempty"`
	UpdatedAt  time.Time     `json:"updated_at"`
	Result     derive.Result `json:"result"`
}

// Media coordinates derived files through disk and OS locks. It contains no
// cached job state. Run belongs to the server lifetime, not an HTTP request.
type Media struct {
	library       *Library
	state, worker *lockfile.Locker
	render        func(context.Context, string, string, string) (derive.Result, error)
}

func (l *Library) Media() *Media {
	return &Media{library: l, state: lockfile.New(filepath.Join(l.layout.Root, "media-state.lock")), worker: lockfile.New(filepath.Join(l.layout.Root, "media-worker.lock")), render: (derive.Renderer{}).Render}
}

func (m *Media) resolve(ctx context.Context, id domain.FileID, recipe string) (MediaJob, error) {
	if _, ok := derive.Lookup(recipe); !ok {
		return MediaJob{}, ErrInvalidRequest
	}
	f, found, err := m.library.File(ctx, id)
	if err != nil {
		return MediaJob{}, err
	}
	if !found {
		return MediaJob{}, ErrNoSuchFile
	}
	return MediaJob{BlobKey: f.BlobKey, Recipe: recipe, State: "missing"}, nil
}

// paths validates even persisted input before constructing any path.
func (m *Media) paths(j MediaJob) (state, content string, err error) {
	key, err := blob.ParseKey(j.BlobKey)
	if err != nil {
		return "", "", err
	}
	recipe, ok := derive.Lookup(j.Recipe)
	if !ok {
		return "", "", errors.New("unknown persisted media recipe")
	}
	dir := filepath.Join(m.library.layout.Derived(), filepath.FromSlash(key.RelPath()))
	return filepath.Join(dir, j.Recipe+".json"), filepath.Join(dir, recipe.Filename()), nil
}

func (m *Media) read(j MediaJob) (MediaJob, error) {
	_, content, err := m.paths(j)
	if err != nil {
		return j, err
	}
	db, err := m.database()
	if err != nil {
		return j, err
	}
	got, err := readMedia(db, "media", j)
	if err != nil {
		return j, err
	}
	switch got.State {
	case "queued", "running", "ready", "blocked", "failed", "cancelled":
	default:
		return j, nil
	}
	if got.State == "ready" {
		st, e := os.Stat(content)
		if errors.Is(e, os.ErrNotExist) || (e == nil && st.Size() != got.Result.Bytes) {
			return j, nil
		}
		if e != nil {
			return j, e
		}
	}
	return got, nil
}

func (m *Media) save(j MediaJob) error {
	_, out, err := m.paths(j)
	if err != nil {
		return err
	}
	if err = os.MkdirAll(filepath.Dir(out), 0755); err != nil {
		return err
	}
	db, err := m.database()
	if err != nil {
		return err
	}
	return db.Transaction(context.Background(), func(tx *sql.Tx) error { return saveMediaTx(tx, "media", j) })
}

// Status rereads disk, including outputs removed since the last request.
func (m *Media) Status(ctx context.Context, id domain.FileID, recipe string) (MediaJob, error) {
	j, err := m.resolve(ctx, id, recipe)
	if err != nil {
		return j, err
	}
	release, err := m.state.Acquire(ctx)
	if err != nil {
		return j, err
	}
	defer release()
	return m.read(j)
}

// Request is idempotent unless retry explicitly restarts a terminal failure.
// It does not append an op, acquire the archive write lock, or decode bytes.
func (m *Media) Request(ctx context.Context, id domain.FileID, recipe string, retry bool) (MediaJob, error) {
	j, err := m.resolve(ctx, id, recipe)
	if err != nil {
		return j, err
	}
	return m.requestJob(ctx, j, retry)
}

func (m *Media) requestJob(ctx context.Context, j MediaJob, retry bool) (MediaJob, error) {
	release, err := m.state.Acquire(ctx)
	if err != nil {
		return j, err
	}
	defer release()
	j, err = m.read(j)
	if err != nil {
		return j, err
	}
	if j.State != "missing" && !(retry && (j.State == "failed" || j.State == "blocked" || j.State == "cancelled")) {
		return j, nil
	}
	j.State = "queued"
	j.Generation = rand.Text()
	j.Reason = ""
	j.Result = derive.Result{}
	j.UpdatedAt = time.Now().UTC()
	return j, m.save(j)
}

func (m *Media) Cancel(ctx context.Context, id domain.FileID, recipe string) (MediaJob, error) {
	j, err := m.resolve(ctx, id, recipe)
	if err != nil {
		return j, err
	}
	release, err := m.state.Acquire(ctx)
	if err != nil {
		return j, err
	}
	defer release()
	j, err = m.read(j)
	if err != nil {
		return j, err
	}
	if j.State == "queued" || j.State == "running" || j.State == "blocked" {
		j.State = "cancelled"
		j.Generation = rand.Text()
		j.Reason = ""
		j.UpdatedAt = time.Now().UTC()
		err = m.save(j)
	}
	return j, err
}

var ErrMediaNotReady = errors.New("derived media is not ready")

// Open only serves completed, atomically published output for a live file.
func (m *Media) Open(ctx context.Context, id domain.FileID, recipe string) (MediaJob, *os.File, error) {
	j, err := m.resolve(ctx, id, recipe)
	if err != nil {
		return j, nil, err
	}
	return m.openJob(ctx, j)
}

func (m *Media) openJob(ctx context.Context, j MediaJob) (MediaJob, *os.File, error) {
	release, err := m.state.Acquire(ctx)
	if err != nil {
		return j, nil, err
	}
	defer release()
	j, err = m.read(j)
	if err != nil {
		return j, nil, err
	}
	if j.State != "ready" {
		return j, nil, ErrMediaNotReady
	}
	_, path, err := m.paths(j)
	if err != nil {
		return j, nil, err
	}
	f, err := os.Open(path)
	return j, f, err
}

// Run uses a separate worker lock, allowing multiple servers to share a
// library without parallel decoders. A dead process releases this OS lock;
// only its successor may recover running jobs. Shutdown preserves the queue.
func (m *Media) Run(ctx context.Context) {
	for ctx.Err() == nil {
		release, err := m.worker.Acquire(ctx)
		if err == nil {
			err = m.work(ctx)
			release()
		}
		if err != nil && ctx.Err() == nil && !lockfile.IsBusy(err) {
			m.library.log.Error("media worker", "error", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(time.Second):
		}
	}
}

func (m *Media) work(ctx context.Context) error {
	for ctx.Err() == nil {
		job, ok, err := m.claim(ctx)
		if err != nil {
			return err
		}
		if !ok {
			return nil
		}
		if err = m.generate(ctx, job); err != nil {
			return err
		}
	}
	return ctx.Err()
}

func (m *Media) claim(ctx context.Context) (MediaJob, bool, error) {
	release, err := m.state.Acquire(ctx)
	if err != nil {
		return MediaJob{}, false, err
	}
	defer release()
	db, err := m.database()
	if err != nil {
		return MediaJob{}, false, err
	}
	j, ok, err := claimMedia(db, "media", false)
	if err == nil && ok {
		err = m.cleanJobTemps(j)
	}
	return j, ok && err == nil, err
}

func (m *Media) generate(ctx context.Context, j MediaJob) error {
	_, output, err := m.paths(j)
	if err != nil {
		return err
	}
	key, _ := blob.ParseKey(j.BlobKey)
	input, _ := m.library.blobs.LocalPath(key)
	temp, err := os.CreateTemp(filepath.Dir(output), ".render-*")
	if err != nil {
		return err
	}
	temp.Close()
	defer os.Remove(temp.Name())
	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	monitorDone := make(chan struct{})
	go func() {
		defer close(monitorDone)
		ticker := time.NewTicker(200 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-runCtx.Done():
				return
			case <-ticker.C:
				release, e := m.state.Acquire(runCtx)
				if e != nil {
					cancel()
					return
				}
				current, e := m.read(MediaJob{BlobKey: j.BlobKey, Recipe: j.Recipe, State: "missing"})
				release()
				if e != nil || current.Generation != j.Generation || current.State != "running" {
					cancel()
					return
				}
			}
		}
	}()
	result, renderErr := m.render(runCtx, input, temp.Name(), j.Recipe)
	cancel()
	<-monitorDone
	// On shutdown leave running durable; the next worker starts it again.
	if ctx.Err() != nil {
		return ctx.Err()
	}
	release, err := m.state.Acquire(ctx)
	if err != nil {
		return err
	}
	defer release()
	current, err := m.read(MediaJob{BlobKey: j.BlobKey, Recipe: j.Recipe, State: "missing"})
	if err != nil {
		return err
	}
	if current.Generation != j.Generation || current.State != "running" {
		return nil
	}
	j.UpdatedAt = time.Now().UTC()
	j.Result = result
	if renderErr != nil {
		j.State = "failed"
		j.Reason = derive.Code(renderErr)
		if j.Reason == "dependency_missing" {
			j.State = "blocked"
		}
	} else {
		if err = os.Rename(temp.Name(), output); err != nil {
			return err
		}
		j.State = "ready"
		j.Reason = ""
	}
	return m.save(j)
}
