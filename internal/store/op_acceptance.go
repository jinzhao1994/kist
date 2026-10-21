package store

import (
	"database/sql"
	"time"
)

// OpAcceptedAt returns the NAS acceptance time only for synchronized operations.
func OpAcceptedAt(q Queryer, id int64) (*time.Time, error) {
	var ns int64
	err := q.QueryRow(`SELECT accepted_at FROM op_sync WHERE op_id = ?`, id).Scan(&ns)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	value := ParseTime(ns)
	return &value, nil
}
