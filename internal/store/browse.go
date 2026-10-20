package store

import (
	"fmt"
	"github.com/jinzhao1994/kist/internal/domain"
	"github.com/jinzhao1994/kist/internal/query"
	"strconv"
	"strings"
	"time"
)

// BrowseEntry distinguishes the stable result identity from its media file.
type BrowseEntry struct {
	File    domain.File
	Primary domain.FileID
	Fileset domain.FolderID
	Size    int64
}

func browseWhere(q query.Query, zone *time.Location) (string, []any, error) {
	base := q
	base.Keywords = nil
	base.Text = ""
	base.All = nil
	where, args, err := translate(base, zone)
	if err != nil {
		return "", nil, err
	}
	where = strings.ReplaceAll(where, "files.id", "files.primary_id")
	terms := q.Keywords
	if q.Text != "" {
		terms = append(append([]string{}, terms...), q.Text)
	}
	for _, term := range terms {
		where += ` AND (instr(name_fold,?)>0 OR instr(primary_name_fold,?)>0)`
		fold := domain.Fold(term)
		args = append(args, fold, fold)
	}
	for _, c := range q.All {
		w, a, e := browseWhere(c, zone)
		if e != nil {
			return "", nil, e
		}
		where += w
		args = append(args, a...)
	}
	return where, args, nil
}

// ListEntries filters representatives before pagination. Ordinary /files stays unchanged.
func ListEntries(db Queryer, q query.Query, include bool, zone *time.Location) ([]BrowseEntry, string, error) {
	if err := q.Validate(); err != nil {
		return nil, "", err
	}
	prefix := "entries-v1:" + strconv.FormatBool(include) + ":"
	if q.Cursor != "" {
		if !strings.HasPrefix(q.Cursor, prefix) {
			return nil, "", fmt.Errorf("cursor belongs to another result mode")
		}
		q.Cursor = strings.TrimPrefix(q.Cursor, prefix)
	}
	where, args, err := browseWhere(q, zone)
	if err != nil {
		return nil, "", err
	}
	scope := `1`
	var headArgs []any
	if q.Folder != nil {
		scope = `s.folder_id NOT IN (SELECT ancestor FROM ancestry WHERE child=?)`
		headArgs = append(headArgs, *q.Folder)
	}
	ordinary := `NOT EXISTS(SELECT 1 FROM ancestry a JOIN visible v ON a.ancestor=v.folder_id WHERE a.child=f.folder_id)`
	if include {
		ordinary = `NOT EXISTS(SELECT 1 FROM visible v JOIN primaries p ON p.set_id=v.folder_id WHERE p.file_id=f.id)`
	}
	cte := `WITH RECURSIVE
 ancestry(child,ancestor) AS (SELECT id,id FROM folders UNION SELECT a.child,f.parent_id FROM ancestry a JOIN folders f ON f.id=a.ancestor WHERE f.parent_id IS NOT NULL),
 choice(set_id,file_id,next_set) AS (
 SELECT s.folder_id,coalesce(s.primary_file_id,m.file_id),CASE WHEN s.primary_file_id IS NULL THEN m.folder_id END
 FROM filesets s LEFT JOIN fileset_members m ON m.fileset_id=s.folder_id AND m.position=(SELECT min(position) FROM fileset_members WHERE fileset_id=s.folder_id AND (s.kind='document_pages' OR (s.kind='live_photo' AND role='image') OR (s.kind='video_subtitles' AND role='video')))),
 primary_walk(set_id,file_id,next_set) AS (SELECT * FROM choice UNION SELECT w.set_id,c.file_id,c.next_set FROM primary_walk w JOIN choice c ON c.set_id=w.next_set),
 primaries AS (SELECT set_id,file_id FROM primary_walk WHERE file_id IS NOT NULL),
 eligible AS (SELECT s.* FROM filesets s WHERE ` + scope + `),
 visible AS (SELECT s.* FROM eligible s WHERE NOT EXISTS(SELECT 1 FROM ancestry a JOIN eligible parent ON parent.folder_id=a.ancestor WHERE a.child=s.folder_id AND a.ancestor<>s.folder_id)),
 files AS (
 SELECT f.id,f.folder_id,f.name,f.name_fold,f.blob_key,f.captured_at,f.created_at,f.updated_at,f.id AS primary_id,NULL AS fileset_id,f.name_fold AS primary_name_fold,f.name AS primary_name,coalesce(b.size,0) AS logical_size
 FROM main.files f LEFT JOIN blobs b ON b.key=f.blob_key WHERE ` + ordinary + `
 UNION ALL
 SELECT d.id,d.parent_id,d.name,d.name_fold,coalesce(f.blob_key,''),f.captured_at,coalesce(f.created_at,d.created_at),d.updated_at,f.id,d.id,coalesce(f.name_fold,''),coalesce(f.name,''),d.total_size
 FROM visible s JOIN folders d ON d.id=s.folder_id LEFT JOIN primaries p ON p.set_id=s.folder_id LEFT JOIN main.files f ON f.id=p.file_id)
 `
	sort := q.Sort
	imported := sort.Key == query.SortImported
	if imported {
		sort.Key = query.SortUpdated
	}
	transform := func(s string) string {
		s = strings.ReplaceAll(s, extraFileSortKey(query.SortSize), "logical_size")
		s = strings.ReplaceAll(s, extraFileSortKey(query.SortType), `kist_file_type(primary_name, (SELECT value FROM file_metadata WHERE file_id=files.primary_id AND key='media:mime'))`)
		if imported {
			s = strings.ReplaceAll(s, "updated_at", "created_at")
		}
		return s
	}
	key := "captured_at"
	switch q.Sort.Key {
	case query.SortName:
		key = "name_fold"
	case query.SortImported:
		key = "created_at"
	case query.SortUpdated:
		key = "updated_at"
	case query.SortType:
		key = transform(extraFileSortKey(query.SortType))
	case query.SortSize:
		key = "logical_size"
	}
	if q.Cursor != "" {
		value, id, e := query.DecodeCursor(q.Sort, q.Cursor)
		if e != nil {
			return nil, "", e
		}
		pred, values := cursorPredicate(sort, value, id)
		if sort.Key == query.SortUpdated || (sort.Key == query.SortCaptured && value != "") {
			for i, v := range values {
				if text, ok := v.(string); ok {
					number, e := strconv.ParseInt(text, 10, 64)
					if e != nil {
						return nil, "", e
					}
					values[i] = number
				}
			}
		}
		where += transform(pred)
		args = append(args, values...)
	}
	sql := cte + `SELECT ` + fileColumns + `,primary_id,fileset_id,logical_size,coalesce(CAST(` + key + ` AS TEXT),'') FROM files WHERE 1=1` + where + transform(orderBy(sort)) + ` LIMIT ?`
	args = append(append(headArgs, args...), q.PageSize()+1)
	rows, err := db.Query(sql, args...)
	if err != nil {
		return nil, "", err
	}
	defer rows.Close()
	var out []BrowseEntry
	var keys []string
	for rows.Next() {
		var e BrowseEntry
		var primary, set []byte
		var captured *int64
		var created, updated int64
		var k string
		if err = rows.Scan(&e.File.ID, &e.File.FolderID, &e.File.Name, &e.File.NameFold, &e.File.BlobKey, &captured, &created, &updated, &primary, &set, &e.Size, &k); err != nil {
			return nil, "", err
		}
		if captured != nil {
			e.File.CapturedAt = domain.NewInstant(*captured)
		}
		e.File.CreatedAt = ParseTime(created)
		e.File.UpdatedAt = ParseTime(updated)
		if primary != nil {
			if err = e.Primary.Scan(primary); err != nil {
				return nil, "", err
			}
		}
		if set != nil {
			if err = e.Fileset.Scan(set); err != nil {
				return nil, "", err
			}
		}
		out = append(out, e)
		keys = append(keys, k)
	}
	if err = rows.Err(); err != nil {
		return nil, "", err
	}
	next := ""
	if len(out) > q.PageSize() {
		out = out[:q.PageSize()]
		next = prefix + query.EncodeCursorValue(q.Sort, keys[len(out)-1], out[len(out)-1].File.ID)
	}
	return out, next, nil
}
