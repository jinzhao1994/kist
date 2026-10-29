package upload

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"time"

	"github.com/jinzhao1994/kist/internal/lockfile"
)

// The library supplies its instance-local database. Staging has no handle to
// archive tables and cannot publish archive facts or pending operations.
func (s *Store) namespace() string { return filepath.Base(s.path) }
func (s *Store) migrate() error {
	ctx := context.Background()
	release, err := lockfile.New(filepath.Join(s.path, "locks", "registry.lock")).Acquire(ctx)
	if err != nil {
		return err
	}
	defer release()
	source := "uploads/" + s.namespace()
	var count int
	if err = s.db.QueryRow(`SELECT count(*) FROM migrations WHERE source=?`, source).Scan(&count); err != nil {
		return err
	}
	if count != 0 {
		return nil
	}
	root, err := os.OpenRoot(s.path)
	if err != nil {
		return err
	}
	defer root.Close()
	entries, err := os.ReadDir(s.path)
	if err != nil {
		return err
	}
	var states []State
	for _, entry := range entries {
		if !validDigest(entry.Name()) {
			continue
		}
		unlock, e := s.lock(ctx, entry.Name())
		if e != nil {
			return e
		}
		defer unlock()
		state, e := readState(root, entry.Name()+"/state.json")
		if e != nil {
			return e
		}
		if state.ID != entry.Name() {
			return problem("corrupt")
		}
		states = append(states, state)
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, state := range states {
		if err = s.saveTx(tx, state); err != nil {
			return err
		}
	}
	if _, err = tx.Exec(`INSERT INTO migrations(source) VALUES(?)`, source); err != nil {
		return err
	}
	return tx.Commit()
}
func (s *Store) saveTx(tx *sql.Tx, state State) error {
	manifest := state.Manifest
	manifest.Files = nil
	raw, err := json.Marshal(manifest)
	if err != nil {
		return err
	}
	_, err = tx.Exec(`INSERT INTO upload_sessions VALUES(?,?,?,?,?,?,?,?,?) ON CONFLICT(namespace,id) DO UPDATE SET version=excluded.version,manifest=excluded.manifest,expires=excluded.expires,canceled=excluded.canceled,op_id=excluded.op_id,local_id=excluded.local_id,noop=excluded.noop`, s.namespace(), state.ID, state.Version, string(raw), state.Expires.Format(time.RFC3339Nano), state.Canceled, state.OpID, state.LocalID, state.Noop)
	if err != nil {
		return err
	}
	for i, file := range state.Manifest.Files {
		var digest any
		if len(state.Digests) > 0 {
			digest = state.Digests[i]
		}
		_, err = tx.Exec(`INSERT INTO upload_files VALUES(?,?,?,?,?,?,?,?,?) ON CONFLICT(namespace,session_id,position) DO UPDATE SET offset=excluded.offset,digest=excluded.digest`, s.namespace(), state.ID, i, file.Path, file.LastModified, file.Size, file.SHA256, state.Offsets[i], digest)
		if err != nil {
			return err
		}
	}
	return nil
}
func (s *Store) saveState(state State) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err = s.saveTx(tx, state); err != nil {
		return err
	}
	return tx.Commit()
}
func (s *Store) readState(id string) (State, error) {
	var state State
	var raw, expires string
	err := s.db.QueryRow(`SELECT id,version,manifest,expires,canceled,op_id,local_id,noop FROM upload_sessions WHERE namespace=? AND id=?`, s.namespace(), id).Scan(&state.ID, &state.Version, &raw, &expires, &state.Canceled, &state.OpID, &state.LocalID, &state.Noop)
	if errors.Is(err, sql.ErrNoRows) {
		return state, problem("not_found")
	}
	if err != nil {
		return state, err
	}
	if err = json.Unmarshal([]byte(raw), &state.Manifest); err != nil {
		return state, problem("corrupt")
	}
	if state.Expires, err = time.Parse(time.RFC3339Nano, expires); err != nil {
		return state, problem("corrupt")
	}
	rows, err := s.db.Query(`SELECT position,path,modified,size,sha256,offset,digest FROM upload_files WHERE namespace=? AND session_id=? ORDER BY position`, s.namespace(), id)
	if err != nil {
		return state, err
	}
	defer rows.Close()
	var sealed bool
	for rows.Next() {
		var i int
		var f File
		var offset int64
		var digest sql.NullString
		if err = rows.Scan(&i, &f.Path, &f.LastModified, &f.Size, &f.SHA256, &offset, &digest); err != nil {
			return state, err
		}
		if i != len(state.Manifest.Files) || i > 0 && sealed != digest.Valid {
			return state, problem("corrupt")
		}
		sealed = digest.Valid
		state.Manifest.Files = append(state.Manifest.Files, f)
		state.Offsets = append(state.Offsets, offset)
		if sealed {
			state.Digests = append(state.Digests, digest.String)
		}
	}
	if err = rows.Err(); err != nil {
		return state, err
	}
	return validateState(state)
}
func (s *Store) stateIDs() ([]string, error) {
	rows, err := s.db.Query(`SELECT id FROM upload_sessions WHERE namespace=? ORDER BY id`, s.namespace())
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		if err = rows.Scan(&id); err != nil {
			return nil, err
		}
		if !validDigest(id) {
			return nil, problem("corrupt")
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}
func (s *Store) deleteState(id string) error {
	_, err := s.db.Exec(`DELETE FROM upload_sessions WHERE namespace=? AND id=?`, s.namespace(), id)
	return err
}
