package extract

import (
	"fmt"
	"sort"
	"strings"
	"sync"
)

// registry holds the extractors this build knows about.
//
// The shape follows oplog's: a package-level default plus a constructor, so
// the ownership rules can be tested without registering anything globally.
type registry struct {
	mu    sync.RWMutex
	byID  map[string]Extractor
	owner map[string]string // namespace → extractor id
}

func newRegistry() *registry {
	return &registry{byID: map[string]Extractor{}, owner: map[string]string{}}
}

var defaultRegistry = newRegistry()

// Register adds an extractor, panicking on a conflict.
//
// It panics rather than returning an error because the callers are init
// functions: a build whose extractors disagree about who owns "exif" is
// broken in a way no library should be opened with.
func Register(e Extractor) {
	if err := defaultRegistry.register(e); err != nil {
		panic("extract: " + err.Error())
	}
}

func (r *registry) register(e Extractor) error {
	id := e.ID()
	namespaces := e.Namespaces()

	switch {
	case id == "":
		return fmt.Errorf("an extractor with no id")
	case id == UserSource:
		return fmt.Errorf("an extractor may not be called %q: that value marks rows a person wrote", UserSource)
	case len(namespaces) == 0:
		return fmt.Errorf("extractor %s owns no namespace, so nothing it produces could be placed", id)
	case e.Version() < 1:
		return fmt.Errorf("extractor %s has version %d; versions start at 1 and only go up", id, e.Version())
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	if _, dup := r.byID[id]; dup {
		return fmt.Errorf("two extractors are called %q", id)
	}
	for _, ns := range namespaces {
		switch {
		case ns == "":
			return fmt.Errorf("extractor %s declares an empty namespace", id)
		case strings.ContainsAny(ns, ":"):
			return fmt.Errorf("extractor %s declares namespace %q; the colon is the separator, not part of the name", id, ns)
		case ns == ReservedNamespace:
			return fmt.Errorf("extractor %s claims the %q namespace, which belongs to people (spec 1.7 rule ②)", id, ReservedNamespace)
		}
		if other, taken := r.owner[ns]; taken {
			return fmt.Errorf("extractor %s claims namespace %q, which %s already owns", id, ns, other)
		}
	}
	for _, ns := range namespaces {
		r.owner[ns] = id
	}
	r.byID[id] = e
	return nil
}

// Lookup finds an extractor by id.
func Lookup(id string) (Extractor, error) { return defaultRegistry.lookup(id) }

func (r *registry) lookup(id string) (Extractor, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	e, ok := r.byID[id]
	if !ok {
		return nil, fmt.Errorf("no extractor called %q in this build", id)
	}
	return e, nil
}

// All lists every registered extractor, ordered by id so a run over "all
// extractors" is reproducible.
func All() []Extractor { return defaultRegistry.all() }

func (r *registry) all() []Extractor {
	r.mu.RLock()
	defer r.mu.RUnlock()
	ids := make([]string, 0, len(r.byID))
	for id := range r.byID {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	out := make([]Extractor, len(ids))
	for i, id := range ids {
		out[i] = r.byID[id]
	}
	return out
}

// For lists the extractors that want a file of this media type, in the same
// stable order as All.
func For(mime string) []Extractor {
	var out []Extractor
	for _, e := range All() {
		if e.Wants(mime) {
			out = append(out, e)
		}
	}
	return out
}
