package integrity

import (
	"context"
	"database/sql"
	"os"
	"strings"
	"testing"

	"github.com/jinzhao1994/kist/internal/blob"
)

// storeBlob puts content into a blob directory and records it in the database
// the way library's write path would.
func storeBlob(t *testing.T, db *sql.DB, d *blob.Dir, content string) blob.Key {
	t.Helper()
	res, err := d.Put(context.Background(), strings.NewReader(content))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO blobs (key, sha256, size) VALUES (?, ?, ?)`,
		string(res.Key), res.Key.Hex(), res.Size); err != nil {
		t.Fatal(err)
	}
	return res.Key
}

func blobDir(t *testing.T) *blob.Dir {
	t.Helper()
	d, err := blob.NewDir(t.TempDir(), nil)
	if err != nil {
		t.Fatal(err)
	}
	return d
}

func TestCheckBlobsIsSilentOnAHealthyLibrary(t *testing.T) {
	db, d := emptyLibrary(t), blobDir(t)
	storeBlob(t, db, d, "one")
	storeBlob(t, db, d, "two")

	r, err := CheckBlobs(t.Context(), db, d)
	if err != nil {
		t.Fatal(err)
	}
	if r.Total() != 0 {
		t.Errorf("a healthy library has %d findings", r.Total())
	}
	if r.Checked["blobs"] != 2 {
		t.Errorf("checked %d blobs, want 2", r.Checked["blobs"])
	}
}

// The finding spec 3.8 opens with: your content is gone, and you find out now
// rather than in five years when you need it.
func TestCheckBlobsFindsMissingContent(t *testing.T) {
	db, d := emptyLibrary(t), blobDir(t)
	k := storeBlob(t, db, d, "passport scan")

	p, _ := d.LocalPath(k)
	if err := os.Chmod(p, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(p); err != nil {
		t.Fatal(err)
	}

	r, err := CheckBlobs(t.Context(), db, d)
	if err != nil {
		t.Fatal(err)
	}
	if r.Count(BlobMissing) != 1 {
		t.Errorf("BlobMissing = %d, want 1", r.Count(BlobMissing))
	}
	if r.Worst() != SeverityError {
		t.Errorf("worst = %v, want SeverityError", r.Worst())
	}
}

// Content on disk that no row knows about is normal in v0.1 — nothing is ever
// collected (spec 3.9) — so it is reported at info level and must not make
// verify fail.
func TestCheckBlobsReportsOrphansWithoutFailing(t *testing.T) {
	db, d := emptyLibrary(t), blobDir(t)
	storeBlob(t, db, d, "known")
	if _, err := d.Put(t.Context(), strings.NewReader("written but never committed")); err != nil {
		t.Fatal(err)
	}

	r, err := CheckBlobs(t.Context(), db, d)
	if err != nil {
		t.Fatal(err)
	}
	if r.Count(BlobOrphan) != 1 {
		t.Errorf("BlobOrphan = %d, want 1", r.Count(BlobOrphan))
	}
	if r.Worst() != SeverityInfo {
		t.Errorf("worst = %v, want SeverityInfo — an orphan blob must not fail a verify", r.Worst())
	}
}

func TestCheckBlobsFindsTheWrongSize(t *testing.T) {
	db, d := emptyLibrary(t), blobDir(t)
	k := storeBlob(t, db, d, "hello")

	p, _ := d.LocalPath(k)
	if err := os.Chmod(p, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte("hello there"), 0o444); err != nil {
		t.Fatal(err)
	}

	r, err := CheckBlobs(t.Context(), db, d)
	if err != nil {
		t.Fatal(err)
	}
	if r.Count(BlobWrongSize) != 1 {
		t.Errorf("BlobWrongSize = %d, want 1", r.Count(BlobWrongSize))
	}
}

// verify must NOT notice content that changed without changing length. That is
// not a gap, it is the contract: verify stats, scrub reads (spec 3.8). If this
// test ever goes green the other way, somebody made verify slow.
func TestCheckBlobsDoesNotNoticeSameLengthCorruption(t *testing.T) {
	db, d := emptyLibrary(t), blobDir(t)
	k := storeBlob(t, db, d, "hello")

	p, _ := d.LocalPath(k)
	if err := os.Chmod(p, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte("world"), 0o444); err != nil {
		t.Fatal(err)
	}

	r, err := CheckBlobs(t.Context(), db, d)
	if err != nil {
		t.Fatal(err)
	}
	if r.Total() != 0 {
		t.Errorf("verify read the content: %d findings", r.Total())
	}
}

// Spec 3.8 puts naming the damaged files first among the things verify is for.
// A bare digest is not the sentence "your 2019 passport scan is damaged", and
// a report nobody can act on is a report that wasted the check.
func TestDamageReportsNameTheFilesNotJustTheHash(t *testing.T) {
	db, d := emptyLibrary(t), blobDir(t)
	k := storeBlob(t, db, d, "passport scan")

	box := []byte("0123456789abcdef")
	folder := []byte("fedcba9876543210")
	mustExec(t, db, `INSERT INTO boxes (id, name, name_fold, created_at, updated_at) VALUES (?, '云南', '云南', 't', 't')`, box)
	mustExec(t, db, `INSERT INTO folders (id, box_id, parent_id, name, name_fold, created_at, updated_at) VALUES (?, ?, NULL, '', '', 't', 't')`, folder, box)
	mustExec(t, db, `INSERT INTO files (id, box_id, folder_id, name, name_fold, blob_key, created_at, updated_at)
	                 VALUES (randomblob(16), ?, ?, 'passport.jpg', 'passport.jpg', ?, 't', 't')`, box, folder, string(k))

	p, _ := d.LocalPath(k)
	if err := os.Chmod(p, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(p); err != nil {
		t.Fatal(err)
	}

	r, err := CheckBlobs(t.Context(), db, d)
	if err != nil {
		t.Fatal(err)
	}
	var sb strings.Builder
	if err := r.Write(&sb); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(sb.String(), "云南/passport.jpg") {
		t.Errorf("the report does not name the file:\n%s", sb.String())
	}
	// The key is still there, shortened — you need it once you have decided to
	// do something, and not before.
	if !strings.Contains(sb.String(), k.Hex()[:8]) {
		t.Errorf("the report does not identify the content:\n%s", sb.String())
	}
	if strings.Contains(sb.String(), k.Hex()) {
		t.Errorf("the full digest took a whole line:\n%s", sb.String())
	}
}

func mustExec(t *testing.T, db *sql.DB, query string, args ...any) {
	t.Helper()
	if _, err := db.Exec(query, args...); err != nil {
		t.Fatal(err)
	}
}
