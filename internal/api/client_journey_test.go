package api

import (
	"bytes"
	"errors"
	"image"
	"image/color"
	"image/png"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"sync/atomic"
	"testing"

	"github.com/jinzhao1994/kist/internal/config"
	"github.com/jinzhao1994/kist/internal/domain"
	"github.com/jinzhao1994/kist/internal/library"
	"github.com/jinzhao1994/kist/internal/query"
	"github.com/jinzhao1994/kist/internal/syncclient"
)

// Exercise the travel workflow across the real HTTP protocol, including the
// interaction between newly assigned tags, nested filesets and replay.
func TestClientHTTPTravelMergeAndReplay(t *testing.T) {
	ctx := t.Context()
	root := t.TempDir()
	if _, err := library.Create(ctx, root); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Load(filepath.Join(root, "kist.toml"))
	if err != nil {
		t.Fatal(err)
	}
	cfg.Library.Timezone = "UTC"
	if err = config.Save(filepath.Join(root, "kist.toml"), cfg); err != nil {
		t.Fatal(err)
	}
	nas, err := library.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	defer nas.Close()
	box, err := nas.CreateBox(ctx, "Trip", "", domain.LocalDate{}, domain.LocalDate{})
	if err != nil {
		t.Fatal(err)
	}
	old := filepath.Join(t.TempDir(), "left-at-home.txt")
	if err = os.WriteFile(old, []byte("not downloaded"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err = nas.Import(ctx, old, library.ImportOptions{Box: box.ID}); err != nil {
		t.Fatal(err)
	}
	cachedPath := filepath.Join(t.TempDir(), "take-along.txt")
	if err = os.WriteFile(cachedPath, []byte("available offline"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err = nas.Import(ctx, cachedPath, library.ImportOptions{Box: box.ID}); err != nil {
		t.Fatal(err)
	}
	credential, err := nas.AuthorizeSync(ctx)
	if err != nil {
		t.Fatal(err)
	}
	remote := httptest.NewUnstartedServer(nil)
	handler, err := New(nas, Options{AllowedHosts: []string{remote.Listener.Addr().String()}})
	if err != nil {
		t.Fatal(err)
	}
	var disconnected atomic.Bool
	remote.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if disconnected.Load() {
			http.Error(w, "NAS unreachable", http.StatusServiceUnavailable)
			return
		}
		handler.ServeHTTP(w, r)
	})
	remote.Start()
	defer remote.Close()
	peer, err := syncclient.New(remote.URL, credential.Token, syncclient.Options{})
	if err != nil {
		t.Fatal(err)
	}
	head, err := peer.Head(ctx)
	if err != nil {
		t.Fatal(err)
	}
	clientRoot := t.TempDir()
	client, err := library.CreateClient(ctx, clientRoot, library.ClientSettings{LibraryID: head.LibraryID})
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if client != nil {
			client.Close()
		}
	}()
	if _, err = client.Synchronize(ctx, peer); err != nil {
		t.Fatal(err)
	}
	files, _, err := client.Files(ctx, query.Query{})
	if err != nil || len(files) != 2 {
		t.Fatal(files, err)
	}
	var cachedKey, uncachedKey string
	for _, f := range files {
		if f.Name == "take-along.txt" {
			cachedKey = f.BlobKey
		} else {
			uncachedKey = f.BlobKey
		}
	}
	if err = client.CacheOriginal(ctx, cachedKey, peer); err != nil {
		t.Fatal(err)
	}
	if reader, err := client.OpenOriginal(ctx, uncachedKey); !errors.Is(err, library.ErrNotCached) {
		if reader != nil {
			reader.Close()
		}
		t.Fatal("catalog synchronization downloaded an unselected original", err)
	}
	disconnected.Store(true)
	if _, err = client.Synchronize(ctx, peer); err == nil {
		t.Fatal("unreachable NAS reported sync success")
	}
	reader, err := client.OpenOriginal(ctx, cachedKey)
	if err != nil {
		t.Fatal(err)
	}
	content, err := io.ReadAll(reader)
	reader.Close()
	if err != nil || string(content) != "available offline" {
		t.Fatal(string(content), err)
	}
	source := t.TempDir()
	for i, name := range []string{"pages/first.png", "second.png"} {
		picture := image.NewNRGBA(image.Rect(0, 0, 2, 2))
		picture.SetNRGBA(0, 0, color.NRGBA{R: uint8(100 + i), A: 255})
		var encoded bytes.Buffer
		if err = png.Encode(&encoded, picture); err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(source, "document", name)
		if err = os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			t.Fatal(err)
		}
		if err = os.WriteFile(path, encoded.Bytes(), 0600); err != nil {
			t.Fatal(err)
		}
	}
	if _, err = client.Import(ctx, source, library.ClientImportOptions{Box: box.ID}); err != nil {
		t.Fatal(err)
	}
	folders, err := client.Folders(ctx, box.ID)
	if err != nil {
		t.Fatal(err)
	}
	folder := map[string]domain.FolderID{}
	for _, f := range folders {
		folder[f.Name] = f.ID
	}
	files, _, err = client.Files(ctx, query.Query{})
	if err != nil || len(files) != 4 {
		t.Fatal(files, err)
	}
	file := map[string]domain.FileID{}
	for _, f := range files {
		file[f.Name] = f.ID
	}
	tag, err := client.CreateTag(ctx, "Travel photos", nil)
	if err != nil {
		t.Fatal(err)
	}
	if n, err := client.TagFiles(ctx, tag.ID, []domain.FileID{file["first.png"], file["second.png"]}); err != nil || n != 2 {
		t.Fatal(n, err)
	}
	sets := []domain.Fileset{
		{FolderID: folder["pages"], Kind: "document_pages", Layout: "directory", Members: []domain.FilesetMember{{FileID: file["first.png"], Role: "page"}}},
		{FolderID: folder["document"], Kind: "document_pages", Layout: "directory", Primary: file["second.png"], Members: []domain.FilesetMember{{FolderID: folder["pages"], Role: "page"}, {FileID: file["second.png"], Role: "page"}}},
	}
	for _, set := range sets {
		if _, err = client.SetFileset(ctx, box.ID, set.FolderID, nil, &set); err != nil {
			t.Fatal(err)
		}
	}
	// NAS remains independently writable during the laptop's disconnection.
	if _, err = nas.UpdateBox(ctx, box.ID, library.BoxFields{Name: box.Name, Description: "NAS note"}); err != nil {
		t.Fatal(err)
	}
	if err = client.Close(); err != nil {
		t.Fatal(err)
	}
	client, err = library.OpenClient(ctx, clientRoot)
	if err != nil {
		t.Fatal(err)
	}
	if got, err := client.Filesets(ctx); err != nil || len(got) != 2 {
		t.Fatal("offline restart lost filesets", got, err)
	}
	disconnected.Store(false)
	result, err := client.Synchronize(ctx, peer)
	if err != nil || !result.Complete || result.Accepted == 0 {
		t.Fatal(result, err)
	}
	count, err := nas.OpCount(ctx)
	if err != nil {
		t.Fatal(err)
	}
	result, err = client.Synchronize(ctx, peer)
	if err != nil || !result.Complete || result.Accepted != 0 {
		t.Fatal(result, err)
	}
	if after, err := nas.OpCount(ctx); err != nil || after != count {
		t.Fatal("duplicate sync appended operations", after, count, err)
	}
	assertState := func() {
		t.Helper()
		got, found, err := nas.Box(ctx, box.ID)
		if err != nil || !found || got.Description != "NAS note" {
			t.Fatal(got, err)
		}
		remoteSets, err := nas.Filesets(ctx)
		if err != nil {
			t.Fatal(err)
		}
		localSets, err := client.Filesets(ctx)
		if err != nil || !reflect.DeepEqual(remoteSets, localSets) || len(remoteSets) != 2 {
			t.Fatal(remoteSets, localSets, err)
		}
		for _, expected := range sets {
			var actual *domain.Fileset
			for i := range remoteSets {
				if remoteSets[i].FolderID == expected.FolderID {
					actual = &remoteSets[i]
				}
			}
			if actual == nil || actual.Kind != expected.Kind || actual.Layout != expected.Layout || actual.Primary != expected.Primary || !reflect.DeepEqual(actual.Members, expected.Members) {
				t.Fatal("nested page configuration changed", actual, expected)
			}
		}
		for _, id := range []domain.FileID{file["first.png"], file["second.png"]} {
			tags, err := nas.FileTags(ctx, id)
			if err != nil || len(tags) != 1 || tags[0] != tag.ID {
				t.Fatal(tags, err)
			}
			_, original, err := nas.OpenContent(ctx, id)
			if err != nil {
				t.Fatal(err)
			}
			_, err = png.Decode(original)
			original.Close()
			if err != nil {
				t.Fatal("synced photo is not intact", err)
			}
		}
	}
	assertState()
	if err = nas.Rebuild(ctx); err != nil {
		t.Fatal(err)
	}
	assertState()
	report, err := nas.Verify(ctx, library.VerifyOptions{Deep: true})
	if err != nil || report.Total() != 0 || report.Checked["tables replayed"] == 0 {
		t.Fatal(report, err)
	}
	if err = nas.ViewAt(ctx, count, func(view *library.HistoricalView) error {
		tags, err := view.FileTags(ctx, file["first.png"])
		if err != nil {
			return err
		}
		if len(tags) != 1 || tags[0] != tag.ID {
			t.Fatal("historical snapshot lost tag", tags)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}
