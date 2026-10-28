package store

import "github.com/jinzhao1994/kist/internal/domain"

// OCRCandidates walks the current files using only materialized indexes.
// Negative after selects all missing files for an explicit backfill.
func OCRCandidates(q Queryer, after, through int64, cursor domain.FileID, limit int) ([]domain.FileID, error) {
	return ImageTextCandidates(q, "ocr", after, through, cursor, limit)
}

// ImageTextCandidates keeps independently versioned tasks out of each other's results.
func ImageTextCandidates(q Queryer, kind string, after, through int64, cursor domain.FileID, limit int) ([]domain.FileID, error) {
	rows, err := q.Query(`SELECT f.id FROM files f WHERE f.id > ?
 AND ((? <> 'orientation' AND NOT EXISTS (SELECT 1 FROM file_extractions e WHERE e.file_id=f.id AND e.extractor=? AND e.version>=1))
 OR (? = 'orientation' AND NOT EXISTS (SELECT 1 FROM file_metadata a WHERE a.file_id=f.id AND a.key='orientation:clockwise_degrees' AND a.value IN ('0','90','180','270') AND a.type='int' AND a.source IN ('orientation','user') AND NOT EXISTS (SELECT 1 FROM file_metadata h WHERE h.file_id=f.id AND h.key='orientation:input_sha256' AND h.value<>f.blob_key))))
 AND EXISTS (SELECT 1 FROM op_files x JOIN ops o ON o.id=x.op_id WHERE x.file_id=f.id AND (? < 0 OR o.id>?) AND o.id<=? AND o.action IN ('import_files','upload_files') AND o.reverts_id IS NULL)
 ORDER BY f.id LIMIT ?`, cursor, kind, kind, kind, after, after, through, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []domain.FileID{}
	for rows.Next() {
		var id domain.FileID
		if err = rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}
