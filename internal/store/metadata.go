package store

import (
	"fmt"

	"github.com/jinzhao1994/kist/internal/domain"
	"github.com/jinzhao1994/kist/internal/extract"
)

// ── the write side, reached only through oplog.Tx ───────────────────────────

// SetFileMetadata writes one observation, replacing any row with the same key.
//
// The upsert is deliberate rather than incidental: Apply must be able to run
// the same payload twice and land in the same place, because that is what a
// rebuild does after a partial failure (spec 2.3.1).
func (v *ViewTx) SetFileMetadata(file domain.FileID, kv extract.KV, source string) error {
	_, err := v.tx.Exec(`
		INSERT INTO file_metadata (file_id, key, value, type, source)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT (file_id, key) DO UPDATE SET
			value = excluded.value, type = excluded.type, source = excluded.source`,
		file, kv.Key, kv.Value, string(kv.Type), source)
	if err != nil {
		return fmt.Errorf("set metadata %s on %s: %w", kv.Key, file, err)
	}
	return nil
}

func (v *ViewTx) DeleteFileMetadata(file domain.FileID, key string) error {
	if _, err := v.tx.Exec(`DELETE FROM file_metadata WHERE file_id = ? AND key = ?`, file, key); err != nil {
		return fmt.Errorf("delete metadata %s on %s: %w", key, file, err)
	}
	return nil
}

func (v *ViewTx) SetFileExtraction(file domain.FileID, extractor string, version int) error {
	_, err := v.tx.Exec(`
		INSERT INTO file_extractions (file_id, extractor, version) VALUES (?, ?, ?)
		ON CONFLICT (file_id, extractor) DO UPDATE SET version = excluded.version`,
		file, extractor, version)
	if err != nil {
		return fmt.Errorf("record extraction %s on %s: %w", extractor, file, err)
	}
	return nil
}

func (v *ViewTx) DeleteFileExtraction(file domain.FileID, extractor string) error {
	_, err := v.tx.Exec(`DELETE FROM file_extractions WHERE file_id = ? AND extractor = ?`, file, extractor)
	if err != nil {
		return fmt.Errorf("clear extraction %s on %s: %w", extractor, file, err)
	}
	return nil
}

func (v *ViewTx) SetCapturedAt(file domain.FileID, at domain.Instant) error {
	if _, err := v.tx.Exec(`UPDATE files SET captured_at = ? WHERE id = ?`, at, file); err != nil {
		return fmt.Errorf("set captured_at on %s: %w", file, err)
	}
	return nil
}

// ── the read side ───────────────────────────────────────────────────────────

// MetadataRow is one stored observation plus who wrote it.
type MetadataRow struct {
	extract.KV
	Source string
}

// FileMetadata reads every row for one file, ordered by key.
func FileMetadata(q Queryer, file domain.FileID) ([]MetadataRow, error) {
	rows, err := q.Query(
		`SELECT key, value, type, source FROM file_metadata WHERE file_id = ? ORDER BY key`, file)
	if err != nil {
		return nil, fmt.Errorf("read metadata of %s: %w", file, err)
	}
	defer rows.Close()

	var out []MetadataRow
	for rows.Next() {
		var r MetadataRow
		var typ string
		if err := rows.Scan(&r.Key, &r.Value, &typ, &r.Source); err != nil {
			return nil, err
		}
		r.Type = extract.Type(typ)
		out = append(out, r)
	}
	return out, rows.Err()
}

// Extractions reports which extractors have seen a file, and at what version.
func Extractions(q Queryer, file domain.FileID) (map[string]int, error) {
	rows, err := q.Query(`SELECT extractor, version FROM file_extractions WHERE file_id = ?`, file)
	if err != nil {
		return nil, fmt.Errorf("read extractions of %s: %w", file, err)
	}
	defer rows.Close()

	out := map[string]int{}
	for rows.Next() {
		var name string
		var version int
		if err := rows.Scan(&name, &version); err != nil {
			return nil, err
		}
		out[name] = version
	}
	return out, rows.Err()
}

// FilesNeedingExtraction lists files an extractor has not seen at this version.
//
// This is the query file_extractions exists for (spec 1.7.6 ③). The same
// answer could be had by reading every payload, but payload is not queryable
// (invariant 4) — and at a hundred thousand files that would mean parsing tens
// of megabytes of JSON to decide there is nothing to do.
//
// after is a cursor: pass the zero FileID for the first batch, then the last
// id of the previous one. It is not an optimisation.
//
// Relying on the recorded version to advance instead would loop forever the
// first time a file failed to extract — a failure is deliberately not
// recorded, so that a later run retries it (spec 3.7.1 ③), which means the
// same batch would come back unchanged for as long as the process lived.
func FilesNeedingExtraction(
	q Queryer, extractor string, version int, after domain.FileID, limit int,
) ([]domain.FileID, error) {
	rows, err := q.Query(`
		SELECT f.id FROM files f
		LEFT JOIN file_extractions e ON e.file_id = f.id AND e.extractor = ?
		WHERE (e.version IS NULL OR e.version < ?) AND f.id > ?
		ORDER BY f.id
		LIMIT ?`, extractor, version, after, limit)
	if err != nil {
		return nil, fmt.Errorf("find files needing %s: %w", extractor, err)
	}
	defer rows.Close()

	var out []domain.FileID
	for rows.Next() {
		var id domain.FileID
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}
