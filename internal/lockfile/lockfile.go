// Package lockfile provides the advisory write lock that lets several kist
// processes share one library (spec 2.7).
//
// The lock is held for one logical write operation, never for the lifetime of
// a process: a `kist serve` that held it while running would stop anybody from
// ever using the CLI again. Ordinary queries never take it, so browsing a library while
// something writes to it is free — which is the whole reason WAL mode is on.
// Integrity verification also holds it while comparing logs and views.
//
// It is an OS advisory lock rather than a pid file because the kernel drops it
// when the holder dies. That removes the entire category of stale locks: there
// is nothing to time out, no liveness check, no --force-unlock, and no way for
// a crash to leave a library that refuses to open.
package lockfile

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"
)

// DefaultWait is how long Acquire waits before giving up.
//
// A write is milliseconds — one transaction plus an incremental projection —
// so almost every wait ends immediately and failing fast would only invite
// retries. Five seconds is long enough to ride out a normal write and short
// enough that a stuck one is reported rather than hung on.
const DefaultWait = 5 * time.Second

// pollCap bounds the retry interval. Acquire polls rather than blocking in the
// kernel because a blocking flock cannot be cancelled, and a write that ignores
// Ctrl-C is worse than one that waits a few extra milliseconds.
const (
	pollStart = time.Millisecond
	pollCap   = 100 * time.Millisecond
)

// Locker is one library's write lock.
//
// It carries an in-process semaphore as well as the file lock. Not because the
// file lock would fail within a process — on Unix flock is per open file
// description, so a second Acquire does block — but because relying on that
// means knowing each platform's answer to "is this per-process or per-fd", and
// a channel is faster and obviously right on all of them. serve makes this a
// live concern rather than a hypothetical: two browser tabs submitting at once
// are two concurrent writes.
type Locker struct {
	path   string
	wait   time.Duration
	sem    chan struct{}
	shared bool
}

// New returns a Locker for the given lock file. The file is created on first
// use and never removed — deleting it while a lock is held would let a second
// process create a fresh one and lock that instead.
func New(path string) *Locker {
	return &Locker{path: path, wait: DefaultWait, sem: make(chan struct{}, 1)}
}

// NewShared creates a reader lease. Independent shared lockers may coexist;
// an exclusive locker on the same path excludes all of them. As with New,
// never remove the lock file while it could be in use.
func NewShared(path string) *Locker {
	l := New(path)
	l.shared = true
	return l
}

// TryAcquire attempts a lock without waiting. A busy lock is not an error;
// callers such as cache eviction can skip an object leased by another process.
func (l *Locker) TryAcquire() (func(), bool, error) {
	select {
	case l.sem <- struct{}{}:
	default:
		return nil, false, nil
	}
	f, err := os.OpenFile(l.path, os.O_RDWR|os.O_CREATE, 0o644)
	if err != nil {
		<-l.sem
		return nil, false, err
	}
	ok, err := tryLock(f, l.shared)
	if err != nil || !ok {
		f.Close()
		<-l.sem
		return nil, false, err
	}
	if !l.shared {
		l.describe(f)
	}
	return func() { unlock(f); f.Close(); <-l.sem }, true, nil
}

// BusyError reports that someone else is writing.
type BusyError struct {
	Path string
	// Holder is what the holder wrote about itself, or "" if it could not be
	// read. It is diagnostic only: correctness comes from the kernel's lock,
	// never from this text, which is why a library with an empty or garbled
	// kist.lock still behaves correctly.
	Holder string
}

func (e *BusyError) Error() string {
	if e.Holder == "" {
		return "another kist operation holds this library lock; try again"
	}
	return fmt.Sprintf("another kist operation holds this library lock:\n  %s", e.Holder)
}

// Acquire takes the lock and returns the function that releases it.
//
// It returns a *BusyError if someone else still holds it after the wait, and
// ctx's error if the caller gave up first. The returned function is safe to
// call once; callers defer it.
func (l *Locker) Acquire(ctx context.Context) (func(), error) {
	// One deadline covers both waits. Which of the two a caller is stuck
	// behind — another process, or another request inside this one — is an
	// implementation detail to the person watching a command not return, so
	// the same five seconds and the same BusyError apply to both.
	deadline := time.Now().Add(l.wait)

	timer := time.NewTimer(l.wait)
	defer timer.Stop()
	select {
	case l.sem <- struct{}{}:
	case <-timer.C:
		return nil, &BusyError{Path: l.path, Holder: readHolder(l.path)}
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	release, err := l.acquireFile(ctx, deadline)
	if err != nil {
		<-l.sem
		return nil, err
	}
	return func() {
		release()
		<-l.sem
	}, nil
}

func (l *Locker) acquireFile(ctx context.Context, deadline time.Time) (func(), error) {
	f, err := os.OpenFile(l.path, os.O_RDWR|os.O_CREATE, 0o644)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", l.path, err)
	}

	for delay := pollStart; ; {
		ok, err := tryLock(f, l.shared)
		if err != nil {
			f.Close()
			return nil, fmt.Errorf("lock %s: %w", l.path, err)
		}
		if ok {
			if !l.shared {
				l.describe(f)
			}
			return func() {
				// Unlock before close so the window where the file is open but
				// unlocked belongs to us rather than to the kernel's cleanup.
				unlock(f)
				f.Close()
			}, nil
		}
		if remaining := time.Until(deadline); remaining <= 0 {
			f.Close()
			return nil, &BusyError{Path: l.path, Holder: readHolder(l.path)}
		} else if delay > remaining {
			delay = remaining
		}

		select {
		case <-time.After(delay):
		case <-ctx.Done():
			f.Close()
			return nil, ctx.Err()
		}
		if delay *= 2; delay > pollCap {
			delay = pollCap
		}
	}
}

// describe records who holds the lock, for the error message the next process
// will print. Failures are ignored on purpose: a library on read-only media
// should still lock, and losing the diagnostic is not worth losing the write.
func (l *Locker) describe(f *os.File) {
	body := fmt.Sprintf("pid %d\n%s\n", os.Getpid(), strings.Join(os.Args, " "))
	if err := f.Truncate(0); err != nil {
		return
	}
	f.WriteAt([]byte(body), 0)
	f.Sync()
}

// readHolder reads the holder description back. It is racy by nature — the
// holder may have released between the failed lock attempt and this read — so
// its result is only ever put in front of a human, never acted on.
func readHolder(path string) string {
	raw, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.Join(strings.Fields(strings.TrimSpace(string(raw))), " ")
}

// IsBusy reports whether err is a *BusyError.
func IsBusy(err error) bool {
	var b *BusyError
	return errors.As(err, &b)
}
