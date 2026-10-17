package derive

import (
	"encoding/binary"
	"io"
	"os"
)

// quickTimePoster reads only bounded atom headers and mvhd from a QuickTime
// branded file. The corresponding MP4 fields are reserved, not poster times.
// Zero is also the ubiquitous unset value; it does not override sampling.
func quickTimePoster(path string, duration float64) (float64, bool) {
	f, err := os.Open(path)
	if err != nil {
		return 0, false
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return 0, false
	}
	type atom struct {
		start, end int64
		kind       string
	}
	read := func(offset, end int64) (atom, bool) {
		var h [16]byte
		if end-offset < 8 {
			return atom{}, false
		}
		if _, err := f.ReadAt(h[:8], offset); err != nil {
			return atom{}, false
		}
		size := uint64(binary.BigEndian.Uint32(h[:4]))
		header := int64(8)
		if size == 1 {
			if end-offset < 16 {
				return atom{}, false
			}
			if _, err := f.ReadAt(h[8:], offset+8); err != nil {
				return atom{}, false
			}
			size = binary.BigEndian.Uint64(h[8:])
			header = 16
		} else if size == 0 {
			size = uint64(end - offset)
		}
		if size < uint64(header) || size > uint64(end-offset) {
			return atom{}, false
		}
		return atom{offset + header, offset + int64(size), string(h[4:8])}, true
	}
	qt := false
	var movie atom
	for offset, count := int64(0), 0; offset < st.Size() && count < 4096; count++ {
		a, ok := read(offset, st.Size())
		if !ok {
			return 0, false
		}
		offset = a.end
		if a.kind == "ftyp" && a.end-a.start >= 8 {
			var brand [4]byte
			if _, err := f.ReadAt(brand[:], a.start); err == nil {
				qt = string(brand[:]) == "qt  "
			}
		}
		if a.kind == "moov" {
			movie = a
		}
	}
	if !qt || movie.end == 0 {
		return 0, false
	}
	for offset, count := movie.start, 0; offset < movie.end && count < 4096; count++ {
		a, ok := read(offset, movie.end)
		if !ok {
			return 0, false
		}
		offset = a.end
		if a.kind != "mvhd" {
			continue
		}
		var h [96]byte
		n, err := f.ReadAt(h[:min(int64(len(h)), a.end-a.start)], a.start)
		if err != nil && err != io.EOF {
			return 0, false
		}
		scaleOffset, posterOffset := 12, 80
		if h[0] == 1 {
			scaleOffset, posterOffset = 20, 92
		} else if h[0] != 0 {
			return 0, false
		}
		if n < posterOffset+4 {
			return 0, false
		}
		scale := binary.BigEndian.Uint32(h[scaleOffset:])
		ticks := int32(binary.BigEndian.Uint32(h[posterOffset:]))
		if scale == 0 || ticks <= 0 {
			return 0, false
		}
		seconds := float64(ticks) / float64(scale)
		return seconds, seconds < duration
	}
	return 0, false
}
