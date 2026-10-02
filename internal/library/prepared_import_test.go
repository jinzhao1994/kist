package library

import (
	"context"
	"errors"
	"testing"
)

func TestImportPreparationDoesNotHoldCommitLock(t *testing.T) {
	l, box := openWithBox(t)
	src := writeTree(t, map[string]string{"a.txt": "alpha", "b.txt": "beta"})
	calls := 0
	l.hooks.afterFile = func() {
		// This would time out if preparation held the request or write lock.
		if calls == 0 {
			if _, err := l.CreateTag(t.Context(), "while-hashing", nil); err != nil {
				t.Fatal(err)
			}
		}
		calls++
	}
	prepared, err := l.PrepareImport(t.Context(), src, ImportOptions{Box: box})
	if err != nil {
		t.Fatal(err)
	}
	if len(listAll(t, l, box)) != 0 {
		t.Fatal("preparation changed the materialized view")
	}
	result := prepared.Result()
	if result.Count(FileImported) != 2 || result.OpID != 0 {
		t.Fatalf("preparation=%+v", result)
	}
	req := requestFor("request-upload", "a:alpha;b:beta")
	first, err := l.ExecuteRequest(t.Context(), req, func(ctx context.Context) error { _, err := prepared.Commit(ctx); return err })
	if err != nil {
		t.Fatal(err)
	}
	again, err := l.ExecuteRequest(t.Context(), req, func(context.Context) error { t.Fatal("upload committed twice"); return nil })
	if err != nil || again != first {
		t.Fatalf("again=%v err=%v", again, err)
	}
	if len(listAll(t, l, box)) != 2 {
		t.Fatal("committed batch does not contain exactly two files")
	}
}

func TestPreparedImportCancellationDoesNotCommit(t *testing.T) {
	l, box := openWithBox(t)
	src := writeTree(t, map[string]string{"a.txt": "alpha"})
	prepared, err := l.PrepareImport(t.Context(), src, ImportOptions{Box: box})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := prepared.Commit(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("commit=%v", err)
	}
	if len(listAll(t, l, box)) != 0 {
		t.Fatal("canceled commit changed state")
	}
}

func TestPreparingInsideRequestIsRejected(t *testing.T) {
	l, box := openWithBox(t)
	src := writeTree(t, map[string]string{"a.txt": "alpha"})
	_, err := l.ExecuteRequest(t.Context(), requestFor("request-bad-upload", "alpha"), func(ctx context.Context) error {
		_, err := l.PrepareImport(ctx, src, ImportOptions{Box: box})
		return err
	})
	if !errors.Is(err, ErrRequestScope) {
		t.Fatalf("prepare within request=%v", err)
	}
	if len(listAll(t, l, box)) != 0 {
		t.Fatal("rejected preparation imported a file")
	}
}

func TestPreparedImportResultIsACopy(t *testing.T) {
	l, box := openWithBox(t)
	prepared, err := l.PrepareImport(t.Context(), writeTree(t, map[string]string{"a.txt": "alpha"}), ImportOptions{Box: box})
	if err != nil {
		t.Fatal(err)
	}
	external := prepared.Result()
	external.Files[0].Name = "changed"
	if prepared.Result().Files[0].Name != "a.txt" {
		t.Fatal("caller mutated prepared result")
	}
}
