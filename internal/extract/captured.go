package extract

import (
	"strconv"
	"strings"
	"time"
)

// CapturedRule and CapturedRuleVersion register the resolution below in
// file_extractions, under a reserved id no extractor may take (spec 1.7.6 ③).
//
// Sharing that table is the point: "which files still have a captured_at from
// an older rule" is the same question as "which files has exif v3 not seen",
// and it gets the same indexed answer instead of a second mechanism.
//
// Bump the version when the chain below would give a different answer.
const (
	CapturedRule        = "kist:captured"
	CapturedRuleVersion = 1
)

// Zone says how the timezone behind a capture instant was arrived at.
//
// It exists so the interface can be honest. A time assembled with an assumed
// zone is not the same claim as one the camera stamped, and showing both the
// same way is how a user comes to believe a guess (spec 1.5.2).
type Zone string

const (
	// ZoneExact means no zone was needed: the source was already an instant.
	ZoneExact Zone = "exact"
	// ZoneFromFile means the file carried an offset.
	ZoneFromFile Zone = "file"
	// ZoneFromLocation means a location was turned into a zone, by whichever
	// extractor produced geo:timezone.
	ZoneFromLocation Zone = "location"
	// ZoneAssumed means the library's configured zone was used because
	// nothing better was available.
	ZoneAssumed Zone = "assumed"
)

// Captured is the resolved capture time and where it came from.
type Captured struct {
	Unix  int64
	Known bool
	// From is the metadata key the wall clock or instant came from.
	From string
	// Zone is how the timezone was decided. Meaningless when Known is false.
	Zone Zone
}

// wallClockKeys are the sources of a wall-clock reading, best first.
//
// DateTimeOriginal is when the shutter opened. CreateDate is when the file was
// written, which for a camera is the same moment and for a scanner is the only
// one it knows. Video creation times have no producer in v0.1; the key is here
// so that adding one is adding an extractor, not editing the kernel.
var wallClockKeys = []string{
	"exif:DateTimeOriginal",
	"exif:CreateDate",
	"video:CreationTime",
}

// instantKeys are sources that are already unix seconds, best first.
//
// file:mtime is last overall because it answers a different question — when
// the file was last written — and for anything copied off a camera it is the
// moment of the copy. For a PDF, a scan or a voice memo it is the only time
// signal there is, and far better than none (spec 1.7).
var instantKeys = []string{"file:mtime"}

// offsetKeys are the tags that carry a UTC offset, best first.
//
// OffsetTimeOriginal belongs to DateTimeOriginal; plain OffsetTime belongs to
// ModifyDate and can disagree after an edit. Reading the wrong one is a real
// bug that shipped elsewhere (immich#15639), so the order is load-bearing
// rather than alphabetical.
var offsetKeys = []string{
	"exif:OffsetTimeOriginal",
	"exif:OffsetTimeDigitized",
	"exif:OffsetTime",
}

// locationKey is a zone name some future extractor derives from GPS. Nothing
// produces it in v0.1 (spec 8 keeps that behind a trigger condition), and the
// rule reads it anyway so that the extractor is all that has to be added.
const locationKey = "geo:timezone"

// exifTimeLayout is EXIF's own, colons and all.
const exifTimeLayout = "2006:01:02 15:04:05"

// ResolveCaptured works out when a file was captured.
//
// It is the kernel's conclusion, not an extractor's observation: extractors
// contribute the rows it reads, and none of them may write this answer
// directly (spec 1.5.2). That is a trust boundary — a plugin must not be able
// to set the column the user's ordering depends on.
//
// It is pure. The zone is a parameter rather than something read from the
// process, so the same file resolves the same way on every machine that opens
// the library (spec 4.7).
func ResolveCaptured(tags []KV, libraryZone *time.Location) Captured {
	if libraryZone == nil {
		libraryZone = time.UTC
	}
	byKey := make(map[string]string, len(tags))
	for _, kv := range tags {
		byKey[kv.Key] = kv.Value
	}

	// A wall clock plus a zone beats a bare instant: it is the moment the
	// photograph was taken rather than the moment its file was last touched.
	for _, key := range wallClockKeys {
		wall, ok := byKey[key]
		if !ok {
			continue
		}
		t, err := time.Parse(exifTimeLayout, wall)
		if err != nil || t.Year() <= 0 {
			// A camera with a dead battery writes something unparsable or
			// absurd. Falling through to the next source beats recording it,
			// and beats failing the file outright.
			continue
		}
		loc, zone := resolveZone(byKey, libraryZone)
		return Captured{
			Unix:  time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), t.Minute(), t.Second(), 0, loc).Unix(),
			Known: true,
			From:  key,
			Zone:  zone,
		}
	}

	for _, key := range instantKeys {
		raw, ok := byKey[key]
		if !ok {
			continue
		}
		sec, err := strconv.ParseInt(raw, 10, 64)
		if err != nil {
			continue
		}
		// No zone was involved and none is claimed. Saying "assumed" here
		// would be a hedge the interface would then repeat to the user about
		// a value that is exact.
		return Captured{Unix: sec, Known: true, From: key, Zone: ZoneExact}
	}

	return Captured{}
}

// resolveZone applies spec 1.5.2's zone chain.
func resolveZone(byKey map[string]string, libraryZone *time.Location) (*time.Location, Zone) {
	for _, key := range offsetKeys {
		if loc, ok := parseOffset(byKey[key]); ok {
			return loc, ZoneFromFile
		}
	}
	if name := byKey[locationKey]; name != "" {
		// A zone name that will not load falls back rather than failing the
		// file: Go's embedded tzdata can be absent on a stripped system, and
		// losing a capture time over that is the worse outcome.
		if loc, err := time.LoadLocation(name); err == nil {
			return loc, ZoneFromLocation
		}
	}
	return libraryZone, ZoneAssumed
}

// parseOffset reads an EXIF offset tag: "+08:00", "-08:00", or "   :  " when
// the camera wrote the tag but had nothing to put in it.
func parseOffset(s string) (*time.Location, bool) {
	s = strings.TrimSpace(s)
	if len(s) != 6 || (s[0] != '+' && s[0] != '-') || s[3] != ':' {
		return nil, false
	}
	hours, err1 := strconv.Atoi(s[1:3])
	minutes, err2 := strconv.Atoi(s[4:6])
	if err1 != nil || err2 != nil || hours > 14 || minutes > 59 {
		return nil, false
	}
	seconds := hours*3600 + minutes*60
	if s[0] == '-' {
		seconds = -seconds
	}
	return time.FixedZone(s, seconds), true
}
