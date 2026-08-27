package store

import "fmt"

// Inconsistency is one row that should not exist. Detail is already
// human-readable: these queries know the names, and making the caller look
// them up again would mean a second round trip per finding.
type Inconsistency struct {
	Detail string
}

func inconsistencies(q Queryer, what, query string) ([]Inconsistency, error) {
	rows, err := q.Query(query)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", what, err)
	}
	defer rows.Close()

	var out []Inconsistency
	for rows.Next() {
		var detail string
		if err := rows.Scan(&detail); err != nil {
			return nil, fmt.Errorf("%s: %w", what, err)
		}
		out = append(out, Inconsistency{Detail: detail})
	}
	return out, rows.Err()
}

// OrphanFiles finds files whose folder is gone.
func OrphanFiles(q Queryer) ([]Inconsistency, error) {
	return inconsistencies(q, "check for orphan files", `
		SELECT hex(f.id) || ' ' || f.name
		FROM files f
		LEFT JOIN folders d ON d.id = f.folder_id
		WHERE d.id IS NULL
		ORDER BY f.id`)
}

// FilesWithBoxMismatch finds files whose box_id disagrees with their folder's.
//
// files.box_id is denormalised on purpose — it is the hottest filter there is
// (spec 1.5) — and a denormalised column is a column that can drift.
func FilesWithBoxMismatch(q Queryer) ([]Inconsistency, error) {
	return inconsistencies(q, "check file/folder box agreement", `
		SELECT hex(f.id) || ' ' || f.name
		FROM files f
		JOIN folders d ON d.id = f.folder_id
		WHERE f.box_id != d.box_id
		ORDER BY f.id`)
}

// FilesWithUnknownBlob finds files pointing at content the database has no row
// for. That is different from the content being missing from disk, which is
// the blob layer's business.
func FilesWithUnknownBlob(q Queryer) ([]Inconsistency, error) {
	return inconsistencies(q, "check file blob keys", `
		SELECT hex(f.id) || ' ' || f.name || ' → ' || f.blob_key
		FROM files f
		LEFT JOIN blobs b ON b.key = f.blob_key
		WHERE b.key IS NULL
		ORDER BY f.id`)
}

// UnreachableFolders finds folders that cannot be walked to from their box's
// root.
//
// One query covers two different corruptions — a parent that is gone, and a
// cycle — because from the root's point of view they look the same: you never
// arrive. That is why there is no separate cycle detector; writing one would
// mean two queries that have to agree about what a healthy tree is.
//
// The recursion cannot run away, and the reason is worth writing down because
// the obvious worry — "a cycle will make this loop forever" — is wrong here.
// parent_id is single-valued, so a folder inside a cycle has its parent inside
// that cycle and therefore is not a child of any root. A cycle is unreachable
// from a root by construction, which is exactly why walking down from the
// roots finds cycles without ever entering one.
//
// Measured, not assumed: swapping UNION for UNION ALL leaves this query's
// behaviour and this package's tests unchanged, for the same reason (each
// folder has one path, so there is nothing to deduplicate). UNION stays
// because it is the form that remains correct if parents ever become
// multi-valued, and it costs nothing.
func UnreachableFolders(q Queryer) ([]Inconsistency, error) {
	return inconsistencies(q, "check folder reachability", `
		WITH RECURSIVE reachable(id) AS (
		    SELECT id FROM folders WHERE parent_id IS NULL
		  UNION
		    SELECT f.id FROM folders f JOIN reachable r ON f.parent_id = r.id
		)
		SELECT hex(d.id) || ' ' || d.name
		FROM folders d
		WHERE d.id NOT IN (SELECT id FROM reachable)
		ORDER BY d.id`)
}

// CrossBoxFolders finds folders whose parent belongs to another box. The
// projection walks the tree from each box root, so such a folder would be
// projected into a box its files do not belong to.
func CrossBoxFolders(q Queryer) ([]Inconsistency, error) {
	return inconsistencies(q, "check folder box agreement", `
		SELECT hex(d.id) || ' ' || d.name
		FROM folders d
		JOIN folders p ON p.id = d.parent_id
		WHERE d.box_id != p.box_id
		ORDER BY d.id`)
}

// BlobRow is one row of the blobs table.
type BlobRow struct {
	Key    string
	SHA256 string
	Size   int64
}

// AllBlobs reads the whole blobs table, ordered by key.
//
// The whole table at once is deliberate: a hundred thousand blobs is about ten
// megabytes of these, and both callers — verify and scrub — need to hold a set
// anyway, to work out what is on disk but not in here.
func AllBlobs(q Queryer) ([]BlobRow, error) {
	rows, err := q.Query(`SELECT key, sha256, size FROM blobs ORDER BY key`)
	if err != nil {
		return nil, fmt.Errorf("read blobs: %w", err)
	}
	defer rows.Close()

	var out []BlobRow
	for rows.Next() {
		var b BlobRow
		if err := rows.Scan(&b.Key, &b.SHA256, &b.Size); err != nil {
			return nil, fmt.Errorf("read blobs: %w", err)
		}
		out = append(out, b)
	}
	return out, rows.Err()
}

// FilesUsingBlobs maps blob keys onto the files that reference them, as
// "<box>/<file>".
//
// This exists so a damage report can say "云南/passport.jpg" instead of
// "sha256:94bacd25…". Spec 3.8 puts naming the damaged files first among the
// things verify is for — "your 2019 passport scan is damaged" is the sentence
// that lets somebody go and look for another copy, and a bare digest is not
// that sentence.
//
// The full folder path is left out deliberately: it needs a recursive walk per
// file, and the box plus the name is already enough to recognise something.
func FilesUsingBlobs(q Queryer, keys []string) (map[string][]string, error) {
	if len(keys) == 0 {
		return nil, nil
	}
	args := make([]any, len(keys))
	for i, k := range keys {
		args[i] = k
	}
	rows, err := q.Query(`
		SELECT f.blob_key, b.name || '/' || f.name
		FROM files f
		JOIN boxes b ON b.id = f.box_id
		WHERE f.blob_key IN (`+placeholders(len(keys))+`)
		ORDER BY f.blob_key, f.id`, args...)
	if err != nil {
		return nil, fmt.Errorf("look up the files behind damaged content: %w", err)
	}
	defer rows.Close()

	out := map[string][]string{}
	for rows.Next() {
		var key, name string
		if err := rows.Scan(&key, &name); err != nil {
			return nil, fmt.Errorf("look up the files behind damaged content: %w", err)
		}
		out[key] = append(out[key], name)
	}
	return out, rows.Err()
}
