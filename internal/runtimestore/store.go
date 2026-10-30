// Package runtimestore persists instance-local execution and transfer state.
// It is independent of the replayable archive database and operation log.
package runtimestore

import (
	"context"
	"database/sql"
	_ "embed"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	_ "modernc.org/sqlite"
)

const Version = 3

//go:embed schema.sql
var schema string

//go:embed video.sql
var videoSchema string

//go:embed previews.sql
var previewSchema string

type Store struct{ DB *sql.DB }

// Open never repairs or replaces a damaged database. Unlike archive views,
// local execution history and retention choices cannot be replayed from ops.
func Open(path string) (*Store, error) {
	return open(path, false)
}

// OpenExisting refuses to initialize a missing or emptied local state store.
func OpenExisting(path string) (*Store, error) {
	return open(path, true)
}

func open(path string, existing bool) (*Store, error) {
	if info, err := os.Lstat(path); err == nil && !info.Mode().IsRegular() {
		return nil, fmt.Errorf("runtime database is not a regular file")
	} else if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	flags := os.O_RDWR
	if !existing {
		flags |= os.O_CREATE
	}
	f, err := os.OpenFile(path, flags, 0600)
	if err != nil {
		return nil, err
	}
	if err = f.Chmod(0600); err != nil {
		f.Close()
		return nil, err
	}
	if err = f.Close(); err != nil {
		return nil, err
	}
	uriPath := filepath.ToSlash(path)
	if filepath.VolumeName(path) != "" && !strings.HasPrefix(uriPath, "/") {
		uriPath = "/" + uriPath
	}
	u := url.URL{Scheme: "file", Path: uriPath}
	q := url.Values{}
	if existing {
		q.Set("mode", "rw")
	}
	q.Set("_txlock", "immediate")
	for _, p := range []string{"journal_mode(WAL)", "foreign_keys(1)", "busy_timeout(5000)", "synchronous(FULL)"} {
		q.Add("_pragma", p)
	}
	u.RawQuery = q.Encode()
	db, err := sql.Open("sqlite", u.String())
	if err != nil {
		return nil, err
	}
	s := &Store{DB: db}
	err = s.Transaction(context.Background(), func(tx *sql.Tx) error {
		var version int
		if err := tx.QueryRow("PRAGMA user_version").Scan(&version); err != nil {
			return err
		}
		if version > Version {
			return fmt.Errorf("unsupported runtime database version %d", version)
		}
		if existing && version == 0 {
			return fmt.Errorf("runtime database has no initialized schema; restore the local state backup")
		}
		if version == Version {
			return nil
		}
		if version == 0 {
			if _, err := tx.Exec(schema); err != nil {
				return err
			}
		}
		if version < 2 {
			if _, err := tx.Exec(videoSchema); err != nil {
				return err
			}
		}
		if _, err := tx.Exec(previewSchema); err != nil {
			return err
		}
		_, err := tx.Exec("PRAGMA user_version = 3")
		return err
	})
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("open runtime database: %w", err)
	}
	return s, nil
}

func (s *Store) Close() error { return s.DB.Close() }

func (s *Store) Transaction(ctx context.Context, fn func(*sql.Tx) error) error {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err = fn(tx); err != nil {
		return err
	}
	return tx.Commit()
}

// Import runs under the caller's legacy subsystem lock. The marker and data
// commit together; even an empty import is final, so old backups never revive.
func (s *Store) Import(ctx context.Context, source string, fn func(*sql.Tx) error) error {
	var done int
	if err := s.DB.QueryRowContext(ctx, "SELECT count(*) FROM migrations WHERE source=?", source).Scan(&done); err != nil {
		return err
	}
	if done != 0 {
		return nil
	}
	return s.Transaction(ctx, func(tx *sql.Tx) error {
		var exists int
		if err := tx.QueryRowContext(ctx, "SELECT count(*) FROM migrations WHERE source=?", source).Scan(&exists); err != nil {
			return err
		}
		if exists != 0 {
			return nil
		}
		if err := fn(tx); err != nil {
			return fmt.Errorf("import %s: %w", source, err)
		}
		_, err := tx.ExecContext(ctx, "INSERT INTO migrations(source) VALUES (?)", source)
		return err
	})
}
