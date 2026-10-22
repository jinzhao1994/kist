package library

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jinzhao1994/kist/internal/integrity"
)

func TestSyncAppendCrashProcess(t *testing.T) {
	root := os.Getenv("KIST_TEST_SYNC_APPEND_CRASH")
	if root == "" {
		return
	}
	nas, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	phase := os.Getenv("KIST_TEST_SYNC_CRASH_PHASE")
	if phase != "append" {
		content, key := syncCrashOriginal()
		var source io.Reader = bytes.NewReader(content)
		if phase == "original-copy" {
			source = &syncCrashReader{reader: source, left: len(content) / 2}
		}
		if err = nas.StoreSyncOriginal(context.Background(), key, int64(len(content)), source); err != nil {
			t.Fatal(err)
		}
		t.Fatal(syncCrashCheckpoint())
	}
	raw, err := os.ReadFile(os.Getenv("KIST_TEST_SYNC_CRASH_PROPOSAL"))
	if err != nil {
		t.Fatal(err)
	}
	var proposal SyncProposal
	if err = json.Unmarshal(raw, &proposal); err != nil {
		t.Fatal(err)
	}
	nas.hooks.afterAppend = syncCrashCheckpoint
	// Do not defer Close: the parent must exercise abrupt process death while
	// the write lock is held and SQLite still trails the durable journal.
	_, err = nas.AcceptSync(context.Background(), proposal)
	t.Fatal("expected process kill", err)
}

// Keep Buffer named: embedding ReaderFrom lets exec bypass Write and its signal.
type syncAppendOutput struct {
	buffer bytes.Buffer
	ready  chan struct{}
	once   sync.Once
}

func (w *syncAppendOutput) String() string { return w.buffer.String() }

func (w *syncAppendOutput) Write(p []byte) (int, error) {
	n, err := w.buffer.Write(p)
	if strings.Contains(w.String(), "SYNC-CHECKPOINT\n") {
		w.once.Do(func() { close(w.ready) })
	}
	return n, err
}

func TestSyncNASProcessKillAfterDurableAppend(t *testing.T) {
	nas := openTestLibrary(t)
	root := nas.Layout().Root
	if err := nas.Close(); err != nil {
		t.Fatal(err)
	}
	proposal := syncBox(t, "nas-crash-before-apply")
	raw, err := json.Marshal(proposal)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "proposal.json")
	if err = os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	killSyncProcessAtCheckpoint(t, root, path, "append")
	nas, err = Open(root)
	if err != nil {
		t.Fatal(err)
	}
	defer nas.Close()
	ctx := t.Context()
	if count, err := nas.OpCount(ctx); err != nil || count != 1 {
		t.Fatal("journal not recovered", count, err)
	}
	receipt, err := nas.AcceptSync(ctx, proposal)
	if err != nil || receipt.OpID != 1 || receipt.AcceptedAt.IsZero() {
		t.Fatal(receipt, err)
	}
	history, err := nas.HistoryOp(ctx, receipt.OpID)
	if err != nil || !history.TS.Equal(proposal.OccurredAt) || history.Origin != "user" {
		t.Fatal(history, err)
	}
	before := snapshotOf(t, nas)
	if err = nas.Rebuild(ctx); err != nil {
		t.Fatal(err)
	}
	if got := snapshotOf(t, nas); got != before {
		t.Fatal("rebuild changed recovered state")
	}
	retried, err := nas.AcceptSync(ctx, proposal)
	if err != nil || retried.OpID != receipt.OpID || !retried.AcceptedAt.Equal(receipt.AcceptedAt) {
		t.Fatal(retried, err)
	}
	report, err := nas.Verify(ctx, VerifyOptions{Deep: true})
	if err != nil || report.Total() != report.Count(integrity.LinkMissing) || report.Count(integrity.LinkMissing) != 1 || report.Checked["tables replayed"] == 0 {
		t.Fatal("expected only a recoverable projection gap", report, err)
	}
	if _, err = nas.Relink(ctx); err != nil {
		t.Fatal(err)
	}
	report, err = nas.Verify(ctx, VerifyOptions{Deep: true})
	if err != nil || report.Total() != 0 {
		t.Fatal(report, err)
	}
}

func killSyncProcessAtCheckpoint(t *testing.T, root, path, phase string) {
	t.Helper()
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(executable, "-test.run=^TestSyncAppendCrashProcess$")
	cmd.Env = append(os.Environ(), "KIST_TEST_SYNC_APPEND_CRASH="+root, "KIST_TEST_SYNC_CRASH_PROPOSAL="+path, "KIST_TEST_SYNC_CRASH_PHASE="+phase)
	output := &syncAppendOutput{ready: make(chan struct{})}
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
	}()
	select {
	case <-output.ready:
	case err := <-done:
		joined = true
		t.Fatalf("helper exited before checkpoint: %v\n%s\n%s", err, output.String(), stderr.String())
	case <-time.After(60 * time.Second):
		t.Fatal("helper did not reach checkpoint")
	}
	if err = cmd.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	err = <-done
	joined = true
	if err == nil {
		t.Fatal("helper exited normally instead of being killed")
	}
}

func syncCrashCheckpoint() error {
	fmt.Fprintln(os.Stdout, "SYNC-CHECKPOINT")
	_, _ = os.Stdin.Read(make([]byte, 1))
	return fmt.Errorf("parent should kill the process before releasing stdin")
}

func syncCrashOriginal() ([]byte, string) {
	content := bytes.Repeat([]byte("original"), 16<<10)
	return content, fmt.Sprintf("sha256:%x", sha256.Sum256(content))
}

type syncCrashReader struct {
	reader io.Reader
	left   int
}

func (r *syncCrashReader) Read(p []byte) (int, error) {
	if r.left == 0 {
		return 0, syncCrashCheckpoint()
	}
	n, err := r.reader.Read(p[:min(len(p), r.left)])
	r.left -= n
	return n, err
}

func TestSyncNASProcessKillDuringOriginalPlacement(t *testing.T) {
	for _, phase := range []string{"original-copy", "original-installed"} {
		t.Run(phase, func(t *testing.T) {
			nas := openTestLibrary(t)
			root := nas.Layout().Root
			if err := nas.Close(); err != nil {
				t.Fatal(err)
			}
			killSyncProcessAtCheckpoint(t, root, "", phase)
			nas, err := Open(root)
			if err != nil {
				t.Fatal(err)
			}
			defer nas.Close()
			ctx := t.Context()
			content, key := syncCrashOriginal()
			status, err := nas.SyncOriginal(ctx, key)
			if err != nil || status.Present != (phase == "original-installed") {
				t.Fatal("partial original exposed", status, err)
			}
			for range 2 {
				if err = nas.StoreSyncOriginal(ctx, key, int64(len(content)), bytes.NewReader(content)); err != nil {
					t.Fatal(err)
				}
			}
			reader, err := nas.OpenSyncOriginal(ctx, key)
			if err != nil {
				t.Fatal(err)
			}
			actual, err := io.ReadAll(reader)
			reader.Close()
			if err != nil || !bytes.Equal(actual, content) {
				t.Fatal("placement retry changed content", err)
			}
			if count, err := nas.OpCount(ctx); err != nil || count != 0 {
				t.Fatal("placement created archive history", count, err)
			}
		})
	}
}
