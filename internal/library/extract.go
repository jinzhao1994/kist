package library

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"slices"
	"sort"
	"time"

	"github.com/jinzhao1994/kist/internal/domain"
	"github.com/jinzhao1994/kist/internal/extract"
	_ "github.com/jinzhao1994/kist/internal/extract/builtin"
	"github.com/jinzhao1994/kist/internal/oplog"
	"github.com/jinzhao1994/kist/internal/store"
)

// DefaultExtractBatch is how many files go into one op.
//
// Bounded rather than one op for the whole library: spec 3.7.1 rejected
// whole-batch rollback for imports because a run interrupted at 99% loses
// everything and holds a write transaction for hours. Re-extracting a hundred
// thousand files has exactly that shape.
const DefaultExtractBatch = 500

// ExtractOptions configures one run.
type ExtractOptions struct {
	// Extractor limits the run to one. Empty means every registered one.
	Extractor string
	// Force re-extracts files already recorded at the current version. It
	// exists for "I changed something the version does not describe", which
	// is a developer's situation, not a user's.
	Force bool
	// BatchSize overrides DefaultExtractBatch.
	BatchSize int
}

// ExtractResult is one run.
type ExtractResult struct {
	OpIDs []int64
	Files []FileResult
	// Interrupted is true when the caller's context was cancelled. The
	// batches already committed stay committed (spec 3.7.1 ①).
	Interrupted bool
}

// Extract runs extractors over the files that need them and records what they
// found.
//
// The extractors run outside the transaction; only the resulting op goes
// inside one. That is what lets an extractor be arbitrarily slow or impure
// without holding a write lock (spec 1.7.1, 2.7).
func (l *Library) Extract(ctx context.Context, opt ExtractOptions, progress func(done int)) (*ExtractResult, error) {
	extractors, err := l.extractorsFor(opt)
	if err != nil {
		return nil, err
	}
	return l.extractUsing(ctx, extractors, opt, progress)
}

// extractUsing is Extract with the set of extractors already chosen. Tests use
// it to run one that is not in the global registry, which is the only way to
// exercise the diffing and batching without depending on what EXIF happens to
// find in a fixture.
func (l *Library) extractUsing(
	ctx context.Context, extractors []extract.Extractor, opt ExtractOptions, progress func(done int),
) (*ExtractResult, error) {
	batch := opt.BatchSize
	if batch <= 0 {
		batch = DefaultExtractBatch
	}

	res := &ExtractResult{}
	done := 0
	for _, e := range extractors {
		var after domain.FileID
		for {
			if ctx.Err() != nil {
				res.Interrupted = true
				return res, nil
			}
			ids, err := l.pending(e, opt.Force, after, batch)
			if err != nil {
				return nil, err
			}
			if len(ids) == 0 {
				break
			}
			after = ids[len(ids)-1]

			opID, results, err := l.extractBatch(ctx, e, ids)
			if err != nil {
				return nil, err
			}
			if opID != 0 {
				res.OpIDs = append(res.OpIDs, opID)
			}
			res.Files = append(res.Files, results...)
			done += len(ids)
			if progress != nil {
				progress(done)
			}
		}
	}
	return res, nil
}

func (l *Library) extractorsFor(opt ExtractOptions) ([]extract.Extractor, error) {
	if opt.Extractor == "" {
		return extract.All(), nil
	}
	e, err := extract.Lookup(opt.Extractor)
	if err != nil {
		return nil, err
	}
	return []extract.Extractor{e}, nil
}

// pending is the next batch of files this extractor has not seen.
//
// The cursor comes from the caller rather than from the recorded version,
// because a file that fails extraction is deliberately not recorded — see
// store.FilesNeedingExtraction.
func (l *Library) pending(
	e extract.Extractor, force bool, after domain.FileID, batch int,
) ([]domain.FileID, error) {
	version := e.Version()
	if force {
		// "Not yet seen at version+1" selects everything, without needing a
		// second query shape.
		version++
	}
	return store.FilesNeedingExtraction(l.db, e.ID(), version, after, batch)
}

// extractBatch runs one extractor over one batch and writes at most one op.
func (l *Library) extractBatch(
	ctx context.Context, e extract.Extractor, ids []domain.FileID,
) (int64, []FileResult, error) {
	payload := oplog.ExtractMetadataPayload{Extractor: e.ID(), Version: e.Version()}
	var results []FileResult

	for _, id := range ids {
		if ctx.Err() != nil {
			break
		}
		change, result := l.extractOne(ctx, e, id)
		results = append(results, result)
		if change != nil {
			payload.Files = append(payload.Files, *change)
		}
	}
	if len(payload.Files) == 0 {
		return 0, results, nil
	}
	// PreviousVersion is what an undo puts back. Every file in a batch shares
	// it because the batch is selected by "has not reached this version".
	payload.PreviousVersion = previousVersion(l.db, e.ID(), payload.Files[0].FileID)

	raw, err := oplog.Marshal(payload)
	if err != nil {
		return 0, nil, err
	}
	// The op must be written even when the context is done: that is what
	// "graceful stop" means, and the work is already paid for (spec 3.7.1 ①).
	opID, err := l.write(context.WithoutCancel(ctx), oplog.ActionExtractMetadata, raw, oplog.OriginExtractor, writeOptions{})
	if err != nil {
		return 0, nil, err
	}
	return opID, results, nil
}

func previousVersion(q store.Queryer, extractor string, file domain.FileID) int {
	got, err := store.Extractions(q, file)
	if err != nil {
		return 0
	}
	return got[extractor]
}

// extractOne runs one extractor over one file and works out what changed.
//
// It never returns an error: a failure here is this file's outcome, not the
// batch's (spec 3.7.1 ③). A failed file is left out of the payload entirely,
// which is also what keeps it eligible for a retry — nothing records it as
// extracted.
func (l *Library) extractOne(
	ctx context.Context, e extract.Extractor, id domain.FileID,
) (*oplog.FileExtraction, FileResult) {
	r := FileResult{Status: FileFailed}

	file, ok, err := l.File(ctx, id)
	if err != nil || !ok {
		r.Err = fmt.Errorf("read file %s: %w", id, errors.Join(err, ErrNoSuchFile))
		return nil, r
	}
	r.Name = file.Name

	path, ok := l.blobs.LocalPath(blobKey(file.BlobKey))
	if !ok {
		r.Err = fmt.Errorf("%s: its content is not in this library", file.Name)
		return nil, r
	}
	f, err := os.Open(path)
	if err != nil {
		r.Err = err
		return nil, r
	}
	defer f.Close()

	mime, err := sniff(f)
	if err != nil {
		r.Err = err
		return nil, r
	}
	if !e.Wants(mime) {
		// Not a failure and not a change: a PDF has no EXIF, and most of an
		// archive is not photographs. It still counts as seen, so the next
		// run does not read it again.
		//
		// captured_at is carried through unchanged rather than left at its
		// zero value. Apply writes whatever the payload says, and a zero
		// Instant is a real value meaning NULL — so an empty one here would
		// wipe the time the import resolved from mtime. That is precisely
		// what happened the first time this was run by hand: every
		// non-photograph in the library lost its capture time to a skip.
		r.Status = FileSkipped
		return &oplog.FileExtraction{
			FileID:      id,
			CapturedAt:  file.CapturedAt,
			CapturedWas: file.CapturedAt,
		}, r
	}
	if _, err := f.Seek(0, 0); err != nil {
		r.Err = err
		return nil, r
	}

	found, err := e.Extract(ctx, f, mime)
	if err != nil {
		r.Err = err
		return nil, r
	}
	if err := extract.Check(e, found); err != nil {
		r.Err = err
		return nil, r
	}

	change, err := l.diffExtraction(ctx, e, file, found)
	if err != nil {
		r.Err = err
		return nil, r
	}
	r.Status = FileImported
	return change, r
}

// diffExtraction turns "what the extractor found" into "what actually changes".
//
// Only real changes go into the op, the same rule tag_files follows: an op
// claiming a change it did not make would invert into an undo that changes
// something the user never did (spec 1.7.1).
func (l *Library) diffExtraction(
	ctx context.Context, e extract.Extractor, file domain.File, found []extract.KV,
) (*oplog.FileExtraction, error) {
	stored, err := store.FileMetadata(l.db, file.ID)
	if err != nil {
		return nil, err
	}
	owned := map[string]bool{}
	for _, ns := range e.Namespaces() {
		owned[ns] = true
	}

	was := map[string]extract.KV{}
	// owner records who wrote each key that already exists. A key belonging to
	// somebody else is off limits entirely — not merely absent from the before
	// state. Getting that distinction wrong is how a hand-entered passport
	// expiry gets washed away by a re-scan (spec 1.7 rule ②, generalised by
	// 1.7.6 ①), and it is silent: the op looks like an ordinary update.
	owner := map[string]string{}
	for _, row := range stored {
		owner[row.Key] = row.Source
		if row.Source == e.ID() {
			was[row.Key] = row.KV
		}
	}

	change := oplog.FileExtraction{FileID: file.ID}
	seen := map[string]bool{}
	for _, kv := range found {
		if src, taken := owner[kv.Key]; taken && src != e.ID() {
			continue
		}
		seen[kv.Key] = true
		if old, had := was[kv.Key]; had && old == kv {
			continue
		}
		change.Set = append(change.Set, kv)
		if old, had := was[kv.Key]; had {
			change.Was = append(change.Was, old)
		}
	}
	for key, old := range was {
		if !seen[key] {
			change.Removed = append(change.Removed, key)
			change.Was = append(change.Was, old)
		}
	}
	sort.Strings(change.Removed)
	sort.Slice(change.Was, func(i, j int) bool { return change.Was[i].Key < change.Was[j].Key })

	// captured_at is recomputed from every row this file has, not just this
	// extractor's: a GPS extractor contributing geo:timezone has to be able to
	// change a conclusion drawn from EXIF (spec 1.5.2).
	merged := mergedTags(stored, e.ID(), found)
	captured := extract.ResolveCaptured(merged, l.zone())
	var at domain.Instant
	if captured.Known {
		at = domain.NewInstant(captured.UnixNano)
	}
	if at != file.CapturedAt {
		change.CapturedAt = at
		change.CapturedWas = file.CapturedAt
	} else {
		change.CapturedAt = at
		change.CapturedWas = at
	}
	return &change, nil
}

// mergedTags is every row this file will have once this extraction lands.
func mergedTags(stored []store.MetadataRow, extractor string, found []extract.KV) []extract.KV {
	out := make([]extract.KV, 0, len(stored)+len(found))
	protected := make(map[string]bool)
	for _, row := range stored {
		if row.Source == extractor && !row.Tombstone {
			continue // this extractor replaces its own observations
		}
		protected[row.Key] = true
		if !row.Tombstone {
			out = append(out, row.KV)
		}
	}
	for _, kv := range found {
		if !protected[kv.Key] {
			out = append(out, kv)
		}
	}
	return out
}

// observations drops tombstones, which carry no value and must not reach the
// capture-time rules as an empty reading of the key they suppress.
func observations(rows []store.MetadataRow) []extract.KV {
	out := make([]extract.KV, 0, len(rows))
	for _, r := range rows {
		if !r.Tombstone {
			out = append(out, r.KV)
		}
	}
	return out
}

// sniff reads enough of a file to name its media type.
//
// From the content, never the file name: an imported file is called whatever
// the user called it, and spec 3.4 makes no promise that the extension
// survived. HEIC needs its own test because DetectContentType predates it.
func sniff(f *os.File) (string, error) { return extract.DetectMedia(f) }

// Count returns how many results have the given status.
func (r *ExtractResult) Count(s FileStatus) int {
	n := 0
	for _, f := range r.Files {
		if f.Status == s {
			n++
		}
	}
	return n
}

// FileMetadata reads everything recorded about a file.
func (l *Library) FileMetadata(ctx context.Context, id domain.FileID) ([]extract.Row, error) {
	return store.FileMetadata(l.db, id)
}

// CapturedInfo says how a file's capture time was established.
//
// It is the structured form of what DescribeCapturedAt says in prose, so that
// the JSON output and the human one cannot disagree about a file (spec 2.8).
// From names the metadata key the time came from; Zone how the timezone was
// established (spec 1.5.2); Extracted which extractors have run, which is what
// separates "nothing has looked yet" from "nothing in this file says when".
type CapturedInfo struct {
	From      string
	Zone      string
	Extracted []string
}

// CapturedInfo resolves the provenance of one file's capture time.
func (l *Library) CapturedInfo(ctx context.Context, f domain.File) (CapturedInfo, error) {
	var result CapturedInfo
	err := l.readTx(ctx, func(tx *store.ViewTx) error {
		var err error
		result, err = readCapturedInfo(tx.Queryer(), l.zone(), f)
		return err
	})
	return result, err
}

func readCapturedInfo(q store.Queryer, zone *time.Location, f domain.File) (CapturedInfo, error) {
	seen, err := store.Extractions(q, f.ID)
	if err != nil {
		return CapturedInfo{}, err
	}
	info := CapturedInfo{Extracted: make([]string, 0, len(seen))}
	for id := range seen {
		info.Extracted = append(info.Extracted, id)
	}
	slices.Sort(info.Extracted)

	if _, ok := f.CapturedAt.Time(zone); !ok {
		return info, nil
	}
	rows, err := store.FileMetadata(q, f.ID)
	if err != nil {
		return CapturedInfo{}, err
	}
	c := extract.ResolveCaptured(observations(rows), zone)
	info.From, info.Zone = c.From, string(c.Zone)
	return info, nil
}

// DescribeCapturedAt writes the capture time in a form that says how much of
// it is known.
//
// Saying "2019-08-12 14:03" for a time assembled with an assumed timezone,
// with no hint that the zone was assumed, is how a person comes to believe a
// guess (spec 1.5.2). The rendering is the place that promise is kept.
func (l *Library) DescribeCapturedAt(ctx context.Context, w io.Writer, f domain.File) {
	seen, err := store.Extractions(l.db, f.ID)
	if err != nil {
		return
	}
	t, ok := f.CapturedAt.Time(l.zone())
	if !ok {
		// "Run kist extract" is only useful advice if it has not already been
		// run. Saying it to somebody who just ran it sends them round a loop
		// looking for a bug that is not there — the honest answer is that the
		// file carries no time.
		if len(seen) == 0 {
			fmt.Fprintln(w, "captured   unknown — run `kist extract`")
		} else {
			fmt.Fprintln(w, "captured   unknown — nothing in this file says when it was made")
		}
		return
	}

	rows, err := store.FileMetadata(l.db, f.ID)
	if err != nil {
		fmt.Fprintf(w, "captured   %s\n", t.Format("2006-01-02 15:04:05"))
		return
	}
	c := extract.ResolveCaptured(observations(rows), l.zone())

	// The zone always gets a sentence, including when it is known. A time
	// rendered in the library's zone from a photograph taken elsewhere is
	// correct and surprising — spec 1.5.2 accepted that cost explicitly, and
	// this line is where it stops being a surprise.
	var note string
	switch c.Zone {
	case extract.ZoneAssumed:
		note = fmt.Sprintf("  (shown in %s, which was assumed — the file does not say)", l.zone())
	case extract.ZoneFromFile, extract.ZoneFromLocation:
		note = fmt.Sprintf("  (shown in %s; the file records %s)", l.zone(), c.From)
	case extract.ZoneExact:
		note = "  (from " + c.From + ")"
	}
	fmt.Fprintf(w, "captured   %s%s\n", t.Format("2006-01-02 15:04:05"), note)
}
