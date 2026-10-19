package library

import (
	"testing"
	"time"

	"github.com/jinzhao1994/kist/internal/domain"
	"github.com/jinzhao1994/kist/internal/lockfile"
)

// blockedBy runs fn while something else holds this library's write lock, and
// reports whether fn waited for it.
//
// It asserts the waiting rather than the eventual BusyError so the test does
// not have to sit through lockfile.DefaultWait — and because waiting is the
// stronger claim: a write that returned an error immediately would satisfy
// "does not proceed" without ever having taken the lock.
func blockedBy(t *testing.T, l *Library, fn func() error) {
	t.Helper()

	other := lockfile.New(l.layout.LockPath())
	release, err := other.Acquire(t.Context())
	if err != nil {
		t.Fatalf("could not take the library's lock: %v", err)
	}

	done := make(chan error, 1)
	go func() { done <- fn() }()

	select {
	case err := <-done:
		release()
		t.Fatalf("the write did not wait for the lock (returned %v)", err)
	case <-time.After(200 * time.Millisecond):
	}

	release()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("the write failed once the lock was free: %v", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("the write never completed after the lock was released")
	}
}

// spec 2.7: every write takes the lock. write is the single path, so covering
// one op covers all of them — but this is the test that would notice if the
// Acquire were dropped from it.
func TestWriteTakesTheLock(t *testing.T) {
	l := openTestLibrary(t)
	blockedBy(t, l, func() error {
		_, err := l.CreateBox(t.Context(), "云南", "", domain.LocalDate{}, domain.LocalDate{})
		return err
	})
}

// Relink rewrites the whole of boxes/, so it has to be serialized against the
// incremental projection a write does at its end.
func TestRelinkTakesTheLock(t *testing.T) {
	l, _ := openWithBox(t)
	blockedBy(t, l, func() error {
		_, err := l.Relink(t.Context())
		return err
	})
}

// Rebuild truncates every materialized view before replaying. A write landing
// in the middle would be applied to tables about to be discarded.
func TestRebuildTakesTheLock(t *testing.T) {
	l, _ := openWithBox(t)
	blockedBy(t, l, func() error { return l.Rebuild(t.Context()) })
}

// project falls back to relink, and project runs inside write — which already
// holds the lock. If the fallback took the lock again it would deadlock
// against itself.
//
// What that looks like is not a failed write, which is why this test does not
// check for one: write treats a projection error as recoverable (relink fixes
// it) so the call still succeeds. The damage is five seconds gone and boxes/
// silently marked stale — the exact failure mode the dirty marker exists to
// make visible, arrived at by a bug rather than by a real filesystem problem.
//
// The trigger is spec 3.4.1's spreading collision suffix: two boxes whose
// directory names collide, where changing one changes the other's name too.
func TestTheRelinkFallbackDoesNotDeadlockAgainstItself(t *testing.T) {
	l := openTestLibrary(t)
	same := domain.LocalDate{}

	a, err := l.CreateBox(t.Context(), "云南", "", same, same)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := l.CreateBox(t.Context(), "云南", "", same, same); err != nil {
		t.Fatal(err)
	}
	// Renaming the first box away frees the bare name, so the second box's
	// suffix has to come off — a box nobody touched whose directory moved,
	// which is exactly the case project hands to relink.
	done := make(chan error, 1)
	go func() {
		_, err := l.UpdateBox(t.Context(), a.ID, BoxFields{Name: "西藏"})
		done <- err
	}()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("UpdateBox: %v", err)
		}
	case <-time.After(30 * time.Second):
		t.Fatal("the write deadlocked against its own lock")
	}
	if l.ProjectionDirty() {
		t.Error("the relink fallback did not run — boxes/ was left stale, " +
			"which is what a write deadlocking against its own lock looks like")
	}
}

func TestOpenSerializesSchemaMigration(t *testing.T) {
	l := openTestLibrary(t)
	blockedBy(t, l, func() error {
		opened, err := Open(l.layout.Root)
		if err == nil {
			err = opened.Close()
		}
		return err
	})
}
