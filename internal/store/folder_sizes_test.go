package store

import (
	"context"
	"database/sql"
	"testing"
)

func sizeExec(t *testing.T, db *sql.DB, query string, args ...any) {
	t.Helper()
	if _, err := db.Exec(query, args...); err != nil {
		t.Fatal(err)
	}
}
func assertFolderSize(t *testing.T, db *sql.DB, id []byte, want int64) {
	t.Helper()
	var got int64
	if err := db.QueryRow(`SELECT total_size FROM folders WHERE id=?`, id).Scan(&got); err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("size=%d, want %d", got, want)
	}
	if drift, err := FolderSizeDrift(db); err != nil || len(drift) != 0 {
		t.Fatalf("independent recomputation: %v %v", drift, err)
	}
}
func TestFolderSizesTrackAllMutations(t *testing.T) {
	db := migrated(t)
	root := boxRoot(t, db, insertBox(t, db, "one"))
	other := boxRoot(t, db, insertBox(t, db, "two"))
	a := insertFolder(t, db, root, "a")
	child := insertFolder(t, db, a, "child")
	key := insertBlob(t, db, "sha256:aa")
	sizeExec(t, db, `UPDATE blobs SET size=10 WHERE key=?`, key)
	f := insertFile(t, db, child, "f", key)
	duplicate := insertFile(t, db, a, "duplicate", key)
	assertFolderSize(t, db, root, 20) // Count logical references, not unique blobs.
	sizeExec(t, db, `UPDATE files SET folder_id=? WHERE id=?`, root, f)
	assertFolderSize(t, db, root, 20)
	assertFolderSize(t, db, a, 10)
	sizeExec(t, db, `UPDATE folders SET parent_id=? WHERE id=?`, other, a)
	assertFolderSize(t, db, root, 10)
	assertFolderSize(t, db, other, 10)
	sizeExec(t, db, `UPDATE folders SET parent_id=? WHERE id=?`, a, child) // No-op.
	big := insertBlob(t, db, "sha256:bb")
	sizeExec(t, db, `UPDATE blobs SET size=30 WHERE key=?`, big)
	sizeExec(t, db, `UPDATE files SET blob_key=? WHERE id=?`, big, duplicate)
	assertFolderSize(t, db, other, 30)
	// Blob metadata corrections propagate to every current reference.
	sizeExec(t, db, `UPDATE blobs SET size=15 WHERE key=?`, key)
	assertFolderSize(t, db, root, 15)
	tx, err := db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(`DELETE FROM files WHERE id=?`, f); err != nil {
		t.Fatal(err)
	}
	if err := tx.Rollback(); err != nil {
		t.Fatal(err)
	}
	assertFolderSize(t, db, root, 15)
	sizeExec(t, db, `DELETE FROM files WHERE id=?`, duplicate)
	assertFolderSize(t, db, other, 0)
	insertFile(t, db, child, "restored", big)
	assertFolderSize(t, db, other, 30)
	// Reparent within the same box: shared ancestors must not double-count.
	sizeExec(t, db, `UPDATE folders SET parent_id=? WHERE id=?`, other, child)
	assertFolderSize(t, db, other, 30)
	assertFolderSize(t, db, a, 0)
	sizeExec(t, db, `UPDATE folders SET total_size=99 WHERE id=?`, a)
	drift, err := FolderSizeDrift(db)
	if err != nil || len(drift) != 1 {
		t.Fatalf("missed corruption: %v %v", drift, err)
	}
}

func TestFolderSizeUpgradeBackfillsAndRetries(t *testing.T) {
	db, err := OpenMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	p, err := migrationProvider(db)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = p.UpTo(context.Background(), 4); err != nil {
		t.Fatal(err)
	}
	root := boxRoot(t, db, insertBox(t, db, "old"))
	child := insertFolder(t, db, root, "child")
	key := insertBlob(t, db, "sha256:aa")
	insertFile(t, db, root, "one", key)
	insertFile(t, db, child, "two", key)
	// An invalid legacy aggregate must roll back the ALTER and version marker.
	sizeExec(t, db, `UPDATE blobs SET size=-1 WHERE key=?`, key)
	if err = Migrate(db); err == nil {
		t.Fatal("negative backfill accepted")
	}
	var version int64
	if err = db.QueryRow(`SELECT max(version_id) FROM goose_db_version`).Scan(&version); err != nil || version != 4 {
		t.Fatalf("migration version after rollback: %d %v", version, err)
	}
	sizeExec(t, db, `UPDATE blobs SET size=12 WHERE key=?`, key)
	if err = Migrate(db); err != nil {
		t.Fatal(err)
	}
	assertFolderSize(t, db, root, 24)
	assertFolderSize(t, db, child, 12)
	if err = Migrate(db); err != nil {
		t.Fatal(err)
	}
	assertFolderSize(t, db, root, 24)
	// Simulate SQL from a pre-upgrade binary: triggers still maintain the sum.
	insertFile(t, db, child, "old-writer", key)
	assertFolderSize(t, db, root, 36)
}

func TestMigrationRejectsNewerSchema(t *testing.T) {
	db := migrated(t)
	sizeExec(t, db, `INSERT INTO goose_db_version(version_id,is_applied) VALUES(999,true)`)
	if err := Migrate(db); err == nil {
		t.Fatal("newer schema accepted")
	}
}

// BenchmarkFolderSizeReads compares page-time aggregation with indexed totals.
// Ten thousand files share a six-level path, including repeated blob references.
func BenchmarkFolderSizeReads(b *testing.B) {
	db, err := OpenMemory()
	if err != nil {
		b.Fatal(err)
	}
	defer db.Close()
	if err = Migrate(db); err != nil {
		b.Fatal(err)
	}
	for _, q := range []string{
		`WITH RECURSIVE n(x) AS (SELECT 1 UNION SELECT x+1 FROM n WHERE x<6)
   INSERT INTO folders(id,parent_id,name,name_fold,created_at,updated_at)
   SELECT printf('%016d',x),CASE WHEN x=1 THEN NULL ELSE printf('%016d',x-1) END,printf('%d',x),printf('%d',x),0,0 FROM n`,
		`INSERT INTO blobs(key,sha256,size) VALUES('sha256:a','a',1024)`,
		`WITH RECURSIVE n(x) AS (SELECT 1 UNION SELECT x+1 FROM n WHERE x<10000)
   INSERT INTO files(id,folder_id,name,name_fold,blob_key,created_at,updated_at)
   SELECT printf('%016d',x),'0000000000000006',printf('%d',x),printf('%d',x),'sha256:a',0,0 FROM n`,
	} {
		if _, err = db.Exec(q); err != nil {
			b.Fatal(err)
		}
	}
	for name, q := range map[string]string{
		"materialized": `SELECT total_size FROM folders WHERE id='0000000000000001'`,
		"aggregate": `WITH RECURSIVE sub(id) AS (SELECT '0000000000000001' UNION SELECT f.id FROM folders f JOIN sub s ON f.parent_id=s.id)
    SELECT sum(b.size) FROM files f JOIN blobs b ON b.key=f.blob_key WHERE f.folder_id IN (SELECT id FROM sub)`,
	} {
		b.Run(name, func(b *testing.B) {
			for b.Loop() {
				var size int64
				if err := db.QueryRow(q).Scan(&size); err != nil || size != 10240000 {
					b.Fatalf("%d %v", size, err)
				}
			}
		})
	}
	if _, err := db.Exec(`INSERT INTO blobs(key,sha256,size) VALUES('sha256:b','b',2048)`); err != nil {
		b.Fatal(err)
	}
	for _, name := range []string{"replace_with_totals", "replace_without_totals"} {
		if name == "replace_without_totals" {
			if _, err := db.Exec(`DROP TRIGGER file_size_update`); err != nil {
				b.Fatal(err)
			}
		}
		b.Run(name, func(b *testing.B) {
			for b.Loop() {
				if _, err := db.Exec(`UPDATE files SET blob_key=CASE blob_key WHEN 'sha256:a' THEN 'sha256:b' ELSE 'sha256:a' END WHERE id='0000000000000001'`); err != nil {
					b.Fatal(err)
				}
			}
		})
	}

}

func TestFolderSizeOverflowRollsBackFile(t *testing.T) {
	db := migrated(t)
	root := boxRoot(t, db, insertBox(t, db, "full"))
	key := insertBlob(t, db, "sha256:large")
	sizeExec(t, db, `UPDATE blobs SET size=9223372036854775807 WHERE key=?`, key)
	insertFile(t, db, root, "max", key)
	one := insertBlob(t, db, "sha256:one")
	if err := tryInsertFile(db, root, "overflow", one); err == nil {
		t.Fatal("integer overflow accepted")
	}
	assertFolderSize(t, db, root, 9223372036854775807)
	var n int
	if err := db.QueryRow(`SELECT count(*) FROM files`).Scan(&n); err != nil || n != 1 {
		t.Fatalf("partial insertion: %d %v", n, err)
	}
}
