package library

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/jinzhao1994/kist/internal/blob"
	"github.com/jinzhao1994/kist/internal/derive"
	"github.com/jinzhao1994/kist/internal/runtimestore"
)

func saveMediaTx(tx *sql.Tx, kind string, j MediaJob) error {
	raw, err := json.Marshal(j.Result)
	if err != nil {
		return err
	}
	_, err = tx.Exec(`INSERT INTO background_jobs(kind,job_key,blob_key,recipe,state,error,request_id,position,updated_at,result)
	 VALUES(?,?,?,?,?,?,?,?,?,?) ON CONFLICT(kind,job_key) DO UPDATE SET state=excluded.state,error=excluded.error,
	 request_id=excluded.request_id,position=excluded.position,updated_at=excluded.updated_at,result=excluded.result`,
		kind, j.BlobKey+"/"+j.Recipe, j.BlobKey, j.Recipe, j.State, j.Reason, j.Generation, j.UpdatedAt.UnixNano(), j.UpdatedAt.Format(time.RFC3339Nano), string(raw))
	if err != nil {
		return err
	}
	if j.Generation == "" {
		return nil
	}
	state := j.State
	started, finished := "", ""
	if state == "running" {
		started = j.UpdatedAt.Format(time.RFC3339Nano)
	}
	if state == "ready" {
		state = "succeeded"
	}
	if state == "succeeded" || state == "failed" || state == "cancelled" || state == "blocked" {
		finished = j.UpdatedAt.Format(time.RFC3339Nano)
	}
	_, err = tx.Exec(`INSERT INTO job_attempts(request_id,kind,job_key,file_id,blob_key,state,started_at,finished_at,error) VALUES(?,?,?,'',?,?,?,?,?) ON CONFLICT(request_id) DO UPDATE SET state=excluded.state,error=excluded.error,started_at=CASE WHEN job_attempts.started_at='' THEN excluded.started_at ELSE job_attempts.started_at END,finished_at=excluded.finished_at`, kind+":"+j.Generation, kind, j.BlobKey+"/"+j.Recipe, j.BlobKey, state, started, finished, j.Reason)
	if err == nil && kind == "media" {
		err = savePreviewCacheTx(tx, j, j.UpdatedAt)
	}
	return err
}

func scanMediaJob(row rowScanner) (MediaJob, error) {
	var j MediaJob
	var raw, updated string
	err := row.Scan(&j.BlobKey, &j.Recipe, &j.State, &j.Reason, &j.Generation, &updated, &raw)
	if err != nil {
		return j, err
	}
	if j.UpdatedAt, err = time.Parse(time.RFC3339Nano, updated); err != nil {
		return j, err
	}
	err = json.Unmarshal([]byte(raw), &j.Result)
	return j, err
}

const mediaColumns = `blob_key,recipe,state,error,request_id,updated_at,result`

func readMedia(db *runtimestore.Store, kind string, j MediaJob) (MediaJob, error) {
	got, err := scanMediaJob(db.DB.QueryRow(`SELECT `+mediaColumns+` FROM background_jobs WHERE kind=? AND job_key=?`, kind, j.BlobKey+"/"+j.Recipe))
	if errors.Is(err, sql.ErrNoRows) {
		return j, nil
	}
	if err == nil && (got.BlobKey != j.BlobKey || got.Recipe != j.Recipe) {
		return j, ErrCacheMetadata
	}
	return got, err
}

func importMedia(db *runtimestore.Store, kind, root string, pathFor func(MediaJob) (string, error)) error {
	return db.Import(context.Background(), kind, func(tx *sql.Tx) error {
		return filepath.WalkDir(root, func(path string, d os.DirEntry, e error) error {
			if errors.Is(e, os.ErrNotExist) {
				return nil
			}
			if e != nil {
				return e
			}
			if d.IsDir() || !strings.HasSuffix(d.Name(), ".json") {
				return nil
			}
			f, err := os.Open(path)
			if err != nil {
				return err
			}
			var j MediaJob
			err = json.NewDecoder(io.LimitReader(f, 16<<10)).Decode(&j)
			f.Close()
			// Other extraction queues share derived/, but not media identities.
			if kind == "media" {
				rel, _ := filepath.Rel(root, path)
				if len(strings.Split(filepath.ToSlash(rel), "/")) != 5 {
					return nil
				}
			}
			if err != nil {
				return err
			}
			expected, err := pathFor(j)
			if err != nil || expected != path {
				return ErrCacheMetadata
			}
			if _, ok := derive.Lookup(j.Recipe); !ok && (j.State == "queued" || j.State == "running" || j.State == "blocked") {
				j.State = "failed"
				j.Reason = "unsupported_recipe"
			}
			return saveMediaTx(tx, kind, j)
		})
	})
}

func (m *Media) database() (*runtimestore.Store, error) {
	db, err := m.library.runtimeStore()
	if err != nil {
		return nil, err
	}
	err = importMedia(db, "media", m.library.layout.Derived(), func(j MediaJob) (string, error) {
		key, e := blob.ParseKey(j.BlobKey)
		if e != nil {
			return "", e
		}
		if !safeRecipe(j.Recipe) {
			return "", ErrInvalidRequest
		}
		return filepath.Join(m.library.layout.Derived(), filepath.FromSlash(key.RelPath()), j.Recipe+".json"), nil
	})
	if err == nil {
		err = adoptPreviewCache(db)
	}
	return db, err
}
func (m *ClientMedia) database() (*runtimestore.Store, error) {
	db, err := m.client.runtimeStore()
	if err != nil {
		return nil, err
	}
	err = importMedia(db, "client-media", filepath.Join(m.client.root, "cache", "media-jobs"), func(j MediaJob) (string, error) {
		p, e := m.client.legacyDerivedPath(j.BlobKey, j.Recipe)
		return filepath.Join(m.client.root, "cache", "media-jobs", filepath.Base(p)), e
	})
	return db, err
}

func claimMedia(db *runtimestore.Store, kind string, client bool) (MediaJob, bool, error) {
	now := time.Now()
	queued := now.UnixNano()
	reason := "%"
	if client {
		queued = now.Add(-time.Second).UnixNano()
		reason = "dependency_missing"
	}
	j, err := scanMediaJob(db.DB.QueryRow(`SELECT `+mediaColumns+` FROM background_jobs WHERE kind=? AND job_key=(
 SELECT job_key FROM (
 SELECT * FROM (SELECT job_key,position FROM background_jobs WHERE kind=? AND state='running' ORDER BY position,job_key LIMIT 1)
 UNION ALL SELECT * FROM (SELECT job_key,position FROM background_jobs WHERE kind=? AND state='queued' AND position<=? ORDER BY position,job_key LIMIT 1)
 UNION ALL SELECT * FROM (SELECT job_key,position FROM background_jobs WHERE kind=? AND state='blocked' AND position<=? AND error LIKE ? ORDER BY position,job_key LIMIT 1)
 ) ORDER BY position,job_key LIMIT 1)`, kind, kind, kind, queued, kind, now.Add(-30*time.Second).UnixNano(), reason))
	if errors.Is(err, sql.ErrNoRows) {
		return j, false, nil
	}
	if err != nil {
		return j, false, err
	}
	j.State = "running"
	j.UpdatedAt = now.UTC()
	err = db.Transaction(context.Background(), func(tx *sql.Tx) error { return saveMediaTx(tx, kind, j) })
	return j, err == nil, err
}

func (m *Media) cleanJobTemps(j MediaJob) error {
	_, out, err := m.paths(j)
	if err != nil {
		return err
	}
	dir := filepath.Dir(out)
	entries, err := os.ReadDir(dir)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".render-") || strings.HasPrefix(entry.Name(), ".heif-") {
			if err = os.RemoveAll(filepath.Join(dir, entry.Name())); err != nil {
				return err
			}
		}
	}
	return nil
}
