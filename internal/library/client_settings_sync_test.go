package library

import (
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/jinzhao1994/kist/internal/extract"
)

func TestClientInheritsNASTimezoneWithoutRewritingPending(t *testing.T) {
	nas, box := openWithBox(t)
	zone, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		t.Fatal(err)
	}
	nas.settings.Load().zone = zone
	f := importOneFile(t, nas, box, "photo.jpg", "original")
	extractWith(t, nas, &fakeExtractor{id: "exif-ish", version: 1, ns: []string{"exif"}, tags: []extract.KV{text("exif:DateTimeOriginal", "2019:08:12 14:03:22")}})
	extractWith(t, nas, &fakeExtractor{id: "geo-ish", version: 1, ns: []string{"geo"}, tags: []extract.KV{text("geo:timezone", "America/Los_Angeles")}})
	c := newSyncClient(t, nas)
	ctx := t.Context()
	settings, err := c.settings()
	if err != nil || settings.Timezone != "Asia/Shanghai" {
		t.Fatalf("timezone=%+v %v", settings, err)
	}
	if removed, err := c.RemoveMetadata(ctx, f.ID, "geo:timezone"); err != nil || !removed {
		t.Fatalf("remove=%t %v", removed, err)
	}
	before, err := c.queue.snapshot(ctx)
	if err != nil {
		t.Fatal(err)
	}
	caps, err := (&testSyncPeer{nas: nas}).Capabilities(ctx)
	if err != nil {
		t.Fatal(err)
	}
	caps.Timezone = "America/New_York"
	if err := c.AdoptSyncSettings(ctx, caps); err != nil {
		t.Fatal(err)
	}
	after, err := c.queue.snapshot(ctx)
	if err != nil || !reflect.DeepEqual(before, after) {
		t.Fatalf("settings rewrote pending: %v", err)
	}
	if _, err := c.Synchronize(ctx, &testSyncPeer{nas: nas}); err != nil {
		t.Fatal(err)
	}
	got, _, err := nas.File(ctx, f.ID)
	if err != nil {
		t.Fatal(err)
	}
	instant, known := got.CapturedAt.UnixNano()
	want := time.Date(2019, 8, 12, 14, 3, 22, 0, zone).UnixNano()
	if !known || instant != want {
		t.Fatalf("wrong wall-clock interpretation=%d want=%d", instant, want)
	}
	settings, err = c.settings()
	if err != nil || settings.Timezone != "Asia/Shanghai" {
		t.Fatalf("sync did not refresh settings=%+v %v", settings, err)
	}
}

func TestSyncRejectsMissingOrMachineLocalTimezone(t *testing.T) {
	nas := openTestLibrary(t)
	caps, err := (&testSyncPeer{nas: nas}).Capabilities(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	for _, zone := range []string{"", "Local", "Not/AZone"} {
		caps.Timezone = zone
		if err := ValidateSyncCapabilities(caps); !errors.Is(err, ErrSyncVersion) {
			t.Fatalf("zone %q accepted: %v", zone, err)
		}
	}
}
