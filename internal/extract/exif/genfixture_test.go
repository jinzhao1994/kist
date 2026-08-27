package exif

import (
	"bytes"
	"encoding/binary"
	"os"
	"testing"
)

// TestGenerateHEICFixture is a tool, not a test.
//
// It splices the EXIF block fixture_test.go builds into a JPEG that has real
// image data, which is what libheif needs before it will carry metadata into a
// HEIC. Skipped unless KIST_FIXTURE_DIR is set; see testdata/README.md for the
// three commands that regenerate the fixture.
//
// A test that always skips is usually worthless, and this one is not a test —
// it is a generator kept next to the thing it generates, so that the fixture
// can be reproduced rather than merely trusted.
func TestGenerateHEICFixture(t *testing.T) {
	dir := os.Getenv("KIST_FIXTURE_DIR")
	if dir == "" {
		t.Skip("not generating fixtures")
	}
	src, err := os.ReadFile(dir + "/e.jpg")
	if err != nil {
		t.Fatal(err)
	}
	blob := exifBlob(
		[]entry{ascii(0x010F, "kist"), ascii(0x0110, "synthetic fixture"), short(0x0112, 1)},
		[]entry{
			ascii(0x9003, "2019:08:12 14:03:22"),
			ascii(0x9011, "-08:00"),
			short(0x8827, 400),
			rational(0x829D, 1244236, 699009),
		},
		[]entry{ascii(0x0001, "N"), degrees(0x0002, 37.936333)},
	)
	payload := append([]byte("Exif\x00\x00"), blob...)
	var out bytes.Buffer
	out.Write(src[:2]) // SOI
	out.Write([]byte{0xFF, 0xE1})
	binary.Write(&out, binary.BigEndian, uint16(len(payload)+2))
	out.Write(payload)
	out.Write(src[2:])
	if err := os.WriteFile(dir+"/withexif.jpg", out.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Logf("wrote %d bytes", out.Len())
}
