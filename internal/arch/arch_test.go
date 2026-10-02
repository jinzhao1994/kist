// Package arch holds no code. It exists for one test: the one that checks the
// dependency direction the whole design rests on.
package arch

import (
	"go/build"
	"path/filepath"
	"strings"
	"testing"
)

const modulePath = "github.com/jinzhao1994/kist/"

// imports returns the non-test imports of a package, relative to the module.
//
// Test imports are deliberately excluded: a test may legitimately reach for
// anything (library's tests open a real database). The invariants below are
// about what the shipped code can touch.
func imports(t *testing.T, dir string) []string {
	t.Helper()
	pkg, err := build.ImportDir(filepath.Join("..", "..", dir), 0)
	if err != nil {
		t.Fatalf("read %s: %v", dir, err)
	}
	var out []string
	for _, imp := range pkg.Imports {
		if strings.HasPrefix(imp, modulePath) {
			out = append(out, strings.TrimPrefix(imp, modulePath))
		}
	}
	return out
}

func forbid(t *testing.T, dir string, banned []string, why string) {
	t.Helper()
	for _, imp := range imports(t, dir) {
		for _, b := range banned {
			if imp == b {
				t.Errorf("%s imports %s.\n\n%s", dir, imp, why)
			}
		}
	}
}

// TestDomainDependsOnNothing keeps the bottom of the stack at the bottom.
func TestDomainDependsOnNothing(t *testing.T) {
	if got := imports(t, "internal/domain"); len(got) != 0 {
		t.Errorf("internal/domain imports %v; it must depend on nothing else in this repository", got)
	}
}

// TestBlobDependsOnNothing keeps the other half of the authoritative pair at
// the bottom.
//
// blobs/ is authoritative alongside ops (invariant 1). verify --deep, scrub and
// repair all rest on it being checkable without the database, which is only
// true while the blob layer cannot read the database either. store does not
// import blob, so blob -> store would compile.
func TestBlobDependsOnNothing(t *testing.T) {
	if got := imports(t, "internal/blob"); len(got) != 0 {
		t.Errorf("internal/blob imports %v.\n\n"+
			"blobs/ is authoritative on its own (invariant 1). verify --deep, scrub and\n"+
			"repair all rest on it being checkable without the database; a blob layer\n"+
			"that could read the views would no longer be able to contradict them.", got)
	}
}

// TestJournalDependsOnNothing keeps the other authority at the bottom too.
//
// ops/ and blobs/ are the two authorities (invariant 1). verify --deep now
// checks the database against the log, which is only a second opinion while
// the log layer cannot read the database: a journal that could would be
// checking the views against themselves, which is exactly the weakness the
// old VACUUM INTO shape had (spec 3.8).
func TestJournalDependsOnNothing(t *testing.T) {
	if got := imports(t, "internal/journal"); len(got) != 0 {
		t.Errorf("internal/journal imports %v.\n\n"+
			"ops/ is one of the two authorities (invariant 1), and verify --deep\n"+
			"compares it against the database. A log layer that could read the\n"+
			"database could no longer contradict it.", got)
	}
}

// TestOplogCannotReachTheDatabase is the structural half of invariant 3.
//
// Apply is required to be a pure function of (tx, payload). That is enforced
// by the shape of oplog.Tx — a small interface with no clock, no id generator
// and no arbitrary SQL. The moment oplog can import store, a handler can take
// a *sql.Tx and the guarantee is gone; nothing in review would necessarily
// catch it, because the code would still compile and the tests would still
// pass until someone ran `kist rebuild` on a real library.
func TestOplogCannotReachTheDatabase(t *testing.T) {
	forbid(t, "internal/oplog", []string{"internal/store", "internal/library", "internal/blob"},
		"Apply must be a pure function of (tx, payload) — invariant 3. oplog.Tx is\n"+
			"the only way a handler may touch a library, and it is small on purpose.")
}

// TestStoreDoesNotDependOnOplog keeps the seam where it belongs.
//
// store satisfies oplog.Tx structurally, and library is the only place that
// knows both. If store imported oplog the two would be welded together and the
// interface would stop being a boundary.
func TestStoreDoesNotDependOnOplog(t *testing.T) {
	forbid(t, "internal/store", []string{"internal/oplog", "internal/library"},
		"store satisfies oplog.Tx structurally; library is the seam between them.")
}

// TestProjectionKnowsOnlyDomain keeps the naming rules testable.
//
// Everything cross-platform about boxes/ is decided in projection, and half of
// those rules are only ever exercised on Windows. Keeping the package free of
// the database and the op log is what lets them be tested exhaustively
// anywhere, without a library and without a disk — which is the only reason
// the Windows-specific half gets executed on every run.
func TestProjectionKnowsOnlyDomain(t *testing.T) {
	forbid(t, "internal/projection", []string{"internal/store", "internal/oplog", "internal/library"},
		"projection is a pure function of the entities it is given. Reaching for\n"+
			"the database would make spec 3.4's naming rules untestable without one.")
}

// TestCommandsCannotWriteDirectly is invariant 2.
//
// library is the only package that may write to a library, because every write
// there is one transaction that appends an op and applies it. A command that
// reached store directly could change the materialized views without leaving a
// trace in the log — which would make "every operation can be undone" false in
// a way no test elsewhere would notice.
func TestCommandsCannotWriteDirectly(t *testing.T) {
	forbid(t, "cmd/kist", []string{"internal/store", "internal/oplog"},
		"library is the only write entry point — invariant 2. Every write is one\n"+
			"transaction that appends an op and applies it; reaching store directly\n"+
			"would change state without leaving a trace in the log.")
}

// TestIntegrityCannotWrite is the structural half of spec 3.8's "repair does
// not write an op".
//
// A repair restores content that is correct by definition — you fetched it by
// its hash and verified it — so nothing about the library's state changed and
// there is nothing to record. More to the point, Apply is a pure function that
// cannot touch blobs/ (invariant 3), so such an op would have to do nothing on
// replay, and both `kist rebuild` and `verify --deep` would need a special case
// for it forever.
//
// Not importing oplog is how that stays true without anyone having to remember
// it.
func TestIntegrityCannotWrite(t *testing.T) {
	forbid(t, "internal/integrity", []string{"internal/oplog", "internal/library"},
		"integrity reads and reports; it never writes. Restoring content that is\n"+
			"correct by definition is not a state change, and Apply cannot touch\n"+
			"blobs/ anyway — such an op would do nothing on replay, forever.")
}

// TestQueryIsOnlyTheContract keeps the data access layer's contract free of
// its implementations.
//
// cmd builds queries and cmd may not import store (invariant 2), so the Query
// type cannot live there. Keeping this package down at domain is also what
// makes spec 4.1's "no SQL in the interface" something a test can hold you to,
// rather than a habit.
func TestQueryIsOnlyTheContract(t *testing.T) {
	for _, imp := range imports(t, "internal/query") {
		if imp != "internal/domain" {
			t.Errorf("internal/query imports %s.\n\n"+
				"The contract may depend on domain and nothing else. It is built by cmd,\n"+
				"which cannot reach store (invariant 2), and translated by store — if it\n"+
				"could see either, the contract and one implementation would be welded\n"+
				"together.", imp)
		}
	}
}

// TestLockfileIsSelfContained keeps the write lock free of kist.
//
// It is a wrapper over one OS primitive (spec 2.7), and the moment it can see
// library or store it stops being that: it grows a notion of what a write is,
// and the question "does this code path take the lock" becomes answerable in
// two places instead of one.
func TestLockfileIsSelfContained(t *testing.T) {
	for _, imp := range imports(t, "internal/lockfile") {
		t.Errorf("internal/lockfile imports %s.\n\n"+
			"The lock wraps flock and LockFileEx and nothing else. Whether a given\n"+
			"code path takes it is library's decision, made at library's call sites.", imp)
	}
}

// TestExtractKnowsOnlyDomain keeps extraction testable without a library, and
// keeps plugins away from the write path.
//
// An extractor is a function from bytes to observations; library is what turns
// those observations into an op. If this package could see store or oplog, an
// extractor could write its own rows — and "extraction results are recorded in
// the log, so an extractor may be impure" would stop being true by
// construction (spec 1.7.1, 1.7.6).
//
// It is also why captured_at does not live here: the kernel draws that
// conclusion, and a plugin must not be able to set the sort key (spec 1.5.2).
func TestExtractKnowsOnlyDomain(t *testing.T) {
	for _, imp := range imports(t, "internal/extract") {
		if imp != "internal/domain" {
			t.Errorf("internal/extract imports %s.\n\n"+
				"Extractors produce data; library turns it into an op. Reaching for\n"+
				"store or oplog would let an extractor write directly, and the\n"+
				"determinism a rebuild depends on would stop being structural.", imp)
		}
	}
}

// TestWireIsOnlyTheShape keeps the JSON contract reachable from both surfaces.
//
// cmd and api both render it (spec 2.8), and cmd may not import store
// (invariant 2). If wire could see store or library it would only be usable
// from one side, and the CLI and the API would end up with two definitions of
// a box that drift apart at exactly the moment somebody adds a field.
//
// It is the same rule as query and oplog, for the same reason.
func TestWireIsOnlyTheShape(t *testing.T) {
	for _, imp := range imports(t, "internal/wire") {
		if imp != "internal/domain" {
			t.Errorf("internal/wire imports %s.\n\n"+
				"The JSON shape may depend on domain and nothing else. It is rendered by\n"+
				"cmd, which cannot reach store (invariant 2), and by api — if it could see\n"+
				"either, one surface would own the contract and the other would copy it.", imp)
		}
	}
}

// TestErrCodesDoesNotLeakIntoTheKernel keeps the wire format at the boundary.
//
// errcodes maps errors to stable codes (spec 2.9), so it necessarily sees
// every package that produces one — it sits above library, not below. The
// invariant worth holding is the other direction: nothing in the kernel may
// import it. The moment library or oplog can name a code, the code list stops
// being a boundary concern and starts being something a handler has an opinion
// about, and the errors that carry data would grow a dependency on the wire
// format they are meant to be independent of.
func TestErrCodesDoesNotLeakIntoTheKernel(t *testing.T) {
	for _, dir := range []string{
		"internal/domain", "internal/blob", "internal/oplog",
		"internal/store", "internal/library", "internal/query",
		"internal/projection", "internal/integrity", "internal/extract",
	} {
		forbid(t, dir, []string{"internal/errcodes"},
			"Codes belong to the wire format, which belongs to the boundary (spec 2.9).\n"+
				"An error that needs to carry data does so with its own type and fields;\n"+
				"errcodes reads it from above with errors.As.")
	}
}

func TestAPICannotWriteDirectly(t *testing.T) {
	forbid(t, "internal/api", []string{"internal/store", "internal/oplog"},
		"The HTTP API must write through library, preserving journal ordering and request deduplication.")
}
