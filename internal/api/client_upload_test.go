package api

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"testing"

	"github.com/jinzhao1994/kist/internal/library"
	staging "github.com/jinzhao1994/kist/internal/upload"
	"github.com/jinzhao1994/kist/internal/wire"
)

func TestClientUploadResumesAndRecoversLostCommitResponse(t *testing.T) {
	_, nas := testServer(t)
	head, err := nas.SyncHead(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	c, err := library.CreateClient(t.Context(), root, library.ClientSettings{LibraryID: head.LibraryID})
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	s, err := NewClient(c, Options{AllowedHosts: []string{"localhost:8473"}})
	if err != nil {
		t.Fatal(err)
	}
	mustWrite(t, s, "POST", "/boxes", "create-offline-upload-box", `{"name":"Offline"}`)
	boxes := decodeBody[wire.BoxList](t, s, "/boxes")
	box := boxes.Boxes[0]
	folders := decodeBody[folderList](t, s, "/boxes/"+box.ID+"/folders")
	raw, _ := json.Marshal(map[string]any{"version": 2, "box": box.ID, "folder": folders.Folders[0].ID, "files": []staging.File{{Path: "folder/file.txt", Size: 8}}})
	state := decodeSession(t, call(s, "POST", Prefix+"/upload-sessions", "offline-upload-001", string(raw)))
	id := state.Session.ID
	decodeSession(t, sessionChunk(s, id, 0, "abcd"))
	other, err := library.OpenClient(t.Context(), root)
	if err != nil {
		t.Fatal(err)
	}
	defer other.Close()
	reopened, err := NewClient(other, Options{AllowedHosts: []string{"localhost:8473"}})
	if err != nil {
		t.Fatal(err)
	}
	got := decodeSession(t, call(reopened, "GET", sessionURL(id), "", ""))
	if got.Session.Offsets[0] != 4 {
		t.Fatal(got)
	}
	decodeSession(t, sessionChunk(reopened, id, 4, "efgh"))
	reopened.uploadFault = func(point string) error {
		if point == "after_commit" {
			return errors.New("lost response")
		}
		return nil
	}
	finish := fmt.Sprintf(`{"digests":["%x"]}`, sha256.Sum256([]byte("abcdefgh")))
	recovered := decodeSession(t, call(reopened, "POST", sessionURL(id)+"/complete", "", finish))
	if recovered.LocalID == "" {
		t.Fatal("failed to recover durable receipt", recovered)
	}
	reopened.uploadFault = nil
	receipt := decodeSession(t, call(reopened, "POST", sessionURL(id)+"/complete", "", finish))
	if receipt.LocalID != recovered.LocalID || receipt.OpID != 0 || receipt.Noop {
		t.Fatal(receipt)
	}
	retry := decodeSession(t, call(reopened, "POST", Prefix+"/upload-sessions", "offline-upload-001", string(raw)))
	if retry.LocalID != receipt.LocalID {
		t.Fatal(retry, receipt)
	}
	files := decodeBody[wire.FileList](t, reopened, "/files")
	if len(files.Files) != 1 {
		t.Fatal(files)
	}
	content := call(reopened, "GET", Prefix+"/files/"+files.Files[0].ID+"/content", "", "")
	if content.Code != 200 || content.Body.String() != "abcdefgh" {
		t.Fatal(content.Code, content.Body)
	}
	count, err := nas.OpCount(t.Context())
	if err != nil || count != 0 {
		t.Fatal("offline upload touched NAS", count, err)
	}
}
