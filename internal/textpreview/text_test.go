package textpreview

import (
	"bytes"
	"strings"
	"testing"

	"golang.org/x/text/encoding/simplifiedchinese"
)

func TestReadText(t *testing.T) {
	gb, _ := simplifiedchinese.GB18030.NewEncoder().Bytes([]byte("中文日志"))
	for _, tc := range []struct {
		name           string
		data           []byte
		encoding, want string
		supported      bool
	}{
		{"unknown suffix", []byte("hello 世界\r\nnext"), "auto", "hello 世界\nnext", true},
		{"empty", nil, "auto", "", true},
		{"html is text", []byte("<script>alert(1)</script>"), "auto", "<script>alert(1)</script>", true},
		{"utf8 bom", []byte("\xef\xbb\xbfhello"), "auto", "hello", true},
		{"utf16le", []byte{0xff, 0xfe, 'a', 0, 0x2d, 0x4e}, "auto", "a中", true},
		{"utf16be", []byte{0xfe, 0xff, 0, 'a', 0x4e, 0x2d}, "auto", "a中", true},
		{"legacy explicit", gb, "gb18030", "中文日志", true},
		{"legacy needs choice", gb, "auto", "", false},
		{"binary", []byte{'a', 0, 'b'}, "auto", "", false},
		{"invalid", []byte{0xff, 'a'}, "auto", "", false},
		{"bad utf16", []byte{0xff, 0xfe, 0, 0xd8}, "auto", "", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Read(bytes.NewReader(tc.data), true, tc.encoding)
			if err != nil || got.Supported != tc.supported || got.Text != tc.want {
				t.Fatalf("%+v %v", got, err)
			}
		})
	}
}
func TestBounds(t *testing.T) {
	raw := strings.Repeat("x", DetailBytes-1) + "中文"
	got, err := Read(strings.NewReader(raw), true, "auto")
	if err != nil || !got.Supported || !got.Truncated || got.Text != raw[:DetailBytes-1] {
		t.Fatalf("utf8 boundary: supported=%v truncated=%v len=%d err=%v", got.Supported, got.Truncated, len(got.Text), err)
	}
	source := strings.NewReader(strings.Repeat("x", DetailBytes*3))
	got, err = Read(source, false, "auto")
	if err != nil || len(got.Text) != ThumbnailBytes || !got.Truncated || source.Len() != DetailBytes*3-ThumbnailBytes-4 {
		t.Fatalf("unbounded read: %+v %v", got, err)
	}
	got, _ = Read(strings.NewReader(strings.Repeat("line\n", 30)), false, "auto")
	if !got.Truncated || strings.Count(got.Text, "\n") != 19 {
		t.Fatalf("line cap: %+v", got)
	}
	got, _ = Read(strings.NewReader(strings.Repeat("x", ThumbnailBytes)), false, "auto")
	if got.Truncated {
		t.Fatal("exactly-sized file is not truncated")
	}
}
