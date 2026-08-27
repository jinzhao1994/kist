// Package integrity checks a library against itself and reports what it finds.
//
// It reads the database, reads blobs/ and reads boxes/. It never writes any of
// them, and it cannot append an op even by accident: it does not import oplog,
// and an arch test keeps it that way. That is the structural half of spec
// 3.8's "repair does not write an op" — a repair restores content that is
// already correct by definition, so there is no state change to record.
package integrity

import (
	"fmt"
	"io"
	"slices"
	"strings"
)

// examplesPerKind bounds how much evidence one kind of finding may print. A
// library with fifty thousand missing blobs has one problem, not fifty
// thousand, and a wall of output is how a report stops being read.
const examplesPerKind = 10

// Severity decides the exit code and the order things are printed in.
type Severity int

const (
	SeverityNone Severity = iota
	// SeverityInfo is "worth knowing, nothing is wrong". Orphan blobs live
	// here: v0.1 collects nothing (spec 3.9), so they are expected.
	SeverityInfo
	// SeverityWarn is "the projection is out of step". boxes/ is a projection,
	// not data — `kist relink` fixes any of these (spec 3.2).
	SeverityWarn
	// SeverityError is "content or the database is wrong". This is the one
	// that should wake somebody up.
	SeverityError
)

func (s Severity) String() string {
	switch s {
	case SeverityInfo:
		return "info"
	case SeverityWarn:
		return "warning"
	case SeverityError:
		return "error"
	}
	return "ok"
}

// Kind is what went wrong. Each one carries its own severity and its own
// sentence, because the sentence is the product: "your 2019 passport scan is
// damaged" is the whole point of this package (spec 3.8).
type Kind int

const (
	// ── blob layer ──────────────────────────────────────────────────────
	BlobMissing Kind = iota
	BlobWrongSize
	BlobCorrupt      // scrub only: the content no longer hashes to its own name
	BlobOrphan       // on disk, not in the blobs table
	BlobKeyMalformed // a row in the blobs table whose key is not a key

	// ── structure layer ─────────────────────────────────────────────────
	FileOrphanFolder  // files.folder_id names a folder that is gone
	FileBoxMismatch   // files.box_id disagrees with the folder's box
	FileUnknownBlob   // files.blob_key is not in the blobs table
	FolderUnreachable // not reachable from its box's root: gone parent, or a cycle
	FolderCrossBox    // a folder whose parent belongs to another box

	// ── projection layer ────────────────────────────────────────────────
	LinkMissing     // should be under boxes/, is not
	LinkStray       // under boxes/, should not be
	LinkWrongTarget // there, but not pointing at the blob it should

	// ── scrub ───────────────────────────────────────────────────────────
	ScrubIncomplete // the run stopped before it had read everything

	// ── deep ────────────────────────────────────────────────────────────
	ViewDrift // a materialized-view row that replaying the log does not produce

	// numKinds is the count, not a kind. It exists so the test that checks
	// every kind has a sentence extends itself when a kind is added — the
	// compiler cannot notice a new constant that never reached kindInfo.
	numKinds
)

var kindInfo = map[Kind]struct {
	severity Severity
	headline string
}{
	BlobMissing:       {SeverityError, "content is missing from blobs/"},
	BlobWrongSize:     {SeverityError, "content is the wrong size"},
	BlobCorrupt:       {SeverityError, "content no longer matches its hash"},
	BlobOrphan:        {SeverityInfo, "blobs/ holds content the database does not know about"},
	BlobKeyMalformed:  {SeverityError, "the blobs table holds a row whose key is not a valid key"},
	FileOrphanFolder:  {SeverityError, "a file is in a folder that does not exist"},
	FileBoxMismatch:   {SeverityError, "a file and its folder disagree about which box they are in"},
	FileUnknownBlob:   {SeverityError, "a file points at content the database does not know about"},
	FolderUnreachable: {SeverityError, "a folder cannot be reached from its box's root"},
	FolderCrossBox:    {SeverityError, "a folder's parent belongs to a different box"},
	LinkMissing:       {SeverityWarn, "boxes/ is missing something"},
	LinkStray:         {SeverityWarn, "boxes/ holds something that belongs to no file"},
	LinkWrongTarget:   {SeverityWarn, "a link under boxes/ points at the wrong content"},
	ScrubIncomplete:   {SeverityWarn, "the scrub did not finish"},
	ViewDrift:         {SeverityError, "a table does not match what replaying the log produces"},
}

// Severity reports how bad this kind of finding is.
func (k Kind) Severity() Severity { return kindInfo[k].severity }

// Headline is the sentence printed once per kind, above the examples.
func (k Kind) Headline() string { return kindInfo[k].headline }

// Finding is one thing that is wrong. Detail identifies it in whatever way is
// most useful to a human — a path, a blob key, a file id and name.
type Finding struct {
	Kind   Kind
	Detail string
	// Fix is the command that would repair this, when there is one. Printed
	// once per kind. Empty means "nothing in kist fixes this for you".
	Fix string
}

// Report accumulates findings. The zero value is ready to use.
//
// Nothing here stops at the first problem: a check that stops early checks
// nothing, and a user runs verify precisely when they suspect the library is
// broken (same reasoning as Relink, spec 3.4.1).
type Report struct {
	counts   map[Kind]int
	examples map[Kind][]string
	fixes    map[Kind]string
	// Checked counts what was looked at, so a clean report can say how much
	// ground it covered rather than just "fine".
	Checked map[string]int
}

func (r *Report) init() {
	if r.counts == nil {
		r.counts = map[Kind]int{}
		r.examples = map[Kind][]string{}
		r.fixes = map[Kind]string{}
	}
}

// Add records one finding.
func (r *Report) Add(f Finding) {
	r.init()
	r.counts[f.Kind]++
	if len(r.examples[f.Kind]) < examplesPerKind && f.Detail != "" {
		r.examples[f.Kind] = append(r.examples[f.Kind], f.Detail)
	}
	if f.Fix != "" {
		r.fixes[f.Kind] = f.Fix
	}
}

// Note records how much was checked, for the summary line.
func (r *Report) Note(what string, n int) {
	if r.Checked == nil {
		r.Checked = map[string]int{}
	}
	r.Checked[what] += n
}

// Merge folds another report in. Each layer builds its own and hands it up.
func (r *Report) Merge(o Report) {
	r.init()
	for k, n := range o.counts {
		r.counts[k] += n
		for _, d := range o.examples[k] {
			if len(r.examples[k]) >= examplesPerKind {
				break
			}
			r.examples[k] = append(r.examples[k], d)
		}
		if fix := o.fixes[k]; fix != "" {
			r.fixes[k] = fix
		}
	}
	for what, n := range o.Checked {
		r.Note(what, n)
	}
}

// Count returns how many findings of one kind there are.
func (r *Report) Count(k Kind) int { return r.counts[k] }

// Total is how many findings there are altogether.
func (r *Report) Total() int {
	n := 0
	for _, c := range r.counts {
		n += c
	}
	return n
}

// Worst is the highest severity present, and the thing the exit code is built
// from. An info-only report is a passing report: orphan blobs are a normal
// state for a v0.1 library, and a check that cries wolf gets removed from cron.
func (r *Report) Worst() Severity {
	worst := SeverityNone
	for k := range r.counts {
		if s := k.Severity(); s > worst {
			worst = s
		}
	}
	return worst
}

// Write renders the report for a person, worst first.
func (r *Report) Write(w io.Writer) error {
	var sb strings.Builder

	kinds := make([]Kind, 0, len(r.counts))
	for k := range r.counts {
		kinds = append(kinds, k)
	}
	slices.SortFunc(kinds, func(a, b Kind) int {
		if d := int(b.Severity()) - int(a.Severity()); d != 0 {
			return d
		}
		return int(a) - int(b)
	})

	for _, k := range kinds {
		n := r.counts[k]
		fmt.Fprintf(&sb, "%s: %s (%d)\n", k.Severity(), k.Headline(), n)
		for _, e := range r.examples[k] {
			fmt.Fprintf(&sb, "    %s\n", e)
		}
		if rest := n - len(r.examples[k]); rest > 0 {
			fmt.Fprintf(&sb, "    … and %d more\n", rest)
		}
		if fix := r.fixes[k]; fix != "" {
			fmt.Fprintf(&sb, "    fix: %s\n", fix)
		}
		sb.WriteString("\n")
	}

	if len(kinds) == 0 {
		sb.WriteString("no problems found\n")
	}
	if summary := r.summary(); summary != "" {
		sb.WriteString(summary + "\n")
	}
	_, err := io.WriteString(w, sb.String())
	return err
}

func (r *Report) summary() string {
	if len(r.Checked) == 0 {
		return ""
	}
	what := make([]string, 0, len(r.Checked))
	for k := range r.Checked {
		what = append(what, k)
	}
	slices.Sort(what)
	parts := make([]string, 0, len(what))
	for _, k := range what {
		parts = append(parts, fmt.Sprintf("%d %s", r.Checked[k], k))
	}
	return "checked " + strings.Join(parts, ", ")
}
