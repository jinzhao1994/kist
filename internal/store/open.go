// Package store owns the database connection and its schema. Query code lives
// with the callers; this package only guarantees a correctly configured
// handle.
package store

import (
	"database/sql"
	"fmt"
	"net/url"

	_ "modernc.org/sqlite"
)

// pragmas are appended to every DSN.
//
// They must travel in the DSN rather than being executed after opening:
// database/sql pools connections and opens new ones on demand, so a PRAGMA run
// once applies to exactly one connection. The failure mode of getting this
// wrong — foreign keys silently unenforced on some queries but not others — is
// close to undebuggable.
var pragmas = []string{
	"journal_mode(WAL)",   // readers do not block the writer
	"foreign_keys(1)",     // OFF by default in SQLite; without it the schema's FKs are comments
	"busy_timeout(5000)",  // wait for a lock instead of failing instantly
	"synchronous(NORMAL)", // WAL's recommended setting; blobs are already durable (spec 3.7)
}

func dsn(path string) string {
	q := url.Values{}
	for _, p := range pragmas {
		q.Add("_pragma", p)
	}
	return "file:" + path + "?" + q.Encode()
}

// Open opens the library database at path, creating it if absent.
func Open(path string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", dsn(path))
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("open database %s: %w", path, err)
	}
	return db, nil
}

// OpenMemory returns a private in-memory database with the same configuration,
// for tests.
func OpenMemory() (*sql.DB, error) {
	db, err := sql.Open("sqlite", dsn(":memory:"))
	if err != nil {
		return nil, fmt.Errorf("open in-memory database: %w", err)
	}
	// An in-memory database lives inside one connection; more than one would
	// mean more than one database.
	db.SetMaxOpenConns(1)
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, err
	}
	return db, nil
}
