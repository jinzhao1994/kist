package library

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/jinzhao1994/kist/internal/config"
	"github.com/jinzhao1994/kist/internal/lockfile"
	"github.com/jinzhao1994/kist/internal/runtimestore"
)

// runtimeHandle caches a connection, never mutable application state. Opening
// lazily keeps archive verification and replay independent of local state.
type runtimeHandle struct {
	once  sync.Once
	store *runtimestore.Store
	err   error
}

func (h *runtimeHandle) open(root string) (*runtimestore.Store, error) {
	h.once.Do(func() {
		unlock, err := lockfile.New(filepath.Join(root, "config.lock")).Acquire(context.Background())
		if err != nil {
			h.err = err
			return
		}
		defer unlock()
		path := filepath.Join(root, "runtime.db")
		cfg, err := config.Load(filepath.Join(root, "kist.toml"))
		if err != nil {
			h.err = err
			return
		}
		if cfg.RuntimeStorage != 0 {
			if _, err := os.Stat(path); err != nil {
				h.err = fmt.Errorf("runtime database is missing or unavailable; restore the local state backup: %w", err)
				return
			}
		}
		if cfg.RuntimeStorage == 0 {
			h.store, h.err = runtimestore.Open(path)
		} else {
			h.store, h.err = runtimestore.OpenExisting(path)
		}
		if h.err != nil {
			return
		}
		id := cfg.Library.ID.String()
		if cfg.Mode == "client" {
			id = cfg.Client.ID.String()
		}
		h.err = h.store.Transaction(context.Background(), func(tx *sql.Tx) error {
			var mode, saved string
			e := tx.QueryRow(`SELECT mode,id FROM instance_identity WHERE singleton=1`).Scan(&mode, &saved)
			if errors.Is(e, sql.ErrNoRows) && cfg.RuntimeStorage == 0 {
				_, e = tx.Exec(`INSERT INTO instance_identity VALUES(1,?,?)`, cfg.Mode, id)
				return e
			}
			if e != nil {
				return e
			}
			if mode != cfg.Mode || saved != id {
				return fmt.Errorf("runtime database belongs to another instance")
			}
			return nil
		})
		if h.err != nil {
			h.store.Close()
			h.store = nil
			return
		}
		// Old binaries reject this unknown configuration key instead of writing
		// stale JSON state after a downgrade. Stop old services before upgrading.
		if cfg.RuntimeStorage == 0 {
			h.err = config.Update(filepath.Join(root, "kist.toml"), "", map[string]any{"runtime_storage": 1})
			if h.err != nil {
				h.store.Close()
				h.store = nil
			}
		}
	})
	return h.store, h.err
}

func (h *runtimeHandle) close() error {
	if h.store != nil {
		return h.store.Close()
	}
	return nil
}

func (l *Library) runtimeStore() (*runtimestore.Store, error) { return l.runtime.open(l.layout.Root) }
func (c *Client) runtimeStore() (*runtimestore.Store, error)  { return c.runtime.open(c.root) }

func joinClose(archive error, runtime *runtimeHandle) error {
	return errors.Join(archive, runtime.close())
}
