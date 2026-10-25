package library

import "testing"

func TestCatalogRevisionObservesOtherHandlesAndSync(t *testing.T) {
	nas, box := openWithBox(t)
	client := newSyncClient(t, nas)
	before, err := nas.CatalogRevision(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	other, err := Open(nas.layout.Root)
	if err != nil {
		t.Fatal(err)
	}
	defer other.Close()
	if _, err := other.UpdateBox(t.Context(), box, BoxFields{Name: "elsewhere"}); err != nil {
		t.Fatal(err)
	}
	after, err := nas.CatalogRevision(t.Context())
	if err != nil || before == after {
		t.Fatalf("other writer: %s %s %v", before, after, err)
	}
	initial, err := client.CatalogRevision(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := client.Synchronize(t.Context(), &testSyncPeer{nas: nas}); err != nil {
		t.Fatal(err)
	}
	synced, err := client.CatalogRevision(t.Context())
	if err != nil || synced == initial {
		t.Fatalf("download: %s %s %v", initial, synced, err)
	}
	second, err := OpenClient(t.Context(), client.root)
	if err != nil {
		t.Fatal(err)
	}
	defer second.Close()
	if _, err := second.UpdateBox(t.Context(), box, BoxFields{Name: "local elsewhere"}); err != nil {
		t.Fatal(err)
	}
	edited, err := client.CatalogRevision(t.Context())
	if err != nil || edited == synced {
		t.Fatalf("local writer: %s %s %v", synced, edited, err)
	}
}
