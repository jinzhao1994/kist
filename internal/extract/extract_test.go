package extract

import (
	"strings"
	"testing"
)

var exifLike = fake{id: "exif", ns: []string{"exif", "gps"}}

// A declaration is a promise; Check is the enforcement. External-process
// plugins return untrusted input, and a built-in extractor's bug caught here
// is far cheaper than the same bug caught during a replay months later.
func TestCheckRejectsAKeyOutsideTheNamespace(t *testing.T) {
	err := Check(exifLike, []KV{
		{Key: "exif:Make", Value: "Apple", Type: TypeText},
		{Key: "user:有效期", Value: "2029", Type: TypeText},
	})
	if err == nil {
		t.Fatal("a key outside the extractor's namespaces was accepted")
	}
	if !strings.Contains(err.Error(), "user:有效期") {
		t.Errorf("the error does not name the offending key: %v", err)
	}
}

func TestCheckRejectsAKeyWithNoNamespace(t *testing.T) {
	if err := Check(exifLike, []KV{{Key: "Make", Value: "Apple", Type: TypeText}}); err == nil {
		t.Error("a bare key with no namespace was accepted")
	}
}

// The type tag is a closed set, because Apply routes on it (spec 1.7.5). An
// unrecognized tag has to be refused before the op is written, not during the
// replay — by then there is nothing to do but fail the rebuild.
func TestCheckRejectsAnUnknownType(t *testing.T) {
	if err := Check(exifLike, []KV{{Key: "exif:X", Value: "1", Type: "rational"}}); err == nil {
		t.Error("an unknown type tag was accepted")
	}
}

// The tag has to match the value, or the strconv in Apply fails during a
// replay — which is exactly the wrong time to find out.
func TestCheckRejectsAValueThatDoesNotParse(t *testing.T) {
	cases := map[string]KV{
		"int that is not one":  {Key: "exif:ISO", Value: "f/1.8", Type: TypeInt},
		"real that is not one": {Key: "exif:FNumber", Value: "one point eight", Type: TypeReal},
		"int that overflows":   {Key: "exif:Big", Value: "99999999999999999999", Type: TypeInt},
	}
	for what, kv := range cases {
		if err := Check(exifLike, []KV{kv}); err == nil {
			t.Errorf("%s was accepted: %+v", what, kv)
		}
	}
}

// int64 has to survive intact — that is the whole reason the value travels as
// a decimal string rather than a JSON number (spec 1.7.5). A nanosecond
// timestamp is past 2^53, where an IEEE double stops being exact.
func TestCheckAcceptsAFullWidthInt64(t *testing.T) {
	for _, v := range []string{"9223372036854775807", "-9223372036854775808", "1787869898922873000", "-315619200"} {
		if err := Check(exifLike, []KV{{Key: "exif:N", Value: v, Type: TypeInt}}); err != nil {
			t.Errorf("%s was rejected: %v", v, err)
		}
	}
}

// spec 1.7.2: binary blobs are not expanded. The rule is enforced at the
// contract too, not only inside the exif extractor, because every future
// extractor needs it and none of them should have to remember.
func TestCheckRejectsAnOversizedValue(t *testing.T) {
	if err := Check(exifLike, []KV{
		{Key: "exif:MakerNote", Value: strings.Repeat("x", MaxValueLen+1), Type: TypeText},
	}); err == nil {
		t.Error("a value past the size limit was accepted")
	}
}

func TestCheckRejectsInvalidUTF8(t *testing.T) {
	if err := Check(exifLike, []KV{{Key: "exif:X", Value: "\xff\xfe", Type: TypeText}}); err == nil {
		t.Error("a value that is not valid UTF-8 was accepted")
	}
}

// A key twice in one batch means the extractor disagrees with itself, and
// whichever row won would depend on iteration order.
func TestCheckRejectsADuplicateKey(t *testing.T) {
	err := Check(exifLike, []KV{
		{Key: "exif:Make", Value: "Apple", Type: TypeText},
		{Key: "exif:Make", Value: "APPLE", Type: TypeText},
	})
	if err == nil {
		t.Error("the same key twice in one batch was accepted")
	}
}

func TestCheckAcceptsAValidBatch(t *testing.T) {
	err := Check(exifLike, []KV{
		{Key: "exif:Make", Value: "Apple", Type: TypeText},
		{Key: "exif:ISO", Value: "400", Type: TypeInt},
		{Key: "gps:Latitude", Value: "37.93633333333333", Type: TypeReal},
	})
	if err != nil {
		t.Errorf("a valid batch was rejected: %v", err)
	}
}
