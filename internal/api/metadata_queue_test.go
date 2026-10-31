package api

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jinzhao1994/kist/internal/domain"
	"github.com/jinzhao1994/kist/internal/library"
	"github.com/jinzhao1994/kist/internal/query"
)

func TestLocalMetadataQueueRunsWithoutBrowserOrAI(t *testing.T) {
	s, l := testServer(t)
	ctx := t.Context()
	box, err := l.CreateBox(ctx, "Synthetic metadata", "", domain.LocalDate{}, domain.LocalDate{})
	if err != nil {
		t.Fatal(err)
	}
	src := t.TempDir()
	if err = os.WriteFile(filepath.Join(src, "note.txt"), []byte("Synthetic archive text"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err = l.Import(ctx, src, library.ImportOptions{Box: box.ID}); err != nil {
		t.Fatal(err)
	}
	files, _, err := l.Files(ctx, query.Query{Limit: 10})
	if err != nil || len(files) != 1 {
		t.Fatal(files, err)
	}
	id := files[0].ID
	runCtx, cancel := context.WithCancel(ctx)
	done := make(chan struct{})
	go func() { defer close(done); s.RunOCR(runCtx) }()
	defer func() { cancel(); <-done }()
	deadline := time.Now().Add(4 * time.Second)
	for {
		info, e := l.MediaInfo(ctx, id)
		if e != nil {
			t.Fatal(e)
		}
		status, e := l.MetadataQueue().Status(ctx)
		if e != nil {
			t.Fatal(e)
		}
		if len(info.Pending) == 0 && status.Running == "" && status.Queued == 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("import remained unprocessed without a page", info)
		}
		time.Sleep(10 * time.Millisecond)
	}
	cancel()
	<-done
	if w := call(s, "GET", Prefix+"/metadata", "", ""); w.Code != 200 {
		t.Fatal(w.Code, w.Body)
	}
	l.MetadataQueue().Pause(ctx, true)
	body, _ := json.Marshal(map[string]any{"retry": false, "force": true, "extractor": "media", "request_id": uuid.NewString()})
	path := Prefix + "/files/" + id.String() + "/metadata"
	w := call(s, "POST", path, "", string(body))
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body)
	}
	var job library.OCRJob
	if err = json.Unmarshal(w.Body.Bytes(), &job); err != nil || job.State != "queued" || job.Extractor != "media" {
		t.Fatal(w.Body, err)
	}
	if err = l.MetadataQueue().Step(ctx); err != nil {
		t.Fatal(err)
	}
	if w = call(s, "POST", path, "", string(body)); w.Code != 200 {
		t.Fatal("refresh retry lost idempotency", w.Code, w.Body)
	}
}
