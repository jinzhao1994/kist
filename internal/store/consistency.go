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

// BoxesWithBadRoot finds boxes whose root_folder_id names a folder that is
// missing, or one that has a parent and so is not a root at all.
//
// It replaces the two checks that used to guard files.box_id and
// folders.box_id against drifting from the folder tree. Those columns are
// gone, and with them both corruptions: a folder's box is now written down in
// exactly one place, and one copy of an answer cannot disagree with itself.
// What is left to check is that single edge — a scan of the boxes table rather
// than of every file in the library.
func BoxesWithBadRoot(q Queryer) ([]Inconsistency, error) {
	return inconsistencies(q, "check box roots", `
		SELECT hex(b.id) || ' ' || b.name
		FROM boxes b
		LEFT JOIN folders d ON d.id = b.root_folder_id
		WHERE d.id IS NULL OR d.parent_id IS NOT NULL
		ORDER BY b.id`)
}

// UnclaimedRootFolders finds parentless folders that no box calls its root.
//
// Such a folder is invisible: every read path starts at a box and walks down,
// so its whole subtree — files included — is unreachable without being
// missing. verify would otherwise report a healthy library.
func UnclaimedRootFolders(q Queryer) ([]Inconsistency, error) {
	return inconsistencies(q, "check root folder ownership", `
		SELECT hex(d.id) || ' ' || d.name
		FROM folders d
		WHERE d.parent_id IS NULL
		  AND d.id NOT IN (SELECT root_folder_id FROM boxes)
		ORDER BY d.id`)
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
	// The box comes from walking up to the root folder, since a file no
	// longer carries a box_id (spec 1.4). The walk is per damaged file, and
	// damaged files are the rare case this function exists for.
	rows, err := q.Query(`
		WITH RECURSIVE up(file_id, blob_key, name, folder_id) AS (
			SELECT id, blob_key, name, folder_id FROM files
			WHERE blob_key IN (`+placeholders(len(keys))+`)
		  UNION
			SELECT u.file_id, u.blob_key, u.name, d.parent_id
			FROM up u JOIN folders d ON d.id = u.folder_id
			WHERE d.parent_id IS NOT NULL
		)
		SELECT up.blob_key, b.name || '/' || up.name
		FROM up JOIN boxes b ON b.root_folder_id = up.folder_id
		ORDER BY up.blob_key, up.file_id`, args...)
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

// FolderSizeDrift independently recomputes logical bytes for deep verification.
// UNION bounds recursion even if the folder graph is damaged.
func FolderSizeDrift(q Queryer) ([]Inconsistency, error) {
	return inconsistencies(q, "check folder sizes", `
 WITH RECURSIVE ancestry(descendant, ancestor) AS (
   SELECT id, id FROM folders
   UNION
   SELECT a.descendant, f.parent_id FROM ancestry a JOIN folders f ON f.id=a.ancestor
   WHERE f.parent_id IS NOT NULL
 ), direct AS (
   SELECT f.folder_id, sum(b.size) AS size FROM files f JOIN blobs b ON b.key=f.blob_key GROUP BY f.folder_id
 ), totals AS (
   SELECT a.ancestor, sum(d.size) AS size FROM ancestry a JOIN direct d ON d.folder_id=a.descendant GROUP BY a.ancestor
 )
 SELECT hex(f.id) || ' ' || f.name || ': stored=' || f.total_size || ', expected=' || coalesce(t.size,0)
 FROM folders f LEFT JOIN totals t ON t.ancestor=f.id WHERE f.total_size != coalesce(t.size,0) ORDER BY f.id`)
}
