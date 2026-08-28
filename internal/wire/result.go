package wire

// The types below are what a command that *did* something prints. They exist
// for the same reason the read envelopes do — an object can grow a field, an
// array or a bare number cannot — and they all answer the same question a
// script has after a write: what changed, and what is its id?

// BoxResult is one box that was created or edited.
type BoxResult struct {
	Box Box `json:"box"`
}

// TagResult is one tag that was created, renamed, moved or deleted.
type TagResult struct {
	Tag Tag `json:"tag"`
}

// TagChange is the outcome of attaching or detaching a tag.
//
// Changed counts what actually moved, not what was asked for: tagging a
// hundred files where ninety already carried the tag changes ten, and only
// those ten are in the op (spec 1.6). A caller that assumed "changed" meant
// "the number I passed" would report the wrong thing.
type TagChange struct {
	Tag     Tag `json:"tag"`
	Changed int `json:"changed"`
}

// TagChanges is one command's worth of tagging.
//
// A batch op carries one tag and many targets (spec 2.2.1), so tagging one box
// with three tags is three ops — and the CLI still has to print one document.
// Box is set when the targets were boxes.
type TagChanges struct {
	Box     *Box        `json:"box,omitempty"`
	Attach  bool        `json:"attach"`
	Changes []TagChange `json:"changes"`
}

// FileOutcome is what happened to one file during an import or an extraction.
type FileOutcome struct {
	Source string `json:"source,omitempty"`
	Path   string `json:"path,omitempty"`
	Name   string `json:"name"`
	Status string `json:"status"`
	// Outcome is what the content store did: stored, deduped, collided,
	// repaired (spec 3.3). Empty outside an import.
	Outcome string `json:"outcome,omitempty"`
	Error   string `json:"error,omitempty"`
}

// ImportResult is one import.
//
// OpID is zero when nothing was imported — an op with no files would be an
// undo that does nothing. Interrupted and Remaining describe a Ctrl-C: what
// was processed is committed, the rest was never touched (spec 3.7.1).
type ImportResult struct {
	// Source is the path on disk this batch came from.
	Source      string        `json:"source"`
	OpID        int64         `json:"op_id"`
	Imported    int           `json:"imported"`
	Skipped     int           `json:"skipped"`
	Failed      int           `json:"failed"`
	Interrupted bool          `json:"interrupted"`
	Remaining   int           `json:"remaining"`
	Files       []FileOutcome `json:"files"`
}

// ImportResults is one command's worth of importing. Each source directory is
// its own op, so that undoing one does not undo the others.
type ImportResults struct {
	Imports []ImportResult `json:"imports"`
}

// ExtractResult is one extraction pass. It produces several ops, one per
// batch, so that a long run is not one all-or-nothing transaction.
type ExtractResult struct {
	OpIDs       []int64       `json:"op_ids"`
	Extracted   int           `json:"extracted"`
	Skipped     int           `json:"skipped"`
	Failed      int           `json:"failed"`
	Interrupted bool          `json:"interrupted"`
	Files       []FileOutcome `json:"files"`
}

// UndoResult names both halves: the op that was undone and the op that undid
// it. The undo is itself an operation and can be undone in turn (spec 2.5).
type UndoResult struct {
	Undone   int64 `json:"undone"`
	UndoneBy int64 `json:"undone_by"`
}

// RelinkResult is a rebuild of boxes/.
//
// Missing is content that is not on local disk, which is not a failure — a
// library can legitimately have its blobs elsewhere.
type RelinkResult struct {
	Boxes   int `json:"boxes"`
	Created int `json:"created"`
	Removed int `json:"removed"`
	Missing int `json:"missing"`
	Failed  int `json:"failed"`
	// WasDirty says the stale-projection marker was set before this ran, and
	// is now cleared (spec 3.4.1).
	WasDirty bool `json:"was_dirty"`
}

// RebuildResult is a replay of the whole log into fresh tables.
//
// Confirmed is false when the user was asked and said no, in which case
// nothing happened and Ops is what would have been replayed.
type RebuildResult struct {
	Confirmed bool         `json:"confirmed"`
	Ops       int64        `json:"ops"`
	Relink    RelinkResult `json:"relink"`
}

// Finding is one kind of problem, with bounded evidence.
//
// Kind is a stable machine-readable name; Headline is prose that will be
// reworded, so match on Kind. Truncated is how many findings of this kind have
// no example listed.
type Finding struct {
	Kind      string   `json:"kind"`
	Severity  string   `json:"severity"`
	Headline  string   `json:"headline"`
	Count     int      `json:"count"`
	Examples  []string `json:"examples,omitempty"`
	Truncated int      `json:"truncated,omitempty"`
	Fix       string   `json:"fix,omitempty"`
}

// Report is what verify, scrub and repair found.
//
// Worst is what the exit code is built from. Checked says how much ground was
// covered, so that a clean report can say so rather than just "fine".
type Report struct {
	Findings []Finding      `json:"findings"`
	Worst    string         `json:"worst"`
	Checked  map[string]int `json:"checked,omitempty"`
}

// Content is one blob a repair looked at, named by the files that need it.
//
// "云南/contract.pdf" is a sentence and a bare digest is not, which is spec
// 3.8's first priority for verify — but the key is here too, because that is
// what a script would go fetch.
type Content struct {
	Key    string   `json:"key"`
	Files  []string `json:"files"`
	Remote string   `json:"remote,omitempty"`
}

// RepairResult is what repair did, or in a dry run would do.
//
// Unfixable is the list that matters: content no remote has a good copy of, so
// it is actually gone.
type RepairResult struct {
	DryRun      bool              `json:"dry_run"`
	Remotes     []string          `json:"remotes"`
	Unreachable map[string]string `json:"unreachable,omitempty"`
	Broken      []string          `json:"broken"`
	Repaired    []Content         `json:"repaired"`
	Unfixable   []Content         `json:"unfixable"`
}

// Remote is one configured second copy of the library.
//
// Blobs is how many it holds; Error says it could not be read at all, which is
// usually an external drive that is not plugged in.
type Remote struct {
	Name  string `json:"name"`
	Path  string `json:"path"`
	Blobs int    `json:"blobs"`
	Error string `json:"error,omitempty"`
}

// RemoteList is every configured remote. An empty list is the normal state and
// also the thing worth saying out loud: repair has nothing to repair from
// until there is a second copy (spec 3.8).
type RemoteList struct {
	Remotes []Remote `json:"remotes"`
}

// InitResult is a newly created library.
type InitResult struct {
	Path string `json:"path"`
	// LinkMode is how boxes/ points at blobs/ on this filesystem: hardlink,
	// symlink or copy, decided by probing (spec 3.5).
	LinkMode string `json:"link_mode"`
}

// Version identifies the binary. Dirty means it was built from a working tree
// with uncommitted changes, which is worth knowing when a bug report arrives.
type Version struct {
	Version  string `json:"version"`
	Commit   string `json:"commit"`
	Dirty    bool   `json:"dirty"`
	Go       string `json:"go"`
	Platform string `json:"platform"`
}
