package exif

import (
	"bytes"
	"context"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/jinzhao1994/kist/internal/extract"
)

func read(t *testing.T, jpeg []byte, mime string) []extract.KV {
	t.Helper()
	kvs, err := New().Extract(context.Background(), bytes.NewReader(jpeg), mime)
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	return kvs
}

func index(kvs []extract.KV) map[string]extract.KV {
	out := map[string]extract.KV{}
	for _, kv := range kvs {
		out[kv.Key] = kv
	}
	return out
}

// The IFD path is dropped and the two namespaces this extractor owns are the
// only prefixes it emits (spec 1.7.6 ②). "exif:Make" is what a person would
// search for; "IFD0:Make" is an implementation detail of the file format.
func TestMapsTagsIntoTheOwnedNamespaces(t *testing.T) {
	got := index(read(t, sampleJPEG(), "image/jpeg"))

	for key, want := range map[string]string{
		"exif:Make":               "Apple",
		"exif:Model":              "iPhone 15",
		"exif:DateTimeOriginal":   "2019:08:12 14:03:22",
		"exif:OffsetTimeOriginal": "-08:00",
	} {
		if got[key].Value != want {
			t.Errorf("%s = %q, want %q", key, got[key].Value, want)
		}
	}
	if len(got) == 0 {
		t.Fatal("no tags at all")
	}
	for _, kv := range got {
		ns, _, ok := strings.Cut(kv.Key, ":")
		if !ok || (ns != "exif" && ns != "gps") {
			t.Errorf("key %q is outside exif: and gps:", kv.Key)
		}
	}
}

// GPS tags land in their own namespace with the redundant prefix removed:
// "gps:GPSLatitude" would say it twice.
func TestGPSGoesToItsOwnNamespace(t *testing.T) {
	got := index(read(t, sampleJPEG(), "image/jpeg"))
	if _, ok := got["gps:Latitude"]; !ok {
		t.Errorf("no gps:Latitude; got %v", keys(got))
	}
	if _, ok := got["exif:GPSLatitude"]; ok {
		t.Error("a GPS tag leaked into the exif namespace")
	}
}

// spec 1.7.2: binary blobs are not expanded. MakerNote is what that rule looks
// like in a real file — an Apple one is a bag of bytes with control characters
// and invalid UTF-8 in it.
func TestSkipsBinaryBlobs(t *testing.T) {
	kvs := read(t, sampleJPEG(), "image/jpeg")
	for _, kv := range kvs {
		if strings.Contains(kv.Key, "MakerNote") {
			t.Errorf("MakerNote was expanded: %s = %.40q", kv.Key, kv.Value)
		}
		for _, r := range kv.Value {
			if r < 0x20 && r != '\t' && r != '\n' {
				t.Errorf("%s holds a control character: %q", kv.Key, kv.Value)
			}
		}
		if len(kv.Value) > extract.MaxValueLen {
			t.Errorf("%s is %d bytes, past the limit", kv.Key, len(kv.Value))
		}
	}

	// The shape rules alone do not cover every MakerNote. Some vendors write
	// a readable header, and such a note is valid UTF-8, short, and free of
	// control characters — it passes every test above. It is excluded because
	// the EXIF standard types MakerNote as an opaque vendor payload, not
	// because we judged it uninteresting (spec 1.7.2).
	for _, kv := range read(t, asciiMakerNoteJPEG(), "image/jpeg") {
		if strings.Contains(kv.Key, "MakerNote") {
			t.Errorf("a printable MakerNote was expanded: %s = %q", kv.Key, kv.Value)
		}
	}
}

// Numbers are declared as numbers. Without this the whole type tag would be
// decorative: everything would be text, extract.Check would pass, and the
// eventual value_int column would have nothing to fill.
func TestNumericTagsAreDeclaredNumeric(t *testing.T) {
	got := index(read(t, sampleJPEG(), "image/jpeg"))
	if got["exif:ISO"].Type != extract.TypeInt {
		t.Errorf("exif:ISO is %q with value %q, want int", got["exif:ISO"].Type, got["exif:ISO"].Value)
	}
	if got["exif:Make"].Type != extract.TypeText {
		t.Errorf("exif:Make is %q, want text", got["exif:Make"].Type)
	}
}

// Everything produced has to survive the contract's own check, which is what
// library runs before any of it becomes an op.
func TestEveryValueSurvivesCheck(t *testing.T) {
	if err := extract.Check(New(), read(t, sampleJPEG(), "image/jpeg")); err != nil {
		t.Errorf("the extractor produced something Check refuses: %v", err)
	}
}

// A file with no EXIF is not a failure. Most of an archive is not
// photographs, and reporting a PDF as an error would make every import look
// broken (spec 3.7.1).
func TestAFileWithNoExifIsNotAnError(t *testing.T) {
	kvs, err := New().Extract(context.Background(), bytes.NewReader(jpegWithExif(nil)), "image/jpeg")
	if err != nil {
		t.Fatalf("a JPEG with no EXIF returned an error: %v", err)
	}
	if len(kvs) != 0 {
		t.Errorf("got %d tags from a file with no EXIF: %v", len(kvs), kvs)
	}
}

// The output has to be ordered, and the same file has to give the same answer
// twice. library decides what changed by comparing extractions; an unstable
// order would make an unchanged file look changed on every single run, and
// every run would write an op.
func TestOutputIsOrderedAndStable(t *testing.T) {
	a := read(t, sampleJPEG(), "image/jpeg")
	b := read(t, sampleJPEG(), "image/jpeg")
	if !reflect.DeepEqual(a, b) {
		t.Fatal("two extractions of the same bytes differ")
	}
	for i := 1; i < len(a); i++ {
		if a[i-1].Key >= a[i].Key {
			t.Errorf("output is not sorted: %q before %q", a[i-1].Key, a[i].Key)
		}
	}
}

// The one claim that needs a real container: EXIF is readable out of a HEIC in
// pure Go. iPhones shoot HEIC by default, so this is the line between M3 being
// complete for iPhone users and being a degraded path for them — ffmpeg is
// needed for the pixels (M5), not for the metadata (spec 5.3).
//
// The fixture is synthetic, not a photograph. See testdata/README.md.
func TestReadsExifFromAHEICContainer(t *testing.T) {
	f, err := os.Open("testdata/synthetic.heic")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	kvs, err := New().Extract(context.Background(), f, "image/heic")
	if err != nil {
		t.Fatalf("Extract from HEIC: %v", err)
	}
	got := index(kvs)
	for key, want := range map[string]string{
		"exif:Make":               "kist",
		"exif:DateTimeOriginal":   "2019:08:12 14:03:22",
		"exif:OffsetTimeOriginal": "-08:00",
	} {
		if got[key].Value != want {
			t.Errorf("%s = %q, want %q", key, got[key].Value, want)
		}
	}
	// The zone offset in particular: it is the first rung of the timezone
	// chain (spec 1.5.2), and an iPhone writes it. If it survives a HEIC, the
	// GPS lookup that would otherwise be needed stays optional.
	if _, ok := got["exif:OffsetTimeOriginal"]; !ok {
		t.Error("no OffsetTimeOriginal: the timezone chain loses its first rung")
	}
	if err := extract.Check(New(), kvs); err != nil {
		t.Errorf("HEIC extraction produced something Check refuses: %v", err)
	}
}

// Wants answers from the media type, and it has to include HEIC: iPhones
// shoot it by default, and its EXIF is readable in pure Go even though its
// pixels need ffmpeg. That distinction is what makes M3 complete for iPhone
// users rather than degraded (spec 5.3).
func TestWantsCoversHEIC(t *testing.T) {
	e := New()
	for _, mime := range []string{"image/jpeg", "image/heic", "image/heif", "image/tiff"} {
		if !e.Wants(mime) {
			t.Errorf("Wants(%q) = false", mime)
		}
	}
	for _, mime := range []string{"application/pdf", "text/plain", "video/mp4"} {
		if e.Wants(mime) {
			t.Errorf("Wants(%q) = true", mime)
		}
	}
}

// A media type this extractor never claimed is a caller error, not a silent
// empty result — the caller consulted Wants and ignored it.
func TestExtractRefusesAFormatItDoesNotRead(t *testing.T) {
	if _, err := New().Extract(context.Background(), bytes.NewReader(sampleJPEG()), "application/pdf"); err == nil {
		t.Error("a format outside Wants was accepted")
	}
}

func keys(m map[string]extract.KV) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
