package runtimestore

import (
	"database/sql"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
)

func TestDurabilityImportRollbackAndVersion(t *testing.T) {
	path := filepath.Join(t.TempDir(), "runtime.db")
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	want := errors.New("interrupted")
	if err = s.Import(t.Context(), "legacy", func(tx *sql.Tx) error {
		_, e := tx.Exec(`INSERT INTO service_limits VALUES('test','future')`)
		if e != nil {
			return e
		}
		return want
	}); !errors.Is(err, want) {
		t.Fatal(err)
	}
	var n int
	if err = s.DB.QueryRow(`SELECT count(*) FROM service_limits`).Scan(&n); err != nil || n != 0 {
		t.Fatal("partial import", err)
	}
	if err = s.Import(t.Context(), "legacy", func(tx *sql.Tx) error {
		_, e := tx.Exec(`INSERT INTO service_limits VALUES('test','future')`)
		return e
	}); err != nil {
		t.Fatal(err)
	}
	other, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer other.Close()
	if err = other.Import(t.Context(), "legacy", func(*sql.Tx) error { return want }); err != nil {
		t.Fatal("reimported stale state", err)
	}
	if err = other.DB.QueryRow(`SELECT count(*) FROM service_limits`).Scan(&n); err != nil || n != 1 {
		t.Fatal("not visible across handles", err)
	}
	var synchronous int
	if err = other.DB.QueryRow(`PRAGMA synchronous`).Scan(&synchronous); err != nil || synchronous != 2 {
		t.Fatal("runtime state is not fully synchronous", synchronous, err)
	}
	if _, err = other.DB.Exec(`PRAGMA user_version=999`); err != nil {
		t.Fatal(err)
	}
	if opened, err := Open(path); err == nil {
		opened.Close()
		t.Fatal("unknown schema accepted")
	}
}

func TestVersionOneUpgradePreservesLocalState(t *testing.T) {
	path := filepath.Join(t.TempDir(), "runtime.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(schema); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`INSERT INTO service_limits VALUES('existing','retained'); PRAGMA user_version=1`); err != nil {
		t.Fatal(err)
	}
	db.Close()
	s, err := OpenExisting(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	var saved string
	if err = s.DB.QueryRow(`SELECT next_at FROM service_limits WHERE service='existing'`).Scan(&saved); err != nil || saved != "retained" {
		t.Fatal(saved, err)
	}
	if _, err = s.DB.Exec(`INSERT INTO video_segments VALUES('synthetic','hls_1080p_v1',0,188,'digest','now')`); err != nil {
		t.Fatal(err)
	}
}

func TestIndexedRuntimeLookups(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "runtime.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	err = s.Transaction(t.Context(), func(tx *sql.Tx) error {
		stmt, err := tx.Prepare(`INSERT INTO background_jobs(kind,job_key,file_id,blob_key,state,position) VALUES('ocr',?,?,?,'queued',?)`)
		if err != nil {
			return err
		}
		defer stmt.Close()
		for i := range 10000 {
			if _, err = stmt.Exec(fmt.Sprint(i), fmt.Sprint(i), "synthetic-blob", i); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, query := range []string{
		`SELECT state FROM background_jobs WHERE kind='ocr' AND job_key='9000'`,
		`SELECT job_key FROM background_jobs WHERE kind='ocr' AND state='queued' ORDER BY position,job_key LIMIT 1`,
		`SELECT request_id FROM job_attempts WHERE file_id='9000' AND kind='ocr' AND sequence<100 ORDER BY sequence DESC LIMIT 100`,
	} {
		rows, err := s.DB.Query("EXPLAIN QUERY PLAN " + query)
		if err != nil {
			t.Fatal(err)
		}
		var plan string
		for rows.Next() {
			var a, b, c int
			var detail string
			if err = rows.Scan(&a, &b, &c, &detail); err != nil {
				t.Fatal(err)
			}
			plan += detail + "\n"
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(plan, "SEARCH") || strings.Contains(plan, "SCAN ") || strings.Contains(plan, "TEMP B-TREE") {
			t.Fatal("unindexed runtime query", plan)
		}
	}
}
