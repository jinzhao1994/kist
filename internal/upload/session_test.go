package upload

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/jinzhao1994/kist/internal/lockfile"
)

func digest(b []byte) string { return fmt.Sprintf("%x", sha256.Sum256(b)) }
func manifest(data []byte) Manifest {
	return Manifest{Box: "box", Folder: "folder", Target: digest([]byte("target")), Files: []File{{Path: "dir/original.bin", LastModified: 123456789, Size: int64(len(data)), SHA256: digest(data)}}}
}
func newStore(t *testing.T, path string, limit int64) *Store {
	t.Helper()
	s, err := New(path, limit, 1000)
	if err != nil {
		t.Fatal(err)
	}
	return s
}
func create(t *testing.T, s *Store, key string, m Manifest) State {
	t.Helper()
	v, err := s.Create(t.Context(), key, m)
	if err != nil {
		t.Fatal(err)
	}
	return v
}
func checkKind(t *testing.T, err error, kind string) {
	t.Helper()
	var p *Problem
	if !errors.As(err, &p) || p.Kind != kind {
		t.Fatalf("want %s, got %v", kind, err)
	}
}
func progress(t *testing.T, s *Store, id string) State {
	t.Helper()
	var state State
	if err := s.With(t.Context(), id, func(session *Session) error { state = session.State(); return nil }); err != nil {
		t.Fatal(err)
	}
	return state
}
func appendBlock(s *Store, ctx context.Context, id string, offset int64, data []byte) error {
	return s.With(ctx, id, func(session *Session) error { return session.Append(ctx, 0, offset, data, digest(data)) })
}

func TestResumeOnlyMissingBytes(t *testing.T) {
	path := t.TempDir()
	s := newStore(t, path, 100)
	data := []byte("abcdefghij")
	state := create(t, s, "request", manifest(data))
	if err := appendBlock(s, t.Context(), state.ID, 0, data[:4]); err != nil {
		t.Fatal(err)
	}
	// A new Store models a new server: no in-memory offset survives.
	s = newStore(t, path, 100)
	state = progress(t, s, state.ID)
	if state.Offsets[0] != 4 {
		t.Fatal(state)
	}
	missing := data[state.Offsets[0]:]
	if len(missing) != 6 {
		t.Fatal("resent confirmed bytes")
	}
	if err := appendBlock(s, t.Context(), state.ID, state.Offsets[0], missing); err != nil {
		t.Fatal(err)
	}
	if err := s.With(t.Context(), state.ID, func(session *Session) error {
		paths, err := session.Verify(t.Context())
		if err != nil {
			return err
		}
		got, err := os.ReadFile(paths[0])
		if err != nil {
			return err
		}
		if string(got) != string(data) {
			t.Fatalf("actual bytes: %q", got)
		}
		info, err := os.Stat(paths[0])
		if err != nil {
			return err
		}
		if info.ModTime().UnixMilli() != 123456789 {
			t.Fatal(info.ModTime())
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

func TestChunksRejectBadDataAndOffsets(t *testing.T) {
	s := newStore(t, t.TempDir(), 100)
	state := create(t, s, "request", manifest([]byte("abcdefgh")))
	err := s.With(t.Context(), state.ID, func(session *Session) error {
		return session.Append(t.Context(), 0, 0, []byte("wrong"), digest([]byte("right")))
	})
	checkKind(t, err, "chunk_integrity")
	if progress(t, s, state.ID).Offsets[0] != 0 {
		t.Fatal("bad block acknowledged")
	}
	checkKind(t, appendBlock(s, t.Context(), state.ID, 1, []byte("abc")), "offset")
	if err := appendBlock(s, t.Context(), state.ID, 0, []byte("abcd")); err != nil {
		t.Fatal(err)
	}
	if err := appendBlock(s, t.Context(), state.ID, 0, []byte("abcd")); err != nil {
		t.Fatal("response-loss retry:", err)
	}
	checkKind(t, appendBlock(s, t.Context(), state.ID, 0, []byte("xxxx")), "chunk_integrity")
	checkKind(t, appendBlock(s, t.Context(), state.ID, 2, []byte("cdef")), "offset")
	checkKind(t, appendBlock(s, t.Context(), state.ID, 4, []byte("efghi")), "offset")
	if got := progress(t, s, state.ID).Offsets[0]; got != 4 {
		t.Fatal(got)
	}
}

func TestCrashHelper(t *testing.T) {
	if os.Getenv("KIST_UPLOAD_CRASH_HELPER") != "1" {
		return
	}
	s, err := New(os.Getenv("KIST_UPLOAD_CRASH_ROOT"), 100, 1000)
	if err != nil {
		t.Fatal(err)
	}
	point := os.Getenv("KIST_UPLOAD_CRASH_POINT")
	s.fault = func(at string) error {
		if at == point {
			os.Exit(71)
		}
		return nil
	}
	if point == "before_publish" {
		_, err := s.Create(context.Background(), "request", manifest([]byte("abcdefgh")))
		t.Fatalf("creation did not crash: %v", err)
	}
	if err := appendBlock(s, context.Background(), ID("request"), 0, []byte("abcd")); err != nil {
		t.Fatal(err)
	}
	t.Fatal("crash point not reached")
}

func TestProcessDeathAtDurabilityBoundaries(t *testing.T) {
	for _, point := range []string{"after_write", "after_data_sync", "before_progress", "after_progress"} {
		t.Run(point, func(t *testing.T) {
			path := t.TempDir()
			s := newStore(t, path, 100)
			state := create(t, s, "request", manifest([]byte("abcdefgh")))
			cmd := exec.Command(os.Args[0], "-test.run=^TestCrashHelper$")
			cmd.Env = append(os.Environ(), "KIST_UPLOAD_CRASH_HELPER=1", "KIST_UPLOAD_CRASH_ROOT="+path, "KIST_UPLOAD_CRASH_POINT="+point)
			out, err := cmd.CombinedOutput()
			var exit *exec.ExitError
			if !errors.As(err, &exit) || exit.ExitCode() != 71 {
				t.Fatalf("child did not reach crash: %v %s", err, out)
			}
			s = newStore(t, path, 100)
			state = progress(t, s, state.ID)
			want := int64(0)
			if point == "after_progress" {
				want = 4
			}
			if state.Offsets[0] != want {
				t.Fatalf("progress %v want %d", state.Offsets, want)
			}
			info, err := os.Stat(filepath.Join(path, state.ID, "0"))
			if err != nil {
				t.Fatal(err)
			}
			if info.Size() != want {
				t.Fatal("unconfirmed tail not truncated", info.Size())
			}
			// The kernel released the dead process's lock, and the next transfer
			// sends exactly the suffix, even when the last response was lost.
			if err := appendBlock(s, t.Context(), state.ID, want, []byte("abcdefgh")[want:]); err != nil {
				t.Fatal(err)
			}
			if err := s.With(t.Context(), state.ID, func(v *Session) error { _, err := v.Verify(t.Context()); return err }); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestMissingConfirmedDataAndWholeFileMismatch(t *testing.T) {
	for _, corrupt := range []bool{false, true} {
		t.Run(fmt.Sprint(corrupt), func(t *testing.T) {
			path := t.TempDir()
			s := newStore(t, path, 100)
			state := create(t, s, "request", manifest([]byte("good")))
			if err := appendBlock(s, t.Context(), state.ID, 0, []byte("good")); err != nil {
				t.Fatal(err)
			}
			data := []byte("evil")
			if !corrupt {
				data = []byte("go")
			}
			if err := os.WriteFile(filepath.Join(path, state.ID, "0"), data, 0600); err != nil {
				t.Fatal(err)
			}
			err := s.With(t.Context(), state.ID, func(v *Session) error { _, err := v.Verify(t.Context()); return err })
			kind := "corrupt"
			if corrupt {
				kind = "file_integrity"
			}
			checkKind(t, err, kind)
		})
	}
}

func TestWriteFailureKeepsEarlierProgress(t *testing.T) {
	s := newStore(t, t.TempDir(), 100)
	state := create(t, s, "request", manifest([]byte("abcdefgh")))
	if err := appendBlock(s, t.Context(), state.ID, 0, []byte("abcd")); err != nil {
		t.Fatal(err)
	}
	s.writeAt = func(f *os.File, data []byte, offset int64) (int, error) {
		n, err := f.WriteAt(data[:2], offset)
		if err != nil {
			return n, err
		}
		return n, syscall.ENOSPC
	}
	if err := appendBlock(s, t.Context(), state.ID, 4, []byte("efgh")); !errors.Is(err, syscall.ENOSPC) {
		t.Fatal(err)
	}
	s.writeAt = nil
	if got := progress(t, s, state.ID).Offsets[0]; got != 4 {
		t.Fatal(got)
	}
	if err := appendBlock(s, t.Context(), state.ID, 4, []byte("efgh")); err != nil {
		t.Fatal(err)
	}
}

func TestIndependentServersSerializeDuplicateBlocks(t *testing.T) {
	path := t.TempDir()
	a := newStore(t, path, 100)
	b := newStore(t, path, 100)
	state := create(t, a, "request", manifest([]byte("abcd")))
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()
	var wg sync.WaitGroup
	for i := 0; i < 12; i++ {
		wg.Go(func() {
			for {
				err := appendBlock(b, ctx, state.ID, 0, []byte("abcd"))
				if err == nil {
					return
				}
				// Busy is a retryable protocol response, not lost serialization.
				// Twelve fsync-heavy Windows requests need not all acquire the
				// lock inside a single five-second wait on a shared CI runner.
				var busy *lockfile.BusyError
				if !errors.As(err, &busy) || ctx.Err() != nil {
					t.Error(err)
					return
				}
			}
		})
	}
	wg.Wait()
	if got := progress(t, a, state.ID).Offsets[0]; got != 4 {
		t.Fatal(got)
	}
	info, err := os.Stat(filepath.Join(path, state.ID, "0"))
	if err != nil {
		t.Fatal(err)
	}
	if info.Size() != 4 {
		t.Fatal(info.Size())
	}
}

func TestQuotaExpiryCancelAndBusyCleanup(t *testing.T) {
	path := t.TempDir()
	s := newStore(t, path, 10)
	now := time.Now()
	s.now = func() time.Time { return now }
	a := create(t, s, "a", manifest([]byte("1234567890")))
	create(t, s, "b", manifest([]byte("1234567890")))
	_, err := s.Create(t.Context(), "c", manifest([]byte("x")))
	checkKind(t, err, "quota")
	// Queries do not renew the session.
	now = now.Add(IdleTTL - time.Second)
	if got := progress(t, s, a.ID).Expires; !got.Equal(a.Expires) {
		t.Fatal(got)
	}
	if err := s.With(t.Context(), a.ID, func(v *Session) error { return v.Renew() }); err != nil {
		t.Fatal(err)
	}
	now = now.Add(2 * time.Second)
	if err := s.Sweep(t.Context()); err != nil {
		t.Fatal(err)
	}
	create(t, s, "c", manifest([]byte("1234567890")))
	if err := s.With(t.Context(), a.ID, func(v *Session) error {
		// A separate server must skip this locked session, even if expired.
		other := newStore(t, path, 10)
		other.now = func() time.Time { return now.Add(IdleTTL) }
		if err := other.Sweep(t.Context()); err != nil {
			return err
		}
		if _, err := os.Stat(filepath.Join(path, a.ID, "state.json")); err != nil {
			return err
		}
		return v.Cancel()
	}); err != nil {
		t.Fatal(err)
	}
	checkKind(t, s.With(t.Context(), a.ID, func(*Session) error { return nil }), "canceled")
	if err := s.Sweep(t.Context()); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(path, a.ID)); !os.IsNotExist(err) {
		t.Fatal(err)
	}
	create(t, s, "d", manifest([]byte("1234567890")))
}

func TestManifestIsolationAndIdempotency(t *testing.T) {
	s := newStore(t, t.TempDir(), 100)
	m := manifest([]byte("abc"))
	a := create(t, s, "request", m)
	b := create(t, s, "request", m)
	if a.ID != b.ID {
		t.Fatal(a, b)
	}
	m.Files[0].Path = "changed"
	_, err := s.Create(t.Context(), "request", m)
	checkKind(t, err, "conflict")
	for _, path := range []string{"../escape", "/escape", "a\\b", "a/../../b", "a/./b", "a//b", "a/ "} {
		m.Files[0].Path = path
		if _, err := s.Create(t.Context(), path, m); err == nil {
			t.Fatalf("accepted %q", path)
		}
	}
	checkKind(t, s.With(t.Context(), "../outside", func(*Session) error { return nil }), "invalid_id")
	if err := s.With(t.Context(), a.ID, func(v *Session) error {
		copy := v.State()
		copy.Offsets[0] = 123
		copy.Manifest.Files[0].Path = "mutated"
		if v.State().Offsets[0] != 0 || v.State().Manifest.Files[0].Path != "dir/original.bin" {
			t.Fatal("snapshot leaked mutable storage")
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

func TestUnknownRecordAndSymlinkAreNotRecoveredAsEmpty(t *testing.T) {
	for _, mode := range []string{"version", "json", "symlink"} {
		t.Run(mode, func(t *testing.T) {
			path := t.TempDir()
			s := newStore(t, path, 100)
			a := create(t, s, "a", manifest([]byte("abc")))
			record := filepath.Join(path, a.ID, "state.json")
			if mode == "symlink" {
				dataPath := filepath.Join(path, a.ID, "0")
				if err := os.Remove(dataPath); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(record, dataPath); err != nil {
					t.Skip("symlinks unavailable:", err)
				}
			} else {
				raw, err := os.ReadFile(record)
				if err != nil {
					t.Fatal(err)
				}
				text := strings.Replace(string(raw), `"version":1`, `"version":99`, 1)
				if mode == "json" {
					text = "{"
				}
				if err := os.WriteFile(record, []byte(text), 0600); err != nil {
					t.Fatal(err)
				}
			}
			err := s.With(t.Context(), a.ID, func(*Session) error { return nil })
			kind := "corrupt"
			if mode == "version" {
				kind = "unsupported_version"
			}
			checkKind(t, err, kind)
		})
	}
}

func TestCleanupInterruptedBeforeAndAfterPublication(t *testing.T) {
	path := t.TempDir()
	s := newStore(t, path, 100)
	// Kill a creator before publication. Its directory must neither reserve
	// capacity forever nor become a readable, incomplete session.
	cmd := exec.Command(os.Args[0], "-test.run=^TestCrashHelper$")
	cmd.Env = append(os.Environ(), "KIST_UPLOAD_CRASH_HELPER=1", "KIST_UPLOAD_CRASH_ROOT="+path, "KIST_UPLOAD_CRASH_POINT=before_publish")
	out, err := cmd.CombinedOutput()
	var exit *exec.ExitError
	if !errors.As(err, &exit) || exit.ExitCode() != 71 {
		t.Fatalf("creator did not crash: %v %s", err, out)
	}
	orphan := filepath.Join(path, "create-"+ID("request"))
	if _, err := os.Stat(orphan); err != nil {
		t.Fatal("missing abandoned staging:", err)
	}
	state := create(t, s, "a", manifest([]byte("abc")))
	if _, err := os.Stat(orphan); !os.IsNotExist(err) {
		t.Fatal(err)
	}
	if err := s.With(t.Context(), state.ID, func(v *Session) error { return v.Cancel() }); err != nil {
		t.Fatal(err)
	}
	interrupted := errors.New("interrupted cleanup")
	s.fault = func(point string) error {
		if point == "after_discard" {
			return interrupted
		}
		return nil
	}
	if err := s.Sweep(t.Context()); !errors.Is(err, interrupted) {
		t.Fatal(err)
	}
	s = newStore(t, path, 100)
	checkKind(t, s.With(t.Context(), state.ID, func(*Session) error { return nil }), "not_found")
	if err := s.Sweep(t.Context()); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(path, "discard-"+state.ID)); !os.IsNotExist(err) {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(path, "locks", state.ID[:2]+".lock")); err != nil {
		t.Fatal("cleanup unlinked stable lock:", err)
	}
}

func TestConcurrentCreationReservesDeclaredSpace(t *testing.T) {
	path := t.TempDir()
	s := newStore(t, path, 10)
	var wg sync.WaitGroup
	var mu sync.Mutex
	created := 0
	for i := 0; i < 10; i++ {
		wg.Go(func() {
			other := newStore(t, path, 10)
			_, err := other.Create(t.Context(), fmt.Sprint(i), manifest([]byte("1234567890")))
			if err == nil {
				mu.Lock()
				created++
				mu.Unlock()
			} else {
				checkKind(t, err, "quota")
			}
		})
	}
	wg.Wait()
	if created != 2 {
		t.Fatalf("reserved %d sessions, want exactly two", created)
	}
	if err := s.Sweep(t.Context()); err != nil {
		t.Fatal(err)
	}
}

func TestEmptyFilesSessionCountAndCompletion(t *testing.T) {
	s := newStore(t, t.TempDir(), 100)
	for i := 0; i < MaxSessions; i++ {
		create(t, s, fmt.Sprint(i), manifest(nil))
	}
	_, err := s.Create(t.Context(), "excess", manifest(nil))
	checkKind(t, err, "quota")
	if err := s.With(t.Context(), ID("0"), func(v *Session) error {
		if _, err := v.Verify(t.Context()); err != nil {
			return err
		}
		if err := v.Complete(42); err != nil {
			return err
		}
		checkKind(t, v.Cancel(), "completed")
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if progress(t, s, ID("0")).OpID != 42 {
		t.Fatal("lost completion receipt")
	}
	create(t, s, "replacement", manifest(nil))
	checkKind(t, s.With(t.Context(), ID("0"), func(*Session) error { return nil }), "not_found")
}

func TestCanceledBlockDoesNotAdvanceProgress(t *testing.T) {
	s := newStore(t, t.TempDir(), 100)
	a := create(t, s, "a", manifest([]byte("abcd")))
	ctx, cancel := context.WithCancel(t.Context())
	s.fault = func(point string) error {
		if point == "after_data_sync" {
			cancel()
		}
		return nil
	}
	if err := appendBlock(s, ctx, a.ID, 0, []byte("abcd")); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	s.fault = nil
	if got := progress(t, s, a.ID).Offsets[0]; got != 0 {
		t.Fatal(got)
	}
	if err := appendBlock(s, t.Context(), a.ID, 0, []byte("abcd")); err != nil {
		t.Fatal(err)
	}
}
