package api

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/jinzhao1994/kist/internal/domain"
	"github.com/jinzhao1994/kist/internal/library"
	"github.com/jinzhao1994/kist/internal/query"
	staging "github.com/jinzhao1994/kist/internal/upload"
)

func TestUploadConflictsRequireConfirmationAndPreserveBothChoices(t *testing.T) {
	for _, mode := range []string{"overwrite", "keep_both"} {
		t.Run(mode, func(t *testing.T) {
			s, l := testServer(t)
			ctx := t.Context()
			box, first, _ := sessionFixture(t, s, l, "old")
			decodeSession(t, sessionChunk(s, first.Session.ID, 0, "old"))
			decodeSession(t, call(s, "POST", sessionURL(first.Session.ID)+"/complete", "", `{}`))
			original, _, err := l.Files(ctx, query.Query{Boxes: []domain.BoxID{box.ID}})
			if err != nil {
				t.Fatal(err)
			}
			body := map[string]any{"version": 2, "box": box.ID.String(), "folder": box.RootFolderID.String(), "files": []staging.File{{Path: "folder/file.bin", Size: 3, LastModified: 1000}}}
			create := func(key string) sessionStatus {
				raw, _ := json.Marshal(body)
				return decodeSession(t, call(s, "POST", Prefix+"/upload-sessions", key, string(raw)))
			}
			preview := create("conflicts-preview")
			if preview.Session != nil || preview.Conflicts == nil || len(preview.Conflicts.Paths) != 1 {
				t.Fatalf("no conflict prompt: %+v", preview)
			}
			body["conflict_mode"] = mode
			body["conflict_token"] = preview.Conflicts.Token
			accepted := create("conflicts-confirmed")
			if accepted.Session == nil {
				t.Fatal(accepted)
			}
			// Retry creation recovers the exact same session and confirmation.
			if again := create("conflicts-confirmed"); again.Session.ID != accepted.Session.ID {
				t.Fatal(again)
			}
			decodeSession(t, sessionChunk(s, accepted.Session.ID, 0, "new"))
			final := fmt.Sprintf(`{"digests":["%x"]}`, sha256.Sum256([]byte("new")))
			done := decodeSession(t, call(s, "POST", sessionURL(accepted.Session.ID)+"/complete", "", final))
			if done.OpID == 0 {
				t.Fatal(done)
			}
			got, _, err := l.Files(ctx, query.Query{Boxes: []domain.BoxID{box.ID}})
			if err != nil {
				t.Fatal(err)
			}
			if mode == "overwrite" {
				if len(got) != 1 || got[0].ID != original[0].ID || got[0].BlobKey == original[0].BlobKey {
					t.Fatal(got)
				}
			} else if len(got) != 2 {
				t.Fatal(got)
			}
			// The same request is a receipt after commit, not another replacement.
			if again := create("conflicts-confirmed"); again.OpID != done.OpID {
				t.Fatal(again)
			}
			if _, err := l.Undo(ctx, done.OpID); err != nil {
				t.Fatal(err)
			}
			report, err := l.Verify(ctx, library.VerifyOptions{Deep: true})
			if err != nil || report.Total() != 0 {
				t.Fatalf("verify %+v %v", report, err)
			}
		})
	}
}
func TestUploadRejectsDestinationChangeAfterConfirmation(t *testing.T) {
	s, l := testServer(t)
	ctx := t.Context()
	box, first, body := sessionFixture(t, s, l, "old")
	decodeSession(t, sessionChunk(s, first.Session.ID, 0, "old"))
	decodeSession(t, call(s, "POST", sessionURL(first.Session.ID)+"/complete", "", `{}`))
	preview := decodeSession(t, call(s, "POST", Prefix+"/upload-sessions", "race-preview", body))
	var input map[string]any
	_ = json.Unmarshal([]byte(body), &input)
	input["conflict_mode"] = "overwrite"
	input["conflict_token"] = preview.Conflicts.Token
	raw, _ := json.Marshal(input)
	accepted := decodeSession(t, call(s, "POST", Prefix+"/upload-sessions", "race-confirmed", string(raw)))
	files, _, _ := l.Files(ctx, query.Query{Boxes: []domain.BoxID{box.ID}})
	if _, err := l.RenameEntry(ctx, box.ID, files[0].ID.ID, false, files[0].Name, "changed.bin"); err != nil {
		t.Fatal(err)
	}
	decodeSession(t, sessionChunk(s, accepted.Session.ID, 0, "old"))
	assertAPIError(t, call(s, "POST", sessionURL(accepted.Session.ID)+"/complete", "", `{}`), 409, "upload_target_changed")
}
