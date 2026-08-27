package lockfile

import (
	"bufio"
	"context"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

func newLocker(t *testing.T) *Locker {
	t.Helper()
	l := New(filepath.Join(t.TempDir(), "kist.lock"))
	// Tests must not sit through the real wait. Every case here is either
	// "free right now" or "held and will stay held".
	l.wait = 150 * time.Millisecond
	return l
}

func TestAcquireAndRelease(t *testing.T) {
	l := newLocker(t)
	release, err := l.Acquire(t.Context())
	if err != nil {
		t.Fatalf("Acquire: %v", err)
	}
	release()

	// And it is available again afterwards.
	release, err = l.Acquire(t.Context())
	if err != nil {
		t.Fatalf("second Acquire: %v", err)
	}
	release()
}

// Two Lockers on one file: the ordinary two-processes case, in one process.
func TestSecondLockerIsBusy(t *testing.T) {
	l := newLocker(t)
	release, err := l.Acquire(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	defer release()

	other := New(l.path)
	other.wait = 150 * time.Millisecond
	if _, err := other.Acquire(t.Context()); !IsBusy(err) {
		t.Fatalf("err = %v, want a BusyError", err)
	}
}

// The same Locker used twice — serve's two browser tabs. It must queue behind
// the in-process semaphore and, if that takes too long, report busy rather
// than hang: which side of the process boundary the holder is on is not
// something the person watching the command cares about.
func TestSameLockerIsBusyRatherThanHanging(t *testing.T) {
	l := newLocker(t)
	release, err := l.Acquire(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	defer release()

	done := make(chan error, 1)
	go func() { _, err := l.Acquire(t.Context()); done <- err }()
	select {
	case err := <-done:
		if !IsBusy(err) {
			t.Fatalf("err = %v, want a BusyError", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("a second Acquire on the same Locker never returned")
	}
}

// The error has to name the holder, or a user staring at "another kist process
// is writing" has nothing to act on (spec 2.7).
func TestBusyErrorNamesTheHolder(t *testing.T) {
	l := newLocker(t)
	release, err := l.Acquire(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	defer release()

	_, err = New(l.path).Acquire(t.Context()) // a different Locker, same file
	if !IsBusy(err) {
		t.Fatalf("err = %v, want a BusyError", err)
	}
	if !strings.Contains(err.Error(), strconv.Itoa(os.Getpid())) {
		t.Errorf("the error does not name the holding pid: %v", err)
	}
}

// Waiting must be interruptible. A write that ignores Ctrl-C because it is
// parked in the kernel is worse than one that waits.
func TestAcquireGivesUpWhenTheContextDoes(t *testing.T) {
	l := newLocker(t)
	l.wait = time.Minute
	release, err := l.Acquire(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	defer release()

	ctx, cancel := context.WithTimeout(t.Context(), 50*time.Millisecond)
	defer cancel()
	start := time.Now()
	if _, err := New(l.path).Acquire(ctx); err != context.DeadlineExceeded {
		t.Fatalf("err = %v, want DeadlineExceeded", err)
	}
	if elapsed := time.Since(start); elapsed > 10*time.Second {
		t.Errorf("Acquire waited %s before honouring the context", elapsed)
	}
}

// An unlocked file that already holds someone's old description must not be
// mistaken for a held lock.
func TestStaleDescriptionDoesNotLock(t *testing.T) {
	l := newLocker(t)
	release, err := l.Acquire(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	release()

	raw, err := os.ReadFile(l.path)
	if err != nil || len(raw) == 0 {
		t.Fatalf("the lock file holds no description: %q, %v", raw, err)
	}
	release, err = l.Acquire(t.Context())
	if err != nil {
		t.Fatalf("a released lock whose file still names the old holder refused: %v", err)
	}
	release()
}

// The one that matters. Everything above would still pass if the file lock
// were deleted and only the in-process semaphore remained; this is the case
// that proves the lock crosses processes, which is the entire point.
func TestLockIsExclusiveAcrossProcesses(t *testing.T) {
	path := filepath.Join(t.TempDir(), "kist.lock")

	cmd := exec.Command(os.Args[0], "-test.run=TestHelperHoldsTheLock", "-test.timeout=60s")
	cmd.Env = append(os.Environ(), "KIST_LOCK_HELPER="+path)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() {
		stdin.Close()
		cmd.Wait()
	}()

	// Wait until the child says it holds the lock.
	out := bufio.NewReader(stdout)
	line, err := out.ReadString('\n')
	if err != nil || strings.TrimSpace(line) != "locked" {
		t.Fatalf("helper did not take the lock: %q, %v", line, err)
	}

	l := New(path)
	l.wait = 150 * time.Millisecond
	if _, err := l.Acquire(t.Context()); !IsBusy(err) {
		t.Fatalf("err = %v, want a BusyError — the lock did not cross the process boundary", err)
	}

	// And releasing it in the child releases it here.
	stdin.Close()
	if err := cmd.Wait(); err != nil {
		t.Fatalf("helper exited badly: %v", err)
	}
	release, err := l.Acquire(t.Context())
	if err != nil {
		t.Fatalf("the lock was not free after the holder exited: %v", err)
	}
	release()
}

// A crashed holder must not leave a library nobody can write to. The kernel
// dropping the lock on exit is the whole reason this is an advisory lock
// rather than a pid file (spec 2.7).
//
// Unlike the rest of this file, no small mutation of flock_unix.go makes this
// fail — the property belongs to the OS primitive, not to our code. It is here
// as a regression guard for the day somebody reaches for a pid file, which is
// exactly the change that would break it.
func TestLockSurvivesAKilledHolder(t *testing.T) {
	path := filepath.Join(t.TempDir(), "kist.lock")

	cmd := exec.Command(os.Args[0], "-test.run=TestHelperHoldsTheLock", "-test.timeout=60s")
	cmd.Env = append(os.Environ(), "KIST_LOCK_HELPER="+path)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	cmd.Stdin = strings.NewReader("") // never closed by us; the kill ends it
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	out := bufio.NewReader(stdout)
	if line, err := out.ReadString('\n'); err != nil || strings.TrimSpace(line) != "locked" {
		t.Fatalf("helper did not take the lock: %q, %v", line, err)
	}

	if err := cmd.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	cmd.Wait()

	l := New(path)
	l.wait = 2 * time.Second
	release, err := l.Acquire(t.Context())
	if err != nil {
		t.Fatalf("a killed holder left the lock stuck: %v", err)
	}
	release()
}

// TestHelperHoldsTheLock is not a test. It is the other process: it takes the
// lock, says so, and holds it until its stdin closes.
func TestHelperHoldsTheLock(t *testing.T) {
	path := os.Getenv("KIST_LOCK_HELPER")
	if path == "" {
		t.Skip("not the helper process")
	}
	l := New(path)
	release, err := l.Acquire(context.Background())
	if err != nil {
		t.Fatalf("helper could not take the lock: %v", err)
	}
	defer release()

	os.Stdout.WriteString("locked\n")
	io.Copy(io.Discard, os.Stdin)
}
