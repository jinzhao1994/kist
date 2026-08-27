// Package projection turns library state into the human-readable directory
// tree under boxes/.
//
// It is a pure function of the entities it is given: no database, no
// filesystem, no clock. That is what lets spec 3.4's cross-platform naming
// rules — half of which only Windows would ever exercise — be tested
// exhaustively on any machine.
package projection

import (
	"cmp"
	"slices"
	"strings"

	"github.com/jinzhao1994/kist/internal/domain"
)

// illegal maps the characters Windows refuses onto full-width counterparts.
//
// The substitution is applied on every platform, not only Windows. The
// alternative — sanitizing per platform — means moving a library between
// operating systems renames the entire tree on the next relink, and it means
// these rules are only ever executed by the Windows CI runner, which is where
// they are least likely to be right.
var illegal = strings.NewReplacer(
	`<`, "＜", `>`, "＞", `:`, "：", `"`, "＂",
	`/`, "／", `\`, "＼", `|`, "｜", `?`, "？", `*`, "＊",
)

// reserved are the DOS device names. Still special in Win32 after forty years:
// a file called CON cannot be created, and one called con.txt opens the
// console instead of the file.
var reserved = map[string]bool{
	"con": true, "prn": true, "aux": true, "nul": true,
	"com1": true, "com2": true, "com3": true, "com4": true, "com5": true,
	"com6": true, "com7": true, "com8": true, "com9": true,
	"lpt1": true, "lpt2": true, "lpt3": true, "lpt4": true, "lpt5": true,
	"lpt6": true, "lpt7": true, "lpt8": true, "lpt9": true,
}

// Sanitize makes one path component creatable on every platform kist supports.
//
// It must be idempotent: relink runs it over the whole library, and a
// non-idempotent rule would rename every file on every run — each rename being
// another opportunity for the Windows sharing violation described in 3.4.1.
func Sanitize(name string) string {
	var b strings.Builder
	for _, r := range illegal.Replace(name) {
		if r < 0x20 || r == 0x7f {
			b.WriteByte('_')
			continue
		}
		b.WriteRune(r)
	}
	// Windows silently drops trailing dots and spaces, so a name ending in one
	// would round-trip differently there than here.
	out := strings.TrimRight(b.String(), ". ")
	if out == "" {
		return "_"
	}
	stem, ext := domain.SplitName(out)
	if reserved[strings.ToLower(stem)] {
		return stem + "_" + ext
	}
	return out
}

// BoxDirName is a box's directory under boxes/: "<date>_<slug>".
//
// The date is start_at falling back to created_at, and its length carries the
// precision — "2019_" and "2019-08-12_" rather than a zero-padded form,
// because "2019-01-01_童年老照片" would tell a browsing human that something
// happened on New Year's Day (spec 3.4).
func BoxDirName(b domain.Box) string {
	date := b.StartAt.DirPrefix()
	if date == "" {
		date = b.CreatedAt.UTC().Format("2006-01-02")
	}
	return Sanitize(date + "_" + b.Name)
}

// Disambiguate appends a short id to a name that is not unique.
//
// The suffix goes before the extension so the file still opens with the right
// application, and it is the first eight hex digits of the UUID — enough to
// separate anything a person will ever have in one directory, short enough to
// still read the name.
func Disambiguate(name string, id domain.ID) string {
	stem, ext := domain.SplitName(name)
	return stem + "_" + strings.ReplaceAll(id.String(), "-", "")[:8] + ext
}

// Named is anything that needs a unique on-disk name.
type Named struct {
	ID   domain.ID
	Name string
}

// Resolve assigns a final, unique name to every item.
//
// The rule from spec 3.4.1: sort by id — UUIDv7, so creation order — and give
// the first claimant of a name the bare form, disambiguating the rest. That is
// what makes the projection a pure function of library state: one box changing
// its date can make another box's name change, and the answer must not depend
// on the order things are visited or on what the directory looked like before.
//
// Uniqueness is decided on the folded name, because two names differing only
// in case are one name on a case-insensitive filesystem — which is the default
// on both macOS and Windows.
func Resolve(items []Named) map[domain.ID]string {
	sorted := slices.Clone(items)
	slices.SortFunc(sorted, func(a, b Named) int { return cmp.Compare(a.ID.String(), b.ID.String()) })

	out := make(map[domain.ID]string, len(sorted))
	used := make(map[string]bool, len(sorted))
	for _, it := range sorted {
		name := Sanitize(it.Name)
		if !used[domain.Fold(name)] {
			used[domain.Fold(name)] = true
			out[it.ID] = name
			continue
		}
		// The short id is almost always enough; the loop is there so that a
		// prefix collision cannot produce two identical paths.
		candidate := Disambiguate(name, it.ID)
		for n := 2; used[domain.Fold(candidate)]; n++ {
			candidate = domain.NumberedName(Disambiguate(name, it.ID), n)
		}
		used[domain.Fold(candidate)] = true
		out[it.ID] = candidate
	}
	return out
}
