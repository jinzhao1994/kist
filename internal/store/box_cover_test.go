package store

import (
	"github.com/jinzhao1994/kist/internal/domain"
	"testing"
)

func TestBoxCoverUpgradePreservesExistingBox(t *testing.T) {
	db, err := OpenMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	p, err := migrationProvider(db)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = p.UpTo(t.Context(), 5); err != nil {
		t.Fatal(err)
	}
	id := insertBox(t, db, "before upgrade")
	if err = Migrate(db); err != nil {
		t.Fatal(err)
	}
	var boxID domain.BoxID
	if err := boxID.Scan(id); err != nil {
		t.Fatal(err)
	}
	b, found, err := DisplayBox(db, boxID)
	if err != nil || !found || b.Name != "before upgrade" || !b.CoverFileID.IsZero() {
		t.Fatalf("upgraded box: %+v %v", b, err)
	}
	if err = Migrate(db); err != nil {
		t.Fatal(err)
	}
}
