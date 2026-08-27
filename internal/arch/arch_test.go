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
