package exif

import (
	"bytes"
	"encoding/binary"
	"math"
)

// This file builds EXIF fixtures instead of committing photographs.
//
// The obvious alternative is a few real images in testdata/, and it was
// rejected twice over. Photographs that are convenient to find carry their own
// licenses — the one iPhone HEIC within reach is CC BY-SA 4.0, which does not
// belong in an Apache-2.0 repository — and a two-megabyte binary tells a
// reader nothing about what the test is asserting. Building the bytes here
// means every case can say exactly which tag it is about.
//
// What it deliberately does NOT cover is container parsing: this produces
// JPEG, and whether HEIC works is a claim about imagemeta rather than about
// our mapping. That claim gets its own test against a real file.

const (
	typeASCII    = 2
	typeShort    = 3
	typeLong     = 4
	typeRational = 5
	typeUndef    = 7
)

// entry is one IFD entry, already resolved to its bytes.
type entry struct {
	tag   uint16
	kind  uint16
	count uint32
	// data is the raw payload. Four bytes or fewer live inline; anything
	// longer is written out of line and referenced by offset.
	data []byte
}

func ascii(tag uint16, s string) entry {
	b := append([]byte(s), 0)
	return entry{tag: tag, kind: typeASCII, count: uint32(len(b)), data: b}
}

func short(tag uint16, v uint16) entry {
	b := make([]byte, 2)
	binary.LittleEndian.PutUint16(b, v)
	return entry{tag: tag, kind: typeShort, count: 1, data: b}
}

func long(tag uint16, v uint32) entry {
	b := make([]byte, 4)
	binary.LittleEndian.PutUint32(b, v)
	return entry{tag: tag, kind: typeLong, count: 1, data: b}
}

func rational(tag uint16, num, den uint32) entry {
	b := make([]byte, 8)
	binary.LittleEndian.PutUint32(b[0:], num)
	binary.LittleEndian.PutUint32(b[4:], den)
	return entry{tag: tag, kind: typeRational, count: 1, data: b}
}

// degrees renders a GPS coordinate the way EXIF does: three rationals.
func degrees(tag uint16, deg float64) entry {
	d := math.Floor(deg)
	m := math.Floor((deg - d) * 60)
	s := ((deg-d)*60 - m) * 60
	b := make([]byte, 24)
	put := func(off int, num, den uint32) {
		binary.LittleEndian.PutUint32(b[off:], num)
		binary.LittleEndian.PutUint32(b[off+4:], den)
	}
	put(0, uint32(d), 1)
	put(8, uint32(m), 1)
	put(16, uint32(s*1000), 1000)
	return entry{tag: tag, kind: typeRational, count: 3, data: b}
}

// undefined is the shape a MakerNote takes: a bag of bytes that is not text
// and must not be expanded into a metadata row (spec 1.7.2).
func undefined(tag uint16, b []byte) entry {
	return entry{tag: tag, kind: typeUndef, count: uint32(len(b)), data: b}
}

// ifd renders one IFD at base, appending out-of-line payloads after it.
//
// base is the entry's offset from the start of the TIFF header, which is what
// every offset inside EXIF is relative to.
func ifd(entries []entry, base uint32, next uint32) []byte {
	var body, tail bytes.Buffer
	// 2 bytes of count, 12 per entry, 4 for the next-IFD pointer.
	tailBase := base + 2 + uint32(len(entries))*12 + 4

	binary.Write(&body, binary.LittleEndian, uint16(len(entries)))
	for _, e := range entries {
		binary.Write(&body, binary.LittleEndian, e.tag)
		binary.Write(&body, binary.LittleEndian, e.kind)
		binary.Write(&body, binary.LittleEndian, e.count)
		if len(e.data) <= 4 {
			padded := make([]byte, 4)
			copy(padded, e.data)
			body.Write(padded)
			continue
		}
		binary.Write(&body, binary.LittleEndian, tailBase+uint32(tail.Len()))
		tail.Write(e.data)
		if tail.Len()%2 == 1 {
			tail.WriteByte(0) // IFD payloads are word-aligned
		}
	}
	binary.Write(&body, binary.LittleEndian, next)
	body.Write(tail.Bytes())
	return body.Bytes()
}

// exifBlob assembles a TIFF header plus IFD0, an Exif sub-IFD and a GPS IFD.
func exifBlob(ifd0, sub, gps []entry) []byte {
	const headerLen = 8
	// IFD0 gains the two pointer entries, so its size is known only after
	// they are counted.
	withPointers := append(append([]entry{}, ifd0...),
		long(0x8769, 0), // ExifIFDPointer, patched below
		long(0x8825, 0), // GPSInfoIFDPointer, patched below
	)
	ifd0Len := uint32(len(ifd(withPointers, headerLen, 0)))
	subOff := headerLen + ifd0Len
	subLen := uint32(len(ifd(sub, subOff, 0)))
	gpsOff := subOff + subLen

	withPointers[len(withPointers)-2] = long(0x8769, subOff)
	withPointers[len(withPointers)-1] = long(0x8825, gpsOff)

	var out bytes.Buffer
	out.WriteString("II")
	binary.Write(&out, binary.LittleEndian, uint16(42))
	binary.Write(&out, binary.LittleEndian, uint32(headerLen))
	out.Write(ifd(withPointers, headerLen, 0))
	out.Write(ifd(sub, subOff, 0))
	out.Write(ifd(gps, gpsOff, 0))
	return out.Bytes()
}

// jpegWithExif wraps an EXIF blob in the smallest JPEG that carries one.
func jpegWithExif(blob []byte) []byte {
	var out bytes.Buffer
	out.Write([]byte{0xFF, 0xD8}) // SOI
	if blob != nil {
		payload := append([]byte("Exif\x00\x00"), blob...)
		out.Write([]byte{0xFF, 0xE1})
		binary.Write(&out, binary.BigEndian, uint16(len(payload)+2))
		out.Write(payload)
	}
	out.Write([]byte{0xFF, 0xD9}) // EOI
	return out.Bytes()
}

// sampleJPEG is the fixture most cases use: an ordinary phone photograph with
// a capture time, a zone offset, GPS, a rational, and a MakerNote full of
// bytes that must never become a row.
func sampleJPEG() []byte {
	return jpegWithExif(exifBlob(
		[]entry{
			ascii(0x010F, "Apple"),     // Make
			ascii(0x0110, "iPhone 15"), // Model
			short(0x0112, 1),           // Orientation
		},
		[]entry{
			ascii(0x9003, "2019:08:12 14:03:22"), // DateTimeOriginal
			ascii(0x9011, "-08:00"),              // OffsetTimeOriginal
			short(0x8827, 400),                   // ISO
			rational(0x829D, 1244236, 699009),    // FNumber
			rational(0x829A, 1, 60),              // ExposureTime
			undefined(0x927C, makerNoteBytes()),  // MakerNote
		},
		[]entry{
			ascii(0x0001, "N"),          // GPSLatitudeRef
			degrees(0x0002, 37.936333),  // GPSLatitude
			ascii(0x0003, "W"),          // GPSLongitudeRef
			degrees(0x0004, 122.365288), // GPSLongitude
		},
	))
}

// asciiMakerNoteJPEG carries a MakerNote made entirely of printable ASCII.
//
// It exists because the U+FFFD rule alone does not cover this: some vendors
// write a readable header, and such a note survives every shape test intact.
// Without this fixture, isVendorBlob would be code nothing proves is needed —
// and a mutation removing it would go green.
func asciiMakerNoteJPEG() []byte {
	return jpegWithExif(exifBlob(
		[]entry{ascii(0x010F, "Nikon")},
		[]entry{
			ascii(0x9003, "2019:08:12 14:03:22"),
			undefined(0x927C, []byte("Nikon MakerNote v2, quality FINE, sharpening AUTO")),
		},
		nil,
	))
}

// makerNoteBytes is deliberately not text: control bytes and an invalid UTF-8
// sequence, which is what a real Apple MakerNote looks like to anything that
// tries to read it as a string.
func makerNoteBytes() []byte {
	b := []byte("Apple iOS\x00\x00\x01MM\x00")
	for i := 0; i < 64; i++ {
		b = append(b, byte(i*7%256))
	}
	return append(b, 0xFF, 0xFE, 0xFD)
}
