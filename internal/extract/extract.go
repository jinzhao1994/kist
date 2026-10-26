// Package extract defines what an extractor is: what it owns, what it
// produces, and what a library may accept from it.
//
// It knows nothing about databases or the op log. An extractor's output is
// data; library is what turns that data into an op. That separation is what
// lets an extractor be arbitrarily impure — shell out to ffmpeg, call a model,
// read the network — without threatening the determinism a rebuild depends on
// (spec 1.7.1). An extractor that returned a different description every run
// would break nothing, because the description that was recorded is the one
// that gets replayed.
//
// It also keeps plugins away from the write path. captured_at is not in here
// (spec 1.5.2): extractors produce observations, and the kernel draws the
// conclusion.
package extract

import (
	"context"
	"fmt"
	"io"
	"strconv"
	"strings"
	"unicode/utf8"
)

// Type is what an extractor says a value is.
//
// It is declared, never inferred. Inferring "is this string a number" inside
// Apply would make a rebuild depend on which version of the inference ran,
// and invariant 3 forbids exactly that (spec 1.7.5).
type Type string

const (
	// TypeText is anything that is not a number: names, dates as the camera
	// wrote them, rationals like "1/60".
	TypeText Type = "text"
	// TypeInt fits int64 exactly. Carried as a decimal string so nothing in
	// the path — JSON included — ever rounds it through an IEEE double.
	TypeInt Type = "int"
	// TypeReal is an IEEE double.
	TypeReal Type = "real"
)

// KV is one observation.
//
// Value is what the extractor saw, verbatim: "f/1.8", "0100" with its leading
// zeros, "1.780" with its trailing one. The display needs that, and a typed
// field would normalize it away — which is why the type is a tag beside the
// string rather than a second, parsed copy of it (spec 1.7.5).
type KV struct {
	Key   string `json:"key"`
	Value string `json:"value"`
	Type  Type   `json:"type"`
}

// Row is one stored observation plus who wrote it.
//
// Tombstone marks a key a person removed. It is a row rather than the absence
// of one because the two must stay distinguishable: an absent key is one no
// extractor has produced yet, and the next run will produce it. A removed key
// must stay removed (spec 1.7.7).
//
// A tombstone carries no KV — its Source is always UserSource, which is what
// makes every extractor skip it under the ownership rule that already exists.
type Row struct {
	KV
	Source    string
	Tombstone bool
}

const (
	// UserSource is the reserved value of file_metadata.source for rows a
	// person wrote. No extractor may claim it, and no extraction may
	// overwrite such a row (spec 1.7 rule ②).
	UserSource = "user"
	// ImportSource marks the rows the import pipeline records: a file's own
	// size and modification time (spec 1.7).
	//
	// It is not an extractor and cannot be one. mtime has to be read from the
	// source file, and once a file is in blobs/ that reading is gone — the
	// stored copy's mtime is when it was written. "Capture it at import or
	// never" is a property of the fact, not a design choice.
	ImportSource = "file"
	// MaxValueLen bounds scalar metadata. OCR text has a separate long-text
	// allowance in checkValue; EXIF binary blocks remain excluded.
	MaxValueLen = 1024
)

// reserved are the namespaces and source ids no extractor may claim.
//
// "user" belongs to people and "file" to the importer. Both are enforced at
// registration rather than trusted, because the failure is silent: an
// extractor writing into either would overwrite something nothing else can
// reproduce.
var reserved = map[string]string{
	UserSource:   "people",
	ImportSource: "the import pipeline",
}

// Extractor reads a file and reports what it observed.
type Extractor interface {
	// ID goes in file_metadata.source and file_extractions.extractor. It is
	// how "re-run exif" knows to replace only exif's rows and leave OCR's
	// alone (spec 1.7.6 ①).
	ID() string
	// Version gates re-runs. Nothing re-extracts because it feels like it: a
	// nondeterministic extractor would write a different batch every time,
	// and the log would fill with churn (spec 1.7.6 ④).
	Version() int
	// Namespaces are the key prefixes this extractor owns. No two extractors
	// may claim the same one (spec 1.7.6 ②).
	Namespaces() []string
	// Wants reports whether this extractor has anything to say about a file
	// of this media type.
	Wants(mime string) bool
	// Extract reads the content. Finding nothing is not an error — a PDF, a
	// text file and a voice memo all have no EXIF, and that is an ordinary
	// answer rather than a failure (spec 3.7.1).
	//
	// It takes a ReadSeeker because container formats are not streams: EXIF in
	// a HEIC lives behind an offset table, and a decoder has to jump. The
	// source is always a blob on disk, so this costs nothing.
	//
	// mime is passed in rather than sniffed here so that every extractor sees
	// the same answer for a given file. Two extractors disagreeing about what
	// a file is would be a very quiet bug.
	Extract(ctx context.Context, r io.ReadSeeker, mime string) ([]KV, error)
}

// Check validates what an extractor produced before any of it becomes an op.
//
// It runs even for built-in extractors, and even though Namespaces is a
// declaration the extractor made about itself. A declaration is a promise;
// this is the enforcement. External-process plugins (spec 1.7.6) return
// untrusted input, and a built-in extractor's bug caught here is far cheaper
// than the same bug caught during a replay months later.
func Check(e Extractor, kvs []KV) error {
	owned := map[string]bool{}
	for _, ns := range e.Namespaces() {
		owned[ns] = true
	}
	seen := map[string]bool{}
	for _, kv := range kvs {
		ns, name, ok := strings.Cut(kv.Key, ":")
		switch {
		case !ok || ns == "" || name == "":
			return fmt.Errorf("extractor %s: key %q has no namespace", e.ID(), kv.Key)
		case !owned[ns]:
			return fmt.Errorf("extractor %s: key %q is outside its namespaces %v", e.ID(), kv.Key, e.Namespaces())
		case seen[kv.Key]:
			return fmt.Errorf("extractor %s: key %q appears twice", e.ID(), kv.Key)
		}
		seen[kv.Key] = true

		if err := checkValue(kv); err != nil {
			return fmt.Errorf("extractor %s: %s: %w", e.ID(), kv.Key, err)
		}
	}
	return nil
}

func checkValue(kv KV) error {
	switch kv.Type {
	case TypeText:
	case TypeInt:
		if _, err := strconv.ParseInt(kv.Value, 10, 64); err != nil {
			return fmt.Errorf("declared int but %q is not one", kv.Value)
		}
	case TypeReal:
		if _, err := strconv.ParseFloat(kv.Value, 64); err != nil {
			return fmt.Errorf("declared real but %q is not one", kv.Value)
		}
	default:
		return fmt.Errorf("unknown type %q", kv.Type)
	}
	limit := MaxValueLen
	if kv.Key == "desc:text" && kv.Type == TypeText {
		limit = 16 << 10
	}
	if kv.Key == "ocr:text" && kv.Type == TypeText {
		limit = 256 << 10
	}
	if len(kv.Value) > limit {
		return fmt.Errorf("value is %d bytes, over the %d-byte limit", len(kv.Value), limit)
	}
	if !utf8.ValidString(kv.Value) {
		return fmt.Errorf("value is not valid UTF-8")
	}
	return nil
}
