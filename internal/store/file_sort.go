package store

import (
	"database/sql/driver"
	"github.com/jinzhao1994/kist/internal/domain"

	"github.com/jinzhao1994/kist/internal/query"
	"modernc.org/sqlite"
)

func init() {
	sqlite.MustRegisterDeterministicScalarFunction("kist_file_type", 2, func(_ *sqlite.FunctionContext, args []driver.Value) (driver.Value, error) {
		name, _ := args[0].(string)
		mime, _ := args[1].(string)
		return domain.DisplayFileType(name, mime), nil
	})
}

// These keys use existing indexed relations rather than operation payloads.
func extraFileSortKey(key query.SortKey) string {
	switch key {
	case query.SortSize:
		return `(SELECT size FROM blobs WHERE key=files.blob_key)`
	case query.SortType:
		return `kist_file_type(name, (SELECT value FROM file_metadata WHERE file_id=files.id AND key='media:mime'))`
	}
	return ""
}
