package extract

import (
	"context"
	"io"
	"strings"
	"testing"
)

type fake struct {
	id      string
	version int
	ns      []string
}

func (f fake) ID() string                                       { return f.id }
func (f fake) Version() int                                     { return max(f.version, 1) }
func (f fake) Namespaces() []string                             { return f.ns }
func (f fake) Wants(string) bool                                { return true }
func (f fake) Extract(context.Context, io.Reader) ([]KV, error) { return nil, nil }

// spec 1.7.6 ②: namespaces belong to one extractor. Without this, a user
// plugin could silently overwrite what the camera wrote into exif:Make — and
// "re-run exif replaces only exif's rows" would stop meaning anything.
func TestRegisterRejectsAnOverlappingNamespace(t *testing.T) {
	r := newRegistry()
	if err := r.register(fake{id: "exif", ns: []string{"exif", "gps"}}); err != nil {
		t.Fatal(err)
	}
	err := r.register(fake{id: "evil", ns: []string{"gps"}})
	if err == nil {
		t.Fatal("two extractors were allowed to claim the same namespace")
	}
	if !strings.Contains(err.Error(), "gps") || !strings.Contains(err.Error(), "exif") {
		t.Errorf("the error names neither the namespace nor its owner: %v", err)
	}
}

// user: is the half a person writes (spec 1.7 rule ②). An extractor writing
// there would wash away a hand-entered passport expiry — and re-scanning is
// never supposed to touch those rows.
func TestRegisterRejectsTheReservedNamespace(t *testing.T) {
	r := newRegistry()
	if err := r.register(fake{id: "greedy", ns: []string{"user"}}); err == nil {
		t.Error("an extractor was allowed to claim the user namespace")
	}
}

// Same rule from the other side: "user" is a reserved value of the source
// column, so no extractor may answer to it.
func TestRegisterRejectsTheReservedID(t *testing.T) {
	r := newRegistry()
	if err := r.register(fake{id: "user", ns: []string{"whatever"}}); err == nil {
		t.Error("an extractor was allowed to call itself user")
	}
}

// An extractor that owns nothing has nowhere to put what it produces, and
// "replace only this extractor's rows" would have no way to name them.
func TestRegisterRejectsAnExtractorThatOwnsNothing(t *testing.T) {
	r := newRegistry()
	if err := r.register(fake{id: "empty"}); err == nil {
		t.Error("an extractor with no namespace was accepted")
	}
}

// The colon separates namespace from tag. A namespace containing one would
// make "exif:Make" ambiguous about where the boundary is.
func TestRegisterRejectsAColonInANamespace(t *testing.T) {
	r := newRegistry()
	if err := r.register(fake{id: "odd", ns: []string{"a:b"}}); err == nil {
		t.Error("a namespace containing a colon was accepted")
	}
}

// Versions start at 1 and only go up (spec 1.7.6 ③/④). A zero version would
// be indistinguishable from "never extracted" in file_extractions.
func TestRegisterRejectsVersionZero(t *testing.T) {
	r := newRegistry()
	if err := r.register(zeroVersion{}); err == nil {
		t.Error("an extractor with version 0 was accepted")
	}
}

type zeroVersion struct{ fake }

func (zeroVersion) ID() string           { return "z" }
func (zeroVersion) Version() int         { return 0 }
func (zeroVersion) Namespaces() []string { return []string{"z"} }

func TestRegisterRejectsADuplicateID(t *testing.T) {
	r := newRegistry()
	if err := r.register(fake{id: "exif", ns: []string{"exif"}}); err != nil {
		t.Fatal(err)
	}
	if err := r.register(fake{id: "exif", ns: []string{"other"}}); err == nil {
		t.Error("two extractors with the same id were accepted")
	}
}

// The order has to be stable, or "run every extractor" produces a different
// op — and therefore a different library — on each run.
func TestAllIsOrdered(t *testing.T) {
	r := newRegistry()
	for _, id := range []string{"ocr", "exif", "ai"} {
		if err := r.register(fake{id: id, ns: []string{id}}); err != nil {
			t.Fatal(err)
		}
	}
	var got []string
	for _, e := range r.all() {
		got = append(got, e.ID())
	}
	if strings.Join(got, ",") != "ai,exif,ocr" {
		t.Errorf("All() = %v, want them sorted", got)
	}
}
