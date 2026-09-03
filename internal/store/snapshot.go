package store

import (
	"fmt"
	"strings"
)

// snapshotTables lists the rebuildable tables and how to order them so the
// dump is byte-stable.
//
// ops is absent: it is the authority, not the thing being proved. blobs is
// present from M1c on — Apply maintains it, idempotently, so a rebuild has to
// reproduce it exactly like everything else here.
var snapshotTables = []struct {
	name    string
	columns string
	order   string
}{
	{"blobs", "key, sha256, size", "key"},
	{"boxes", "hex(id), name, name_fold, description, created_at, updated_at, coalesce(start_at,''), coalesce(end_at,'')", "id"},
	{"folders", "hex(id), hex(box_id), coalesce(hex(parent_id),''), name, name_fold, created_at, updated_at", "id"},
	{"files", "hex(id), hex(box_id), hex(folder_id), name, name_fold, blob_key, coalesce(captured_at,''), created_at, updated_at", "id"},
	{"tags", "hex(id), coalesce(hex(parent_id),''), name, name_fold", "id"},
	{"box_tags", "hex(box_id), hex(tag_id)", "box_id, tag_id"},
	{"file_tags", "hex(file_id), hex(tag_id)", "file_id, tag_id"},
	{"file_tag_sources", "hex(file_id), hex(tag_id), source", "file_id, tag_id, source"},
	{"tag_rejections", "hex(file_id), hex(tag_id)", "file_id, tag_id"},
	{"file_metadata", "hex(file_id), key, coalesce(value,''), coalesce(type,''), source", "file_id, key"},
	{"file_extractions", "hex(file_id), extractor, version", "file_id, extractor"},
	{"op_boxes", "op_id, hex(box_id)", "op_id, box_id"},
	{"op_files", "op_id, hex(file_id)", "op_id, file_id"},
	{"op_blobs", "op_id, blob_key", "op_id, blob_key"},
}

// Snapshot renders every rebuildable table as deterministic text.
//
// It exists for one test, and that test is the one that keeps invariant 3
// honest: dump, rebuild, dump again, compare (spec 2.3.1). A snapshot whose
// row order could vary would make that test flap, which is worse than not
// having it.
func Snapshot(q Queryer) (string, error) {
	var sb strings.Builder
	for _, t := range snapshotTables {
		fmt.Fprintf(&sb, "== %s ==\n", t.name)
		rows, err := q.Query(`SELECT ` + t.columns + ` FROM ` + t.name + ` ORDER BY ` + t.order)
		if err != nil {
			return "", fmt.Errorf("snapshot %s: %w", t.name, err)
		}
		cols, err := rows.Columns()
		if err != nil {
			rows.Close()
			return "", err
		}
		for rows.Next() {
			cells := make([]any, len(cols))
			ptrs := make([]any, len(cols))
			for i := range cells {
				ptrs[i] = &cells[i]
			}
			if err := rows.Scan(ptrs...); err != nil {
				rows.Close()
				return "", fmt.Errorf("snapshot %s: %w", t.name, err)
			}
			parts := make([]string, len(cells))
			for i, c := range cells {
				parts[i] = fmt.Sprintf("%v", c)
			}
			sb.WriteString(strings.Join(parts, "\t"))
			sb.WriteByte('\n')
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return "", fmt.Errorf("snapshot %s: %w", t.name, err)
		}
	}
	return sb.String(), nil
}
