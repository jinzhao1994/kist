package syncclient_test

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jinzhao1994/kist/internal/api"
	"github.com/jinzhao1994/kist/internal/config"
	"github.com/jinzhao1994/kist/internal/domain"
	"github.com/jinzhao1994/kist/internal/library"
	"github.com/jinzhao1994/kist/internal/oplog"
	"github.com/jinzhao1994/kist/internal/syncclient"
)

// The helper intentionally never closes its client after successful sync. The
// parent kills the process, so recovery cannot depend on deferred cleanup.
func TestSyncCrashClientProcess(t *testing.T) {
	root := os.Getenv("KIST_TEST_SYNC_CRASH_CLIENT")
	if root == "" {
		return
	}
	ctx := context.Background()
	c, err := library.OpenClient(ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	connection, token, err := c.Connection(ctx)
	if err != nil {
		t.Fatal(err)
	}
	peer, err := syncclient.New(connection.Server, token, syncclient.Options{AllowHTTP: connection.AllowHTTP})
	if err != nil {
		t.Fatal(err)
	}
	if result, err := c.Synchronize(ctx, peer); err != nil || !result.Complete {
		t.Fatal(result, err)
	}
	fmt.Fprintln(os.Stdout, "SYNC-DONE")
	_, _ = os.Stdin.Read(make([]byte, 1))
	t.Fatal("parent should kill the process before releasing stdin")
}

// Keep Buffer named: embedding ReaderFrom lets exec bypass Write and its signal.
type syncCrashOutput struct {
	buffer bytes.Buffer
	once   sync.Once
	ready  chan struct{}
}

func (w *syncCrashOutput) String() string { return w.buffer.String() }

func (w *syncCrashOutput) Write(p []byte) (int, error) {
	n, err := w.buffer.Write(p)
	if strings.Contains(w.String(), "SYNC-DONE\n") {
		w.once.Do(func() { close(w.ready) })
	}
	return n, err
}

func TestHTTPClientRecoversAfterProcessKill(t *testing.T) {
	for _, phase := range []string{"upload-block", "original-installed", "nas-committed", "local-confirmed"} {
		t.Run(phase, func(t *testing.T) {
			ctx := t.Context()
			root := filepath.Join(t.TempDir(), "nas")
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
			box, err := nas.CreateBox(ctx, "Travel", "", domain.LocalDate{}, domain.LocalDate{})
			if err != nil {
				t.Fatal(err)
			}
			folders, err := nas.Folders(ctx, box.ID)
			if err != nil {
				t.Fatal(err)
			}
			var folder domain.FolderID
			for _, f := range folders {
				if f.ParentID == nil {
					folder = f.ID
				}
			}
			credential, err := nas.AuthorizeSync(ctx)
			if err != nil {
				t.Fatal(err)
			}
			server := httptest.NewUnstartedServer(nil)
			handler, err := api.New(nas, api.Options{AllowedHosts: []string{server.Listener.Addr().String()}})
			if err != nil {
				t.Fatal(err)
			}
			reached, release := make(chan struct{}), make(chan struct{})
			var unblock sync.Once
			defer unblock.Do(func() { close(release) })
			var armed atomic.Bool
			var mu sync.Mutex
			var offsets []string
			server.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				block := r.Method == "PUT" && strings.Contains(r.URL.Path, "/original-uploads/")
				if block {
					mu.Lock()
					offsets = append(offsets, r.URL.Query().Get("offset"))
					mu.Unlock()
				}
				target := phase == "upload-block" && block || phase == "original-installed" && strings.HasSuffix(r.URL.Path, "/finish") || phase == "nas-committed" && strings.HasSuffix(r.URL.Path, "/submit")
				if !target || !armed.CompareAndSwap(true, false) {
					handler.ServeHTTP(w, r)
					return
				}
				recorder := httptest.NewRecorder()
				handler.ServeHTTP(recorder, r)
				if recorder.Code != 200 {
					t.Errorf("checkpoint failed: %d %s", recorder.Code, recorder.Body)
					return
				}
				// The real NAS has durably completed the operation, but no reply has
				// reached the client. Kill it while its library and sync lock are live.
				close(reached)
				select {
				case <-release:
				case <-r.Context().Done():
				}
			})
			server.Start()
			defer server.Close()
			peer, err := syncclient.New(server.URL, credential.Token, syncclient.Options{})
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
			if err = client.SetConnection(ctx, head.LibraryID, library.ClientConnection{Server: server.URL}, credential.Token); err != nil {
				t.Fatal(err)
			}
			if _, err = client.Synchronize(ctx, peer); err != nil {
				t.Fatal(err)
			}
			content := bytes.Repeat([]byte("trip"), (9<<20)/4)
			stored, err := client.PutOriginal(ctx, bytes.NewReader(content))
			if err != nil {
				t.Fatal(err)
			}
			fileID := domain.NewFileID()
			raw, err := oplog.Marshal(oplog.ImportFilesPayload{BoxID: box.ID, Files: []oplog.ImportedFile{{FileID: fileID, FolderID: folder, Name: "trip.bin", BlobKey: string(stored.Key), Sha256: stored.Key.Hex(), Size: stored.Size}}})
			if err != nil {
				t.Fatal(err)
			}
			proposal := library.SyncProposal{Version: 1, ID: "process-crash-import", OccurredAt: time.Now().UTC().Add(-48 * time.Hour), Action: string(oplog.ActionImportFiles), Origin: "user", Payload: raw}
			if err = client.Stage(ctx, proposal); err != nil {
				t.Fatal(err)
			}
			if err = client.Close(); err != nil {
				t.Fatal(err)
			}
			armed.Store(true)
			executable, err := os.Executable()
			if err != nil {
				t.Fatal(err)
			}
			cmd := exec.Command(executable, "-test.run=^TestSyncCrashClientProcess$")
			cmd.Env = append(os.Environ(), "KIST_TEST_SYNC_CRASH_CLIENT="+clientRoot)
			output := &syncCrashOutput{ready: make(chan struct{})}
			var stderr bytes.Buffer
			cmd.Stdout, cmd.Stderr = output, &stderr
			input, err := cmd.StdinPipe()
			if err != nil {
				t.Fatal(err)
			}
			defer input.Close()
			if err = cmd.Start(); err != nil {
				t.Fatal(err)
			}
			done := make(chan error, 1)
			go func() { done <- cmd.Wait() }()
			joined := false
			defer func() {
				if !joined {
					_ = cmd.Process.Kill()
					<-done
				}
				unblock.Do(func() { close(release) })
			}()
			boundary := reached
			if phase == "local-confirmed" {
				boundary = output.ready
			}
			select {
			case <-boundary:
			case err := <-done:
				joined = true
				t.Fatalf("helper exited before checkpoint: %v\n%s\n%s", err, output.String(), stderr.String())
			case <-time.After(60 * time.Second):
				t.Fatal("helper did not reach durable checkpoint")
			}
			if err = cmd.Process.Kill(); err != nil {
				t.Fatal(err)
			}
			err = <-done
			joined = true
			if err == nil {
				t.Fatal("helper exited normally instead of being killed")
			}
			unblock.Do(func() { close(release) })
			client, err = library.OpenClient(ctx, clientRoot)
			if err != nil {
				t.Fatal(err)
			}
			defer client.Close()
			for range 2 {
				result, err := client.Synchronize(ctx, peer)
				if err != nil || !result.Complete {
					t.Fatal("restart did not converge", result, err)
				}
			}
			mu.Lock()
			gotOffsets := strings.Join(offsets, ",")
			mu.Unlock()
			if gotOffsets != "0,8388608" {
				t.Fatalf("durable block retransmitted: %s", gotOffsets)
			}
			history, err := client.LocalHistoryOp(ctx, proposal.ID)
			if err != nil || history.State != "accepted" || history.Receipt == nil || history.Receipt.OpID != 2 || !history.OccurredAt.Equal(proposal.OccurredAt) {
				t.Fatal(history, err)
			}
			if count, err := nas.OpCount(ctx); err != nil || count != 2 {
				t.Fatal("duplicate formal intent", count, err)
			}
			reader, err := nas.OpenSyncOriginal(ctx, string(stored.Key))
			if err != nil {
				t.Fatal(err)
			}
			actual, err := io.ReadAll(reader)
			reader.Close()
			if err != nil || !bytes.Equal(actual, content) {
				t.Fatal("original changed across process death", err)
			}
			if err = nas.Rebuild(ctx); err != nil {
				t.Fatal(err)
			}
			if result, err := client.Synchronize(ctx, peer); err != nil || !result.Complete || result.Accepted != 0 {
				t.Fatal(result, err)
			}
		})
	}
}
