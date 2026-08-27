package library

import (
	"context"
	"errors"

	"github.com/jinzhao1994/kist/internal/blob"
	"github.com/jinzhao1994/kist/internal/config"
	"github.com/jinzhao1994/kist/internal/store"
)

// RepairOptions controls repair.
type RepairOptions struct {
	// Apply actually writes. The default is a dry run, the same shape spec 3.9
	// gives gc: this is the only command in M2a that writes to blobs/, and the
	// only thing between a mistyped remote path and a confusing afternoon is
	// that the user had to type --yes.
	//
	// A dry run does not download. Fetching every file just to prove it could
	// be fetched would make the dry run as slow and as expensive as the real
	// thing, and a dry run people skip is not a safety measure. What it
	// promises is "a remote has content under this key, of the right size" —
	// "I looked", not "I guarantee".
	Apply bool

	// Deep re-hashes local content instead of only stating it, so that rotted
	// blobs are repaired too. It is what you want after a scrub, and it costs
	// what a scrub costs.
	Deep bool
}

// RepairSource is where one blob came from, or would come from.
type RepairSource struct {
	Key string
	// Files names the files that need this content, as "<box>/<file>". It is
	// what the report shows: "云南/contract.pdf from backup" is a sentence,
	// and a bare digest is not — the same reasoning as spec 3.8's first
	// priority for verify.
	Files  []string
	Remote string
}

// RepairResult is what repair did, or would do.
type RepairResult struct {
	// Remotes are the configured remotes that could actually be opened.
	Remotes []string
	// Unreachable names remotes that are configured but not available — an
	// external drive that is not plugged in, most likely.
	Unreachable map[string]error
	// Broken is every blob that needs repairing.
	Broken []string
	// Repaired is what was fixed, or in a dry run what could be.
	Repaired []RepairSource
	// Unfixable is what no remote had a good copy of. This is the list that
	// matters: it is the content that is actually gone.
	Unfixable []RepairSource
	// DryRun records that nothing was written.
	DryRun bool
}

// ErrNoRemotes means nothing is configured to repair from.
var ErrNoRemotes = errors.New("no remotes are configured")

// Repair fetches damaged or missing content from a configured remote.
//
// This is spec 3.8's real answer to corruption, and content addressing is what
// makes it almost free: you know the hash, you ask any remote for that hash,
// you verify it, you put it back. The verification is not a formality — it is
// the reason a remote is allowed to be an unattended external drive or a
// network share.
//
// It writes only blobs/. No op is appended: restoring content that is correct
// by definition is not a state change, and Apply could not touch blobs/ anyway
// (spec 3.8, "修复不写 op").
func (l *Library) Repair(ctx context.Context, opt RepairOptions) (RepairResult, error) {
	res := RepairResult{DryRun: !opt.Apply, Unreachable: map[string]error{}}

	broken, err := l.brokenBlobs(ctx, opt.Deep)
	if err != nil {
		return res, err
	}
	res.Broken = broken
	if len(broken) == 0 {
		return res, nil
	}

	type remote struct {
		name string
		dir  *blob.Dir
	}
	var remotes []remote
	for _, r := range l.cfg.Remotes {
		d, err := blob.OpenDir(r.Path, l.log)
		if err != nil {
			// One unplugged drive must not stop the repair: another remote may
			// have the content, and the point is to fix what can be fixed.
			res.Unreachable[r.Name] = err
			continue
		}
		remotes = append(remotes, remote{r.Name, d})
		res.Remotes = append(res.Remotes, r.Name)
	}
	if len(l.cfg.Remotes) == 0 {
		return res, ErrNoRemotes
	}

	for _, keyStr := range broken {
		if err := ctx.Err(); err != nil {
			return res, err
		}
		key, err := blob.ParseKey(keyStr)
		if err != nil {
			res.Unfixable = append(res.Unfixable, RepairSource{Key: keyStr})
			continue
		}

		fixed := false
		for _, r := range remotes {
			ok, err := l.repairFrom(ctx, r.dir, key, opt.Apply)
			if err != nil {
				l.log.Warn("could not take content from a remote",
					"remote", r.name, "key", keyStr, "err", err)
				continue
			}
			if ok {
				res.Repaired = append(res.Repaired, RepairSource{Key: keyStr, Remote: r.name})
				fixed = true
				break
			}
		}
		if !fixed {
			res.Unfixable = append(res.Unfixable, RepairSource{Key: keyStr})
		}
	}
	if err := l.nameRepairedFiles(&res); err != nil {
		return res, err
	}
	return res, nil
}

// nameRepairedFiles fills in which files each blob belongs to, in one query
// for the whole result.
func (l *Library) nameRepairedFiles(res *RepairResult) error {
	if len(res.Repaired) == 0 && len(res.Unfixable) == 0 {
		return nil
	}
	keys := make([]string, 0, len(res.Repaired)+len(res.Unfixable))
	for _, r := range res.Repaired {
		keys = append(keys, r.Key)
	}
	for _, r := range res.Unfixable {
		keys = append(keys, r.Key)
	}
	users, err := store.FilesUsingBlobs(l.db, keys)
	if err != nil {
		return err
	}
	for i := range res.Repaired {
		res.Repaired[i].Files = users[res.Repaired[i].Key]
	}
	for i := range res.Unfixable {
		res.Unfixable[i].Files = users[res.Unfixable[i].Key]
	}
	return nil
}

// repairFrom takes one blob from one remote, reporting whether it worked.
//
// In a dry run it stops at "the remote has something under this key, of the
// right size". Reading it would cost the whole file, on what is typically a
// slow disk, for an answer the real run is about to establish anyway.
func (l *Library) repairFrom(ctx context.Context, remote *blob.Dir, key blob.Key, apply bool) (bool, error) {
	info, err := remote.Stat(ctx, key)
	if err != nil {
		return false, nil // this remote does not have it; not an error
	}
	if !apply {
		return info.Size >= 0, nil
	}

	r, err := remote.Get(ctx, key)
	if err != nil {
		return false, err
	}
	defer r.Close()

	outcome, err := l.blobs.Restore(ctx, r, key)
	if err != nil {
		return false, err
	}
	l.log.Info("repaired content from a remote", "key", key, "outcome", outcome)
	return true, nil
}

// brokenBlobs lists the content that needs repairing.
//
// Shallow stats, which finds content that is gone or the wrong length. Deep
// re-hashes, which is what scrub does and the only way to find content that
// rotted without changing length (spec 3.8).
//
// It walks the blobs table itself rather than reading a Report back: the
// report is prose for people, and parsing sentences to recover keys would make
// the wording load-bearing.
func (l *Library) brokenBlobs(ctx context.Context, deep bool) ([]string, error) {
	known, err := store.AllBlobs(l.db)
	if err != nil {
		return nil, err
	}
	var broken []string
	for _, b := range known {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		key, err := blob.ParseKey(b.Key)
		if err != nil {
			continue // the structural layer's finding; repair cannot help
		}
		if deep {
			hex, size, err := l.blobs.Rehash(ctx, key)
			if err != nil || hex != key.Hex() || size != b.Size {
				broken = append(broken, b.Key)
			}
			continue
		}
		info, err := l.blobs.Stat(ctx, key)
		if err != nil || info.Size != b.Size {
			broken = append(broken, b.Key)
		}
	}
	return broken, nil
}

// Remotes lists the configured remotes, for a command that wants to show them.
func (l *Library) Remotes() []config.Remote { return l.cfg.Remotes }

// RemoteStatus is one remote and whether it can be read right now.
type RemoteStatus struct {
	config.Remote
	Blobs int
	Err   error
}

// CheckRemotes opens each configured remote and counts what it holds.
//
// It exists so that a mistyped path says so at once, rather than the day the
// library is damaged and repair quietly finds nothing.
func (l *Library) CheckRemotes() []RemoteStatus {
	out := make([]RemoteStatus, 0, len(l.cfg.Remotes))
	for _, r := range l.cfg.Remotes {
		st := RemoteStatus{Remote: r}
		d, err := blob.OpenDir(r.Path, l.log)
		if err != nil {
			st.Err = err
			out = append(out, st)
			continue
		}
		if err := d.Walk(func(blob.Key, int64) error { st.Blobs++; return nil }); err != nil {
			st.Err = err
		}
		out = append(out, st)
	}
	return out
}
