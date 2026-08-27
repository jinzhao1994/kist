package extract

import (
	"strconv"
	"testing"
	"time"
)

var lib = time.FixedZone("library", 8*3600)

func kv(key, value string, typ Type) KV { return KV{Key: key, Value: value, Type: typ} }

func exifTime(v string) KV { return kv("exif:DateTimeOriginal", v, TypeText) }
func mtime(sec int64) KV   { return kv("file:mtime", strconv.FormatInt(sec, 10), TypeInt) }
func offset(v string) KV   { return kv("exif:OffsetTimeOriginal", v, TypeText) }

// spec 1.5.2: EXIF first, then a video's creation time, then file:mtime.
// EXIF wins even when mtime is present, because mtime is when the file was
// last written and that is a different question.
func TestWallClockPrefersExifOverMtime(t *testing.T) {
	got := ResolveCaptured([]KV{exifTime("2019:08:12 14:03:22"), mtime(1700000000)}, lib)
	if !got.Known {
		t.Fatal("nothing resolved")
	}
	if got.From != "exif:DateTimeOriginal" {
		t.Errorf("From = %q, want the EXIF tag", got.From)
	}
	if want := time.Date(2019, 8, 12, 14, 3, 22, 0, lib).Unix(); got.Unix != want {
		t.Errorf("Unix = %d, want %d", got.Unix, want)
	}
}

// file:mtime is already an instant, so this rung needs no zone at all — and
// claiming one would be a lie the UI would then repeat to the user.
func TestMtimeIsUsedDirectlyWithNoZone(t *testing.T) {
	got := ResolveCaptured([]KV{mtime(1700000000)}, lib)
	if !got.Known || got.Unix != 1700000000 {
		t.Fatalf("got %+v, want the mtime itself", got)
	}
	if got.From != "file:mtime" {
		t.Errorf("From = %q, want file:mtime", got.From)
	}
	if got.Zone != ZoneExact {
		t.Errorf("Zone = %q, want %q — an instant carries no zone question", got.Zone, ZoneExact)
	}
}

// spec 1.5.2's zone chain: the offset tag, then geo:timezone, then the
// library's own zone.
func TestZoneChain(t *testing.T) {
	wall := exifTime("2019:08:12 14:03:22")

	fromOffset := ResolveCaptured([]KV{wall, offset("-08:00")}, lib)
	if fromOffset.Zone != ZoneFromFile {
		t.Errorf("Zone = %q, want %q", fromOffset.Zone, ZoneFromFile)
	}
	if want := time.Date(2019, 8, 12, 14, 3, 22, 0, time.FixedZone("", -8*3600)).Unix(); fromOffset.Unix != want {
		t.Errorf("Unix = %d, want %d", fromOffset.Unix, want)
	}

	// geo:timezone has no producer in v0.1. The rule reads it anyway, so that
	// a GPS-lookup extractor can be added later without touching the kernel —
	// which is the whole shape of spec 1.5.2's trust boundary.
	fromGeo := ResolveCaptured([]KV{wall, kv("geo:timezone", "America/Los_Angeles", TypeText)}, lib)
	if fromGeo.Zone != ZoneFromLocation {
		t.Errorf("Zone = %q, want %q", fromGeo.Zone, ZoneFromLocation)
	}

	fallback := ResolveCaptured([]KV{wall}, lib)
	if fallback.Zone != ZoneAssumed {
		t.Errorf("Zone = %q, want %q", fallback.Zone, ZoneAssumed)
	}
	if want := time.Date(2019, 8, 12, 14, 3, 22, 0, lib).Unix(); fallback.Unix != want {
		t.Errorf("Unix = %d, want %d", fallback.Unix, want)
	}
}

// An explicit offset beats a location: the offset was written by the camera
// at the moment of the photograph, while a location has to be turned into an
// offset through a rule that changes with daylight saving.
func TestAnOffsetTagBeatsALocation(t *testing.T) {
	got := ResolveCaptured([]KV{
		exifTime("2019:08:12 14:03:22"),
		offset("-08:00"),
		kv("geo:timezone", "Asia/Tokyo", TypeText),
	}, lib)
	if got.Zone != ZoneFromFile {
		t.Errorf("Zone = %q, want the offset tag to win", got.Zone)
	}
}

// The fallback is the library's zone, never UTC. Immich changed exactly this
// from "the server's zone" to UTC and got a regression report (immich#24107):
// most of a personal archive was taken where its owner lives, so assuming UTC
// shifts all of it by hours and interleaves it wrongly with the photos that
// do carry a zone.
func TestTheAssumedZoneIsTheLibrarysNotUTC(t *testing.T) {
	got := ResolveCaptured([]KV{exifTime("2019:08:12 14:03:22")}, lib)
	if utc := time.Date(2019, 8, 12, 14, 3, 22, 0, time.UTC).Unix(); got.Unix == utc {
		t.Error("the unknown-zone fallback resolved to UTC")
	}
}

// The zone is a parameter, not something read from the process. Two libraries
// configured differently must disagree, or the same photo would sort
// differently depending on which machine imported it (spec 4.7).
func TestResolutionReadsNothingFromTheEnvironment(t *testing.T) {
	tags := []KV{exifTime("2019:08:12 14:03:22")}
	a := ResolveCaptured(tags, time.FixedZone("a", 3600))
	b := ResolveCaptured(tags, time.FixedZone("b", 7200))
	if a.Unix == b.Unix {
		t.Error("the library zone had no effect; something else is being read")
	}
	if a.Unix-b.Unix != 3600 {
		t.Errorf("the two zones differ by %ds, want 3600", a.Unix-b.Unix)
	}
}

// A camera with a dead battery writes a garbage time. Falling through to the
// next rung is better than recording it, and much better than failing the
// file: mtime is a worse answer than EXIF but a far better one than nothing.
func TestAGarbageExifTimeFallsThrough(t *testing.T) {
	for _, bad := range []string{"0000:00:00 00:00:00", "", "not a date", "2019-08-12 14:03:22"} {
		got := ResolveCaptured([]KV{exifTime(bad), mtime(1700000000)}, lib)
		if got.From != "file:mtime" {
			t.Errorf("%q: From = %q, want the fallthrough to mtime", bad, got.From)
		}
	}
}

// A zone name that will not load falls back to the library's rather than
// failing the file. Go's tzdata can be absent on a stripped system, and losing
// a photo's capture time over that would be a poor trade.
func TestAnUnloadableZoneFallsBackToTheLibrarys(t *testing.T) {
	got := ResolveCaptured([]KV{
		exifTime("2019:08:12 14:03:22"),
		kv("geo:timezone", "Mars/Olympus_Mons", TypeText),
	}, lib)
	if got.Zone != ZoneAssumed {
		t.Errorf("Zone = %q, want the library fallback", got.Zone)
	}
	if want := time.Date(2019, 8, 12, 14, 3, 22, 0, lib).Unix(); got.Unix != want {
		t.Errorf("Unix = %d, want the library zone applied", got.Unix)
	}
}

// Nothing to go on is not an error: it is captured_at being NULL, and spec
// 4.5.1 already says where such files sort.
func TestNoTimeAtAll(t *testing.T) {
	got := ResolveCaptured([]KV{kv("exif:Make", "Apple", TypeText)}, lib)
	if got.Known {
		t.Errorf("resolved a capture time out of nothing: %+v", got)
	}
}

// Sub-second precision is deliberately dropped (spec 1.5.2). Within one second
// the keyset tiebreaker already orders a burst by scan order, which is sorted
// file name, which is capture order.
func TestSubSecondIsIgnored(t *testing.T) {
	with := ResolveCaptured([]KV{
		exifTime("2019:08:12 14:03:22"),
		kv("exif:SubSecTimeOriginal", "146", TypeInt),
	}, lib)
	without := ResolveCaptured([]KV{exifTime("2019:08:12 14:03:22")}, lib)
	if with.Unix != without.Unix {
		t.Errorf("sub-second changed the answer: %d vs %d", with.Unix, without.Unix)
	}
}

// A scan of a 1960 photograph is exactly what an archive is for, and its
// instant is negative. Nothing in the chain may treat that as unset.
func TestANegativeInstantIsAnOrdinaryAnswer(t *testing.T) {
	got := ResolveCaptured([]KV{exifTime("1960:03:04 09:00:00")}, lib)
	if !got.Known {
		t.Fatal("a 1960 capture time was treated as unknown")
	}
	if got.Unix >= 0 {
		t.Errorf("Unix = %d, want a negative instant", got.Unix)
	}
}
