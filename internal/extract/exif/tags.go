package exif

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/bep/imagemeta"

	"github.com/jinzhao1994/kist/internal/extract"
)

// mapValue turns one imagemeta value into a row, or reports that it should not
// become one.
//
// spec 1.7.2 draws the line by shape, not by usefulness: expand every scalar,
// printable, bounded tag; expand no binary blob. "Is this a scalar string" has
// an objective answer that will still be right in five years, while "will
// anyone ever search this" is a guess that ages badly.
//
// The type is declared here rather than inferred later, because inference
// inside Apply would make a rebuild depend on which version of the inference
// ran (spec 1.7.5, invariant 3).
func mapValue(v any) (string, extract.Type, bool) {
	switch x := v.(type) {
	case string:
		return textOrSkip(x)
	case bool:
		return strconv.FormatBool(x), extract.TypeText, true
	case int, int8, int16, int32, int64:
		return fmt.Sprintf("%d", x), extract.TypeInt, true
	case uint, uint8, uint16, uint32, uint64:
		return fmt.Sprintf("%d", x), extract.TypeInt, true
	case float32:
		return strconv.FormatFloat(float64(x), 'f', -1, 32), extract.TypeReal, true
	case float64:
		return strconv.FormatFloat(x, 'f', -1, 64), extract.TypeReal, true
	case []byte:
		// A MakerNote, an ICC profile, an embedded thumbnail. Never a row.
		return "", "", false
	default:
		// Rationals and anything else render through fmt. FNumber arrives as
		// "1244236/699009" and is kept exactly that way: spec 1.7.5 says the
		// value is what the extractor saw, and a decimal approximation would
		// be a second, lossy copy of a tag nothing queries in v0.1.
		return textOrSkip(fmt.Sprint(x))
	}
}

// textOrSkip accepts a string only if it is one a person could read.
//
// The U+FFFD test is the one that does the work, and it was not obvious.
// The plan assumed the size and control-byte bounds would exclude binary
// tags; measurement said otherwise. imagemeta hands a MakerNote over as a
// Go string, having already replaced every invalid byte with U+FFFD and
// dropped the control bytes — so it arrives 128 bytes long, valid UTF-8, with
// zero control characters. Shaped exactly like text.
//
// A replacement character means the decoder substituted for bytes it could
// not represent. Whatever is left is therefore not what the extractor saw,
// and spec 1.7.5 requires the value to be verbatim. Refusing on that basis is
// still a rule about shape, not about usefulness (spec 1.7.2) — it just reads
// the evidence that the shape test was applied too late.
func textOrSkip(s string) (string, extract.Type, bool) {
	s = strings.TrimRight(s, "\x00")
	if s == "" || len(s) > extract.MaxValueLen || !utf8.ValidString(s) {
		return "", "", false
	}
	if strings.ContainsRune(s, utf8.RuneError) {
		return "", "", false
	}
	for _, r := range s {
		if r < 0x20 && r != '\t' && r != '\n' {
			return "", "", false
		}
	}
	return s, extract.TypeText, true
}

// isVendorBlob reports whether a tag is one the EXIF standard defines as an
// opaque vendor payload.
//
// MakerNote (0x927C) is type UNDEFINED by definition: its contents are a
// camera maker's private structure, and every vendor's is different. This is
// not "we guessed nobody would search it" — it is the format saying the field
// is not a scalar (spec 1.7.2).
//
// It is matched by name because imagemeta does not surface the EXIF type, and
// names every variant MakerNoteApple, MakerNoteCanon, and so on. The U+FFFD
// test above already catches most of them; this catches the ones whose bytes
// happen to survive as legal text.
func isVendorBlob(tag string) bool {
	return strings.HasPrefix(tag, "MakerNote")
}

// isNoMetadata reports whether err just means "this file carries none".
//
// imagemeta has no sentinel for it, so this matches on the message. That is
// unpleasant and it is bounded: the alternative is treating "a PNG with no
// EXIF" as a per-file failure, and most of an archive is not photographs.
// A message change upstream turns a silent skip into a loud failure, which is
// the right direction for the mistake to fall.
func isNoMetadata(err error) bool {
	if err == nil || errors.Is(err, imagemeta.ErrStopWalking) {
		return true
	}
	msg := strings.ToLower(err.Error())
	for _, s := range []string{"no exif", "not found", "no metadata", "unsupported"} {
		if strings.Contains(msg, s) {
			return true
		}
	}
	return false
}
