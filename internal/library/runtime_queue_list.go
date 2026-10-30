package library

import (
	"context"
	"encoding/base64"
	"encoding/json"

	"github.com/jinzhao1994/kist/internal/domain"
	"github.com/jinzhao1994/kist/internal/store"
)

// ExtractionQueueItem omits provider output and resolves current archive names.
// Unavailable files retain their task snapshot name without a navigation link.
type ExtractionQueueItem struct {
	FileID        domain.FileID `json:"file_id"`
	Name          string        `json:"name"`
	State         string        `json:"state" enum:"queued,failed"`
	Error         string        `json:"error,omitempty"`
	RequestID     string        `json:"request_id"`
	FileAvailable bool          `json:"file_available"`
	Box           *QueueItemBox `json:"box,omitempty"`
}

type QueueItemBox struct {
	Name    string `json:"name"`
	StartAt string `json:"start_at"`
}

type ExtractionQueuePage struct {
	Items      []ExtractionQueueItem `json:"items"`
	NextCursor string                `json:"next_cursor"`
}

type extractionQueueCursor struct {
	Kind     string `json:"kind"`
	Position int64  `json:"position"`
	Key      string `json:"key"`
}

// Jobs lists pending and failed work, not historical attempts or completed jobs.
// Keyset pagination survives earlier jobs completing between page requests.
func (q *OCRQueue) Jobs(ctx context.Context, cursor string, limit int) (ExtractionQueuePage, error) {
	out := ExtractionQueuePage{Items: []ExtractionQueueItem{}}
	if limit < 1 || limit > 100 {
		return out, ErrInvalidRequest
	}
	after := extractionQueueCursor{Position: -1}
	if cursor != "" {
		if len(cursor) > 1024 {
			return out, ErrInvalidRequest
		}
		raw, err := base64.RawURLEncoding.DecodeString(cursor)
		if err != nil || json.Unmarshal(raw, &after) != nil || after.Kind != q.kind || after.Position < 0 || after.Key == "" {
			return out, ErrInvalidRequest
		}
	}
	unlock, err := q.state.Acquire(ctx)
	if err != nil {
		return out, err
	}
	db, err := q.database()
	unlock()
	if err != nil {
		return out, err
	}
	rows, err := db.DB.QueryContext(ctx, `SELECT file_id,name,state,error,request_id,position,job_key
	 FROM background_jobs WHERE kind=? AND state IN ('queued','failed') AND (position,job_key)>(?,?)
	 ORDER BY position,job_key LIMIT ?`, q.kind, after.Position, after.Key, limit+1)
	if err != nil {
		return out, err
	}
	ids := []domain.FileID{}
	var last extractionQueueCursor
	for rows.Next() {
		if len(out.Items) == limit {
			raw, _ := json.Marshal(last)
			out.NextCursor = base64.RawURLEncoding.EncodeToString(raw)
			break
		}
		var item ExtractionQueueItem
		var fileID string
		last.Kind = q.kind
		if err = rows.Scan(&fileID, &item.Name, &item.State, &item.Error, &item.RequestID, &last.Position, &last.Key); err != nil {
			break
		}
		parsed, parseErr := domain.ParseID(fileID)
		if parseErr != nil {
			err = parseErr
			break
		}
		item.FileID = domain.FileID{ID: parsed}
		out.Items = append(out.Items, item)
		ids = append(ids, item.FileID)
	}
	if err == nil {
		err = rows.Err()
	}
	rows.Close()
	if err != nil {
		return out, err
	}
	err = q.library.readTx(ctx, func(tx *store.ViewTx) error {
		locations, err := store.FileLocations(tx.Queryer(), ids)
		if err != nil {
			return err
		}
		for i := range out.Items {
			item := &out.Items[i]
			if location, ok := locations[item.FileID]; ok {
				item.Name = location.Name
				item.FileAvailable = true
				item.Box = &QueueItemBox{Name: location.BoxName, StartAt: location.BoxStart}
			}
		}
		return nil
	})
	return out, err
}
