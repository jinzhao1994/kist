package errcodes

import (
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jinzhao1994/kist/internal/domain"
	"github.com/jinzhao1994/kist/internal/library"
	"github.com/jinzhao1994/kist/internal/lockfile"
)

// covered are the packages whose sentinels must all have codes. A package not
// listed here produces only internal errors as far as clients are concerned.
var covered = []string{
	"internal/library",
	"internal/oplog",
	"internal/domain",
	"internal/blob",
	"internal/query",
}

// Every exported sentinel in the packages above must have a code.
//
// This is the guard behind spec 2.9's claim that the code list is the
// contract. Without it a new sentinel silently becomes CodeInternal, which
// means a client sees "something went wrong" for a condition the library knew
// exactly — and nobody finds out until somebody hits it.
//
// It reads the source rather than using reflection because a sentinel is a
// package-level var, and there is no way to enumerate those at run time. This
// is the same technique internal/arch uses.
func TestEverySentinelHasACode(t *testing.T) {
	src, err := os.ReadFile("errcodes.go")
	if err != nil {
		t.Fatal(err)
	}
	body := string(src)

	for _, pkg := range covered {
		for _, name := range sentinelsIn(t, pkg) {
			// The qualified form is what appears in table, e.g.
			// "library.ErrNoSuchBox".
			short := pkg[strings.LastIndexByte(pkg, '/')+1:]
			if !strings.Contains(body, short+"."+name) {
				t.Errorf("%s.%s has no entry in errcodes.\n\n"+
					"Every sentinel a caller can match on needs a stable code (spec 2.9);\n"+
					"without one a client sees \"internal\" for a condition kist knew exactly.\n"+
					"Add a line to table, and a class to classes.", short, name)
			}
		}
	}
}

// sentinelsIn returns the names of exported error vars declared in a package.
func sentinelsIn(t *testing.T, pkg string) []string {
	t.Helper()
	dir := filepath.Join("..", "..", pkg)
	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, dir, func(fi os.FileInfo) bool {
		return !strings.HasSuffix(fi.Name(), "_test.go")
	}, 0)
	if err != nil {
		t.Fatalf("parse %s: %v", pkg, err)
	}

	var out []string
	for _, p := range pkgs {
		for _, f := range p.Files {
			for _, d := range f.Decls {
				gen, ok := d.(*ast.GenDecl)
				if !ok || gen.Tok != token.VAR {
					continue
				}
				for _, spec := range gen.Specs {
					vs, ok := spec.(*ast.ValueSpec)
					if !ok {
						continue
					}
					for _, n := range vs.Names {
						// Exported, and named like an error. Matching on the
						// name is what the convention in this repository
						// already guarantees; a var called Err… that is not an
						// error would be the surprise.
						if strings.HasPrefix(n.Name, "Err") && ast.IsExported(n.Name) {
							out = append(out, n.Name)
						}
					}
				}
			}
		}
	}
	return out
}

// Every code must have a class, or it silently becomes internal — which would
// give a "tag name taken" an HTTP 500 and no hint that anything was wrong.
func TestEveryCodeHasAClass(t *testing.T) {
	for _, e := range table {
		if _, ok := classes[e.code]; !ok {
			t.Errorf("code %q has no class", e.code)
		}
	}
	// The busy code is matched by type rather than through table, so it needs
	// checking separately.
	if _, ok := classes[CodeLibraryBusy]; !ok {
		t.Errorf("code %q has no class", CodeLibraryBusy)
	}
}

// Codes must be distinct: two conditions sharing one would merge two different
// problems into one thing a client cannot tell apart.
func TestCodesAreDistinct(t *testing.T) {
	seen := map[Code]bool{}
	for _, e := range table {
		if seen[e.code] {
			t.Errorf("code %q is used for two different sentinels", e.code)
		}
		seen[e.code] = true
	}
}

func TestOf(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want Code
	}{
		{"nil is no code", nil, ""},
		{"a plain sentinel", library.ErrNoSuchBox, CodeBoxNotFound},
		{"wrapped", fmt.Errorf("looking up %q: %w", "x", library.ErrNoSuchTag), CodeTagNotFound},
		{"from another package", domain.ErrNameTooLong, CodeNameTooLong},
		{"unclassified is internal", errors.New("the disk is on fire"), CodeInternal},
		// The lock's error carries a diagnostic and has no sentinel of its
		// own, so it is matched by type (spec 2.7).
		{"the write lock", &lockfile.BusyError{Path: "kist.lock"}, CodeLibraryBusy},
	}
	for _, c := range cases {
		if got := Of(c.err); got != c.want {
			t.Errorf("%s: Of = %q, want %q", c.name, got, c.want)
		}
	}
}

// The typed errors carry their sentinel through Unwrap, so a caller can still
// match on the sentinel while a client gets the fields.
func TestTypedErrorsKeepTheirCode(t *testing.T) {
	cases := []struct {
		err  error
		want Code
	}{
		{&library.BlockedUndoError{OpID: 47, BlockingOp: 51}, CodeUndoBlocked},
		{&library.AlreadyRevertedError{OpID: 47, RevertedBy: 48}, CodeAlreadyReverted},
		{&library.AmbiguousRefError{Ref: "云南", Kind: "box"}, CodeAmbiguousRef},
		{&library.NameTakenError{Name: "地点"}, CodeTagNameTaken},
	}
	for _, c := range cases {
		if got := Of(c.err); got != c.want {
			t.Errorf("%T: Of = %q, want %q", c.err, got, c.want)
		}
	}
}

// A conflict and a missing thing must not exit the same way, and a busy
// library must be distinguishable from a real failure so that cron can retry.
func TestClassOf(t *testing.T) {
	cases := []struct {
		err  error
		want Class
	}{
		{library.ErrNoSuchBox, ClassNotFound},
		{library.ErrTagNameTaken, ClassConflict},
		{domain.ErrNameTooLong, ClassInvalid},
		{library.ErrAmbiguousRef, ClassAmbiguous},
		{&lockfile.BusyError{}, ClassUnavailable},
		{library.ErrInvariantViolated, ClassDamaged},
		{errors.New("boom"), ClassInternal},
	}
	for _, c := range cases {
		if got := ClassOf(c.err); got != c.want {
			t.Errorf("%v: ClassOf = %v, want %v", c.err, got, c.want)
		}
	}
}
