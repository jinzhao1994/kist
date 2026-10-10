package library

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jinzhao1994/kist/internal/blob"
	"github.com/jinzhao1994/kist/internal/lockfile"
)

type blobReadFunc func([]byte) (int, error)

func (f blobReadFunc) Read(p []byte) (int, error) { return f(p) }

// A separate lock owner models another process. Staging must finish while it
// holds the blob lock, but final placement must wait. The archive commit lock
// stays held throughout, proving that neither phase needs the database lock.
func TestBlobPlacementUsesSharedLockAfterStaging(t *testing.T) {
	for _, restore := range []bool{false, true} {
		t.Run(fmt.Sprintf("restore=%t", restore), func(t *testing.T) {
			l := openTestLibrary(t)
			releaseCommit, err := l.lock.Acquire(t.Context())
			if err != nil {
				t.Fatal(err)
			}
			defer releaseCommit()
			external := lockfile.New(filepath.Join(l.layout.Root, "blobs.lock"))
			releaseBlob, err := external.Acquire(t.Context())
			if err != nil {
				t.Fatal(err)
			}
			defer func() {
				if releaseBlob != nil {
					releaseBlob()
				}
			}()
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			read := make(chan struct{})
			r := io.MultiReader(strings.NewReader("same content"), blobReadFunc(func([]byte) (int, error) {
				close(read)
				return 0, io.EOF
			}))
			done := make(chan error, 1)
			go func() {
				if restore {
					key := blob.NewKey(blob.AlgoSHA256, fmt.Sprintf("%x", sha256.Sum256([]byte("same content"))), 0)
					_, err := l.blobs.Restore(ctx, r, key)
					done <- err
				} else {
					_, err := l.blobs.Put(ctx, r)
					done <- err
				}
			}()
			select {
			case <-read:
			case <-time.After(3 * time.Second):
				t.Fatal("staging waited for a placement or archive lock")
			}
			select {
			case err := <-done:
				t.Fatalf("placement bypassed another process's lock: %v", err)
			case <-time.After(200 * time.Millisecond):
			}
			releaseBlob()
			releaseBlob = nil
			select {
			case err := <-done:
				if err != nil {
					t.Fatal(err)
				}
			case <-time.After(3 * time.Second):
				t.Fatal("placement did not resume after releasing the blob lock")
			}
		})
	}
}

// Separate Library instances must coordinate through the OS lock, not only
// through a mutex attached to one server or one blob.Dir.
func TestConcurrentBlobPlacementAcrossLibraries(t *testing.T) {
	first := openTestLibrary(t)
	second, err := Open(first.layout.Root)
	if err != nil {
		t.Fatal(err)
	}
	defer second.Close()
	start := make(chan struct{})
	results := make(chan blob.PutResult, 16)
	errors := make(chan error, 16)
	var wg sync.WaitGroup
	for i := range 16 {
		l := []*Library{first, second}[i%2]
		wg.Go(func() {
			<-start
			result, err := l.blobs.Put(t.Context(), strings.NewReader("concurrent identical content"))
			results <- result
			errors <- err
		})
	}
	close(start)
	wg.Wait()
	close(results)
	close(errors)
	for err := range errors {
		if err != nil {
			t.Fatal(err)
		}
	}
	stored, deduped := 0, 0
	for result := range results {
		switch result.Outcome {
		case blob.OutcomeStored:
			stored++
		case blob.OutcomeDeduped:
			deduped++
		default:
			t.Fatalf("unexpected outcome: %v", result.Outcome)
		}
	}
	if stored != 1 || deduped != 15 {
		t.Fatalf("stored=%d deduped=%d; a concurrent writer overwrote an existing blob", stored, deduped)
	}
}
