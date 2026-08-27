package oplog

import (
	"errors"
	"strings"
	"testing"
)

type fakeHandler struct{ applied int }

func (h *fakeHandler) Apply(Tx, []byte) error                 { h.applied++; return nil }
func (h *fakeHandler) Inverse([]byte) (Action, []byte, error) { return "noop", nil, nil }
func (h *fakeHandler) Targets([]byte) (Targets, error)        { return Targets{}, nil }

func TestLookupFindsRegisteredHandlers(t *testing.T) {
	// Every action the build knows about must resolve. A typo in a const or a
	// handler nobody registered would otherwise only surface at runtime, on a
	// user's library.
	for _, a := range AllActions() {
		if _, err := Lookup(a); err != nil {
			t.Errorf("Lookup(%q): %v", a, err)
		}
	}
}

func TestLookupRejectsUnknownAction(t *testing.T) {
	_, err := Lookup(Action("teleport_box"))
	if err == nil {
		t.Fatal("Lookup of an unknown action = nil error, want a rejection")
	}
	if !errors.Is(err, ErrUnknownAction) {
		t.Errorf("error = %v, want it to wrap ErrUnknownAction", err)
	}
	// spec 2.2: the message has to tell the user what actually happened. An
	// append-only history table must be forgiving of the future.
	if !strings.Contains(err.Error(), "newer") {
		t.Errorf("error = %q, want it to say a newer kist wrote this", err)
	}
}

func TestRegisterRejectsDuplicates(t *testing.T) {
	// Registered here rather than reusing a real action, so this test says the
	// same thing no matter which handlers the build happens to contain.
	const probe = Action("registry_duplicate_probe")
	Register(probe, &fakeHandler{})

	defer func() {
		if recover() == nil {
			t.Error("registering the same action twice should panic at init time")
		}
	}()
	Register(probe, &fakeHandler{})
}

func TestAllActionsTracksWhatIsRegistered(t *testing.T) {
	// AllActions is what TestLookupFindsRegisteredHandlers walks, so it has to
	// describe reality rather than intent — a constant nobody wrote a handler
	// for must not appear in it.
	registered := map[Action]bool{}
	for _, a := range AllActions() {
		if registered[a] {
			t.Errorf("AllActions lists %q twice", a)
		}
		registered[a] = true
	}
	for a := range registry {
		if !registered[a] {
			t.Errorf("action %q has a handler but is missing from AllActions", a)
		}
	}
}

func TestPayloadRoundTrip(t *testing.T) {
	type sample struct {
		Name  string `json:"name"`
		Count int    `json:"count"`
	}
	raw, err := Marshal(sample{Name: "婚礼", Count: 3})
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	// spec 2.2.1: TEXT json, not jsonb — the audit log has to stay readable by
	// a human with a SQL prompt.
	if !strings.Contains(string(raw), `"name"`) {
		t.Errorf("payload is not readable JSON: %s", raw)
	}

	got, err := Payload[sample](raw)
	if err != nil {
		t.Fatalf("Payload: %v", err)
	}
	if got.Name != "婚礼" || got.Count != 3 {
		t.Errorf("round trip gave %+v", got)
	}
}

func TestPayloadRejectsUnknownFields(t *testing.T) {
	// A payload written by a newer kist carrying a field we do not understand
	// must not be silently half-applied.
	type sample struct {
		Name string `json:"name"`
	}
	if _, err := Payload[sample]([]byte(`{"name":"x","unexpected":1}`)); err == nil {
		t.Error("Payload accepted an unknown field")
	}
}

func TestSourcePropagatesUpdatedAt(t *testing.T) {
	// spec 1.5.1 ②: updated_at records human intent, not system activity.
	// Getting this wrong turns ten years of archive into "modified just now"
	// the first time an extractor is re-run.
	if !SourceUser.PropagatesUpdatedAt() {
		t.Error("user ops must propagate updated_at")
	}
	for _, s := range []Source{SourceExtractor, SourceMaintenance} {
		if s.PropagatesUpdatedAt() {
			t.Errorf("%s must not propagate updated_at", s)
		}
	}
}

// TestEveryDeclaredActionHasAHandler closes the loop the other way round.
//
// AllActions describes what is registered; this checks that nothing was
// declared as a constant and then left without an implementation. A constant
// with no handler reads as supported everywhere it appears, and only fails on
// a user's library.
func TestEveryDeclaredActionHasAHandler(t *testing.T) {
	declared := []Action{
		ActionCreateBox, ActionDeleteBox, ActionUpdateBox,
		ActionCreateTag, ActionDeleteTag, ActionRenameTag, ActionMoveTag,
		ActionTagBoxes, ActionUntagBoxes,
		ActionImportFiles, ActionDeleteFiles,
		ActionTagFiles, ActionUntagFiles,
	}
	for _, a := range declared {
		if _, err := Lookup(a); err != nil {
			t.Errorf("action %q is declared but has no handler: %v", a, err)
		}
	}
	if len(AllActions()) < len(declared) {
		t.Errorf("AllActions has %d entries but %d actions are declared", len(AllActions()), len(declared))
	}
}
