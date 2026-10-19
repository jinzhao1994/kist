// Package textpreview reads bounded, non-persistent text excerpts from files.
package textpreview

import (
	"bytes"
	"encoding/binary"
	"io"
	"strings"
	"unicode/utf16"
	"unicode/utf8"

	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/transform"
)

const ThumbnailBytes = 8 << 10
const DetailBytes = 1 << 20

// Preview never contains unbounded file content. Reason distinguishes binary
// data from text that cannot be decoded with the supported encodings.
type Preview struct {
	Supported bool   `json:"supported"`
	Text      string `json:"text"`
	Encoding  string `json:"encoding"`
	Truncated bool   `json:"truncated"`
	Reason    string `json:"reason,omitempty"`
}

// Read validates the entire bounded excerpt, not the filename or MIME hint.
func Read(r io.Reader, detail bool, encoding string) (Preview, error) {
	limit := ThumbnailBytes
	if detail {
		limit = DetailBytes
	}
	data, err := io.ReadAll(io.LimitReader(r, int64(limit+4)))
	if err != nil {
		return Preview{}, err
	}
	truncated := len(data) > limit
	lookahead := data
	if truncated {
		data = data[:limit]
	}
	out := Preview{Encoding: "UTF-8", Truncated: truncated}
	var text string
	switch {
	case bytes.HasPrefix(data, []byte{0xff, 0xfe}) || bytes.HasPrefix(data, []byte{0xfe, 0xff}):
		var order binary.ByteOrder = binary.LittleEndian
		out.Encoding = "UTF-16LE"
		if data[0] == 0xfe {
			order = binary.BigEndian
			out.Encoding = "UTF-16BE"
		}
		data = data[2:]
		if len(data)%2 != 0 {
			if !truncated {
				out.Reason = "encoding"
				return out, nil
			}
			data = data[:len(data)-1]
		}
		units := make([]uint16, len(data)/2)
		for i := range units {
			units[i] = order.Uint16(data[i*2:])
		}
		if truncated && len(units) > 0 && units[len(units)-1] >= 0xd800 && units[len(units)-1] <= 0xdbff {
			units = units[:len(units)-1]
		}
		text = string(utf16.Decode(units))
	case encoding == "gb18030":
		out.Encoding = "GB18030"
		var ok bool
		text, ok = decodeGB18030(data, truncated)
		if !ok {
			out.Reason = "encoding"
			return out, nil
		}
	default:
		bom := bytes.HasPrefix(data, []byte{0xef, 0xbb, 0xbf})
		data = bytes.TrimPrefix(data, []byte{0xef, 0xbb, 0xbf})
		lookahead = bytes.TrimPrefix(lookahead, []byte{0xef, 0xbb, 0xbf})
		original := data
		if truncated {
			// Drop only an incomplete final UTF-8 sequence, never invalid interior bytes.
			start := len(data) - 1
			for start > 0 && data[start]&0xc0 == 0x80 {
				start--
			}
			_, size := utf8.DecodeRune(lookahead[start:])
			if !utf8.FullRune(data[start:]) && size > 1 {
				data = data[:start]
			}
		}
		if !utf8.Valid(data) {
			// A BOM is authoritative. Only unmarked input may fall back to GBK/GB18030.
			var ok bool
			if !bom {
				text, ok = decodeGB18030(original, truncated)
			}
			if !ok {
				out.Reason = "encoding"
				return out, nil
			}
			out.Encoding = "GB18030"
		} else {
			text = string(data)
		}
	}
	for _, r := range text {
		if r == utf8.RuneError && out.Encoding != "UTF-8" {
			out.Reason = "encoding"
			return out, nil
		}
		if r < 32 && r != '\n' && r != '\r' && r != '\t' && r != '\f' {
			out.Reason = "binary"
			return out, nil
		}
		if r == 127 {
			out.Reason = "binary"
			return out, nil
		}
	}
	text = strings.ReplaceAll(strings.ReplaceAll(text, "\r\n", "\n"), "\r", "\n")
	if !detail {
		lines := strings.SplitN(text, "\n", 21)
		if len(lines) > 20 {
			text = strings.Join(lines[:20], "\n")
			out.Truncated = true
		}
	}
	out.Supported = true
	out.Text = text
	return out, nil
}

func decodeGB18030(data []byte, truncated bool) (string, bool) {
	decoder := simplifiedchinese.GB18030.NewDecoder()
	dst := make([]byte, len(data)*3+4)
	n, _, err := decoder.Transform(dst, data, !truncated)
	if err != nil && !(truncated && err == transform.ErrShortSrc) {
		return "", false
	}
	text := string(dst[:n])
	return text, !strings.ContainsRune(text, utf8.RuneError)
}
