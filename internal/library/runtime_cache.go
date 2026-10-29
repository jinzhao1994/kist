package library

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/jinzhao1994/kist/internal/blob"
	"github.com/jinzhao1994/kist/internal/derive"
	"github.com/jinzhao1994/kist/internal/domain"
	"github.com/jinzhao1994/kist/internal/runtimestore"
)

// cacheDatabase is called under the cache lock. Legacy records are imported
// once, including empty directories, and retained solely as pre-upgrade backups.
func (c *Client) cacheDatabase() (*runtimestore.Store, error) {
	db, err := c.runtimeStore()
	if err != nil {
		return nil, err
	}
	err = db.Import(context.Background(), "client-cache", func(tx *sql.Tx) error {
		var pins cachePins
		if err := readClientJSON(filepath.Join(c.root, "cache-pins.json"), &pins); err == nil {
			if err = savePinsTx(tx, pins); err != nil {
				return err
			}
		} else if !errors.Is(err, os.ErrNotExist) {
			return err
		}
		for _, group := range []string{"objects", "derived", "workspaces"} {
			root := filepath.Join(c.root, "cache", group)
			entries, err := os.ReadDir(root)
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			if err != nil {
				return err
			}
			for _, entry := range entries {
				if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
					continue
				}
				path := filepath.Join(root, entry.Name())
				switch group {
				case "objects":
					var r cachedOriginal
					if err = readClientJSON(path, &r); err != nil {
						return err
					}
					key := "sha256:" + strings.TrimSuffix(entry.Name(), ".json")
					if _, err = blob.ParseKey(key); err != nil {
						return err
					}
					if !r.valid(key) || r.Version != 1 {
						return ErrCacheMetadata
					}
					err = saveCacheObjectTx(tx, r)
				case "derived":
					var r cachedDerived
					if err = readClientJSON(path, &r); err != nil {
						return err
					}
					if r.Version != 1 {
						return ErrSyncVersion
					}
					expected, e := c.legacyDerivedPath(r.Descriptor.Job.BlobKey, r.Descriptor.Job.Recipe)
					if e != nil || expected != path {
						return ErrCacheMetadata
					}
					if e = validateCachedDerived(r); e != nil {
						return e
					}
					err = saveDerivedTx(tx, r)
				case "workspaces":
					var r cacheWorkspaceRecord
					if err = readClientJSON(path, &r); err != nil {
						return err
					}
					if r.ID+".json" != entry.Name() || r.Version != 1 || r.Budget <= 0 || r.CreatedAt.IsZero() {
						return ErrCacheMetadata
					}
					if id, e := domain.ParseID(r.ID); e != nil || id.IsZero() {
						return ErrCacheMetadata
					}
					err = saveWorkspaceTx(tx, r)
				}
				if err != nil {
					return fmt.Errorf("import cache %s: %w", group, err)
				}
			}
		}
		return nil
	})
	return db, err
}

func saveCacheObjectTx(tx *sql.Tx, r cachedOriginal) error {
	_, err := tx.Exec(`INSERT INTO cache_objects VALUES(?,?,?,?,?,?,?) ON CONFLICT(key) DO UPDATE SET version=excluded.version,size=excluded.size,offset=excluded.offset,complete=excluded.complete,corrupt=excluded.corrupt,accessed_at=excluded.accessed_at`, r.Key, r.Version, r.Size, r.Offset, r.Complete, r.Corrupt, r.AccessedAt.Format(time.RFC3339Nano))
	return err
}
func scanCacheObject(row rowScanner) (cachedOriginal, error) {
	var r cachedOriginal
	var at string
	err := row.Scan(&r.Key, &r.Version, &r.Size, &r.Offset, &r.Complete, &r.Corrupt, &at)
	if errors.Is(err, sql.ErrNoRows) {
		return r, os.ErrNotExist
	}
	if err != nil {
		return r, err
	}
	r.AccessedAt, err = time.Parse(time.RFC3339Nano, at)
	return r, err
}
func (c *Client) readCacheObject(key string) (cachedOriginal, error) {
	db, err := c.cacheDatabase()
	if err != nil {
		return cachedOriginal{}, err
	}
	return scanCacheObject(db.DB.QueryRow(`SELECT key,version,size,offset,complete,corrupt,accessed_at FROM cache_objects WHERE key=?`, key))
}
func (c *Client) writeCacheObject(r cachedOriginal) error {
	db, err := c.cacheDatabase()
	if err != nil {
		return err
	}
	return db.Transaction(context.Background(), func(tx *sql.Tx) error { return saveCacheObjectTx(tx, r) })
}
func (c *Client) deleteCacheObject(key string) error {
	db, err := c.cacheDatabase()
	if err != nil {
		return err
	}
	_, err = db.DB.Exec(`DELETE FROM cache_objects WHERE key=?`, key)
	return err
}
func (c *Client) cacheObjectRecords() ([]cachedOriginal, error) {
	db, err := c.cacheDatabase()
	if err != nil {
		return nil, err
	}
	rows, err := db.DB.Query(`SELECT key,version,size,offset,complete,corrupt,accessed_at FROM cache_objects ORDER BY key`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var records []cachedOriginal
	for rows.Next() {
		r, err := scanCacheObject(rows)
		if err != nil {
			return nil, err
		}
		records = append(records, r)
	}
	return records, rows.Err()
}

func savePinsTx(tx *sql.Tx, pins cachePins) error {
	if pins.Version != 1 {
		return ErrSyncVersion
	}
	for owner, selection := range pins.Selections {
		if _, ok := pins.Owners[owner]; !ok || len(selection.Files)+len(selection.Filesets) == 0 {
			return ErrCacheMetadata
		}
		for _, id := range selection.Files {
			if id.IsZero() {
				return ErrCacheMetadata
			}
		}
		for _, id := range selection.Filesets {
			if id.IsZero() {
				return ErrCacheMetadata
			}
		}
	}
	// Selection writes are small and atomic. Pins are relational, not an opaque
	// JSON map, so reverse content lookups and future quota queries can use indexes.
	if _, err := tx.Exec(`DELETE FROM cache_selections`); err != nil {
		return err
	}
	for owner, keys := range pins.Owners {
		if owner == "" {
			return ErrCacheMetadata
		}
		var selection any
		if v, ok := pins.Selections[owner]; ok {
			raw, err := json.Marshal(v)
			if err != nil {
				return err
			}
			selection = string(raw)
		}
		if _, err := tx.Exec(`INSERT INTO cache_selections VALUES(?,?)`, owner, selection); err != nil {
			return err
		}
		for _, key := range keys {
			if _, err := blob.ParseKey(key); err != nil {
				return err
			}
			if _, err := tx.Exec(`INSERT INTO cache_pins VALUES(?,?)`, owner, key); err != nil {
				return err
			}
		}
	}
	return nil
}
func (c *Client) writeCachePins(pins cachePins) error {
	db, err := c.cacheDatabase()
	if err != nil {
		return err
	}
	return db.Transaction(context.Background(), func(tx *sql.Tx) error { return savePinsTx(tx, pins) })
}
func (c *Client) readCachePins() (cachePins, error) {
	pins := cachePins{Version: 1, Owners: map[string][]string{}, Selections: map[string]ClientOfflineSelection{}}
	db, err := c.cacheDatabase()
	if err != nil {
		return pins, err
	}
	rows, err := db.DB.Query(`SELECT owner,selection FROM cache_selections ORDER BY owner`)
	if err != nil {
		return pins, err
	}
	for rows.Next() {
		var owner string
		var raw sql.NullString
		if err = rows.Scan(&owner, &raw); err != nil {
			rows.Close()
			return pins, err
		}
		pins.Owners[owner] = []string{}
		if raw.Valid {
			var selection ClientOfflineSelection
			if err = json.Unmarshal([]byte(raw.String), &selection); err != nil {
				rows.Close()
				return pins, err
			}
			pins.Selections[owner] = selection
		}
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return pins, err
	}
	rows, err = db.DB.Query(`SELECT owner,content_key FROM cache_pins ORDER BY owner,content_key`)
	if err != nil {
		return pins, err
	}
	defer rows.Close()
	for rows.Next() {
		var owner, key string
		if err = rows.Scan(&owner, &key); err != nil {
			return pins, err
		}
		pins.Owners[owner] = append(pins.Owners[owner], key)
	}
	return pins, rows.Err()
}

func saveDerivedTx(tx *sql.Tx, r cachedDerived) error {
	descriptor, err := json.Marshal(r.Descriptor)
	if err != nil {
		return err
	}
	outputs, err := json.Marshal(r.Outputs)
	if err != nil {
		return err
	}
	_, err = tx.Exec(`INSERT INTO cache_derivations VALUES(?,?,?,?,?,?) ON CONFLICT(original,recipe) DO UPDATE SET version=excluded.version,content_key=excluded.content_key,descriptor=excluded.descriptor,outputs=excluded.outputs`, r.Version, r.Descriptor.Job.BlobKey, r.Descriptor.Job.Recipe, r.Descriptor.ContentKey, string(descriptor), string(outputs))
	return err
}
func (c *Client) writeDerivedRecord(r cachedDerived) error {
	db, err := c.cacheDatabase()
	if err != nil {
		return err
	}
	return db.Transaction(context.Background(), func(tx *sql.Tx) error { return saveDerivedTx(tx, r) })
}
func (c *Client) loadDerivedRecord(path string) (cachedDerived, error) {
	var r cachedDerived
	base := strings.TrimSuffix(filepath.Base(path), ".json")
	split := strings.LastIndex(base, ".")
	if split < 0 {
		return r, ErrCacheMetadata
	}
	// Recipe identifiers contain no dots; collision suffixes stay on the key.
	db, err := c.cacheDatabase()
	if err != nil {
		return r, err
	}
	var descriptor, outputs string
	err = db.DB.QueryRow(`SELECT version,descriptor,outputs FROM cache_derivations WHERE original=? AND recipe=?`, "sha256:"+base[:split], base[split+1:]).Scan(&r.Version, &descriptor, &outputs)
	if errors.Is(err, sql.ErrNoRows) {
		return r, os.ErrNotExist
	}
	if err != nil {
		return r, err
	}
	if err = json.Unmarshal([]byte(descriptor), &r.Descriptor); err != nil {
		return r, err
	}
	err = json.Unmarshal([]byte(outputs), &r.Outputs)
	return r, err
}
func (c *Client) derivedRecordPaths() ([]string, error) {
	db, err := c.cacheDatabase()
	if err != nil {
		return nil, err
	}
	rows, err := db.DB.Query(`SELECT original,recipe FROM cache_derivations ORDER BY original,recipe`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var paths []string
	for rows.Next() {
		var original, recipe string
		if err = rows.Scan(&original, &recipe); err != nil {
			return nil, err
		}
		path, e := c.legacyDerivedPath(original, recipe)
		if e != nil {
			return nil, e
		}
		paths = append(paths, path)
	}
	return paths, rows.Err()
}
func (c *Client) deleteDerivedRecord(path string) error {
	db, err := c.cacheDatabase()
	if err != nil {
		return err
	}
	base := strings.TrimSuffix(filepath.Base(path), ".json")
	split := strings.LastIndex(base, ".")
	if split < 0 {
		return ErrCacheMetadata
	}
	_, err = db.DB.Exec(`DELETE FROM cache_derivations WHERE original=? AND recipe=?`, "sha256:"+base[:split], base[split+1:])
	return err
}

func saveWorkspaceTx(tx *sql.Tx, r cacheWorkspaceRecord) error {
	_, err := tx.Exec(`INSERT INTO cache_workspaces VALUES(?,?,?,?) ON CONFLICT(id) DO UPDATE SET version=excluded.version,budget=excluded.budget,created_at=excluded.created_at`, r.ID, r.Version, r.Budget, r.CreatedAt.Format(time.RFC3339Nano))
	return err
}
func (c *Client) writeWorkspace(r cacheWorkspaceRecord) error {
	db, err := c.cacheDatabase()
	if err != nil {
		return err
	}
	return db.Transaction(context.Background(), func(tx *sql.Tx) error { return saveWorkspaceTx(tx, r) })
}
func (c *Client) readWorkspace(id string) (cacheWorkspaceRecord, error) {
	var r cacheWorkspaceRecord
	var at string
	db, err := c.cacheDatabase()
	if err != nil {
		return r, err
	}
	err = db.DB.QueryRow(`SELECT id,version,budget,created_at FROM cache_workspaces WHERE id=?`, id).Scan(&r.ID, &r.Version, &r.Budget, &at)
	if errors.Is(err, sql.ErrNoRows) {
		return r, os.ErrNotExist
	}
	if err != nil {
		return r, err
	}
	r.CreatedAt, err = time.Parse(time.RFC3339Nano, at)
	return r, err
}
func (c *Client) deleteWorkspace(id string) error {
	db, err := c.cacheDatabase()
	if err != nil {
		return err
	}
	_, err = db.DB.Exec(`DELETE FROM cache_workspaces WHERE id=?`, id)
	return err
}
func (c *Client) workspaceIDs() ([]string, error) {
	db, err := c.cacheDatabase()
	if err != nil {
		return nil, err
	}
	rows, err := db.DB.Query(`SELECT id FROM cache_workspaces ORDER BY id`)
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
		parsed, e := domain.ParseID(id)
		if e != nil || parsed.String() != id {
			return nil, ErrCacheMetadata
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// Obsolete recipes remain useful for accounting and historical output mappings.
// Public rendering requests still require derive.Lookup in derivedPath.
func safeRecipe(recipe string) bool {
	return recipe != "" && recipe != "." && recipe != ".." && !strings.ContainsAny(recipe, "/\\\x00.")
}
func (c *Client) legacyDerivedPath(original, recipe string) (string, error) {
	if _, err := blob.ParseKey(original); err != nil {
		return "", err
	}
	if !safeRecipe(recipe) {
		return "", ErrInvalidRequest
	}
	return filepath.Join(c.root, "cache", "derived", strings.TrimPrefix(original, "sha256:")+"."+recipe+".json"), nil
}
func validateCachedDerived(r cachedDerived) error {
	if r.Version != 1 {
		return ErrSyncVersion
	}
	j := r.Descriptor.Job
	if _, err := blob.ParseKey(j.BlobKey); err != nil {
		return ErrCacheMetadata
	}
	if !safeRecipe(j.Recipe) || j.Generation == "" || j.UpdatedAt.IsZero() {
		return ErrCacheMetadata
	}
	switch j.State {
	case "ready":
		if _, err := blob.ParseKey(r.Descriptor.ContentKey); err != nil || j.Result.Bytes <= 0 || j.Result.Bytes > derive.MaxOutputBytes {
			return ErrCacheMetadata
		}
	case "queued", "running", "blocked", "failed", "cancelled":
		if r.Descriptor.ContentKey != "" {
			return ErrCacheMetadata
		}
	default:
		return ErrCacheMetadata
	}
	for key, size := range r.Outputs {
		if _, err := blob.ParseKey(key); err != nil || size <= 0 || size > derive.MaxOutputBytes {
			return ErrCacheMetadata
		}
	}
	return nil
}
