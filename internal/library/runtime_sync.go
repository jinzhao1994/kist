package library

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"

	"github.com/jinzhao1994/kist/internal/domain"
	"github.com/jinzhao1994/kist/internal/runtimestore"
)

func (c *Client) syncDatabase() (*runtimestore.Store, error) {
	db, err := c.runtimeStore()
	if err != nil {
		return nil, err
	}
	err = db.Import(context.Background(), "client-sync", func(tx *sql.Tx) error {
		for _, name := range []string{"catalog-head", "catalog-transfer"} {
			var b SyncBoundary
			err := readClientJSON(filepath.Join(c.root, name+".json"), &b)
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			if err != nil {
				return err
			}
			if b.LibraryID.IsZero() || b.OpID < 0 || (b.OpID == 0) != (b.Sum == "") {
				return ErrSyncConflict
			}
			if err = writeBoundaryTx(tx, name, b); err != nil {
				return err
			}
		}
		// A view stamp is only a performance hint. Rebuild it from verified logs
		// instead of importing a stamp that might describe a different database.
		return nil
	})
	return db, err
}
func writeBoundaryTx(tx *sql.Tx, name string, b SyncBoundary) error {
	_, err := tx.Exec(`INSERT INTO sync_boundaries VALUES(?,?,?,?) ON CONFLICT(name) DO UPDATE SET library_id=excluded.library_id,op_id=excluded.op_id,sum=excluded.sum`, name, b.LibraryID.String(), b.OpID, b.Sum)
	return err
}
func (c *Client) readBoundary(name string) (SyncBoundary, error) {
	var b SyncBoundary
	db, err := c.syncDatabase()
	if err != nil {
		return b, err
	}
	var id string
	err = db.DB.QueryRow(`SELECT library_id,op_id,sum FROM sync_boundaries WHERE name=?`, name).Scan(&id, &b.OpID, &b.Sum)
	if errors.Is(err, sql.ErrNoRows) {
		return b, os.ErrNotExist
	}
	if err != nil {
		return b, err
	}
	b.LibraryID, err = domain.ParseID(id)
	return b, err
}
func (c *Client) writeBoundary(name string, b SyncBoundary) error {
	db, err := c.syncDatabase()
	if err != nil {
		return err
	}
	return db.Transaction(context.Background(), func(tx *sql.Tx) error { return writeBoundaryTx(tx, name, b) })
}
func (c *Client) readViewStamp() (clientViewStamp, error) {
	var s clientViewStamp
	db, err := c.syncDatabase()
	if err != nil {
		return s, err
	}
	err = db.DB.QueryRow(`SELECT version,catalog_id,catalog_digest,pending_id,pending_digest FROM view_stamps WHERE name='client'`).Scan(&s.Version, &s.CatalogID, &s.CatalogDigest, &s.PendingID, &s.PendingDigest)
	if errors.Is(err, sql.ErrNoRows) {
		return s, os.ErrNotExist
	}
	return s, err
}
func (c *Client) writeViewStamp(s clientViewStamp) error {
	db, err := c.syncDatabase()
	if err != nil {
		return err
	}
	return db.Transaction(context.Background(), func(tx *sql.Tx) error { return writeViewStampTx(tx, s) })
}
func writeViewStampTx(tx *sql.Tx, s clientViewStamp) error {
	_, err := tx.Exec(`INSERT INTO view_stamps VALUES('client',?,?,?,?,?) ON CONFLICT(name) DO UPDATE SET version=excluded.version,catalog_id=excluded.catalog_id,catalog_digest=excluded.catalog_digest,pending_id=excluded.pending_id,pending_digest=excluded.pending_digest`, s.Version, s.CatalogID, s.CatalogDigest, s.PendingID, s.PendingDigest)
	return err
}
