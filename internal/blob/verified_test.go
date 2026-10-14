package blob

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPutVerified(t *testing.T) {
	for _, content := range []string{"", "original content"} {
		t.Run(content, func(t *testing.T) {
			d := newTestDir(t)
			expected := ExpectedContent{SHA256: hexOf(content), Size: int64(len(content))}
			for _, outcome := range []Outcome{OutcomeStored, OutcomeDeduped} {
				got, err := d.PutVerified(context.Background(), strings.NewReader(content), expected)
				if err != nil || got.Outcome != outcome {
					t.Fatalf("put = %+v, %v", got, err)
				}
				digest, size, err := d.Rehash(context.Background(), got.Key)
				if err != nil || digest != expected.SHA256 || size != expected.Size {
					t.Fatalf("stored = %s, %d, %v", digest, size, err)
				}
			}
		})
	}
}

func TestPutVerifiedRejectsBeforePlacement(t *testing.T) {
	for _, tc := range []struct {
		name, content string
		expected      ExpectedContent
	}{
		{"same length corruption", "wrong", ExpectedContent{hexOf("right"), 5}},
		{"wrong size", "right", ExpectedContent{hexOf("right"), 6}},
		{"truncated", "righ", ExpectedContent{hexOf("right"), 5}},
		{"invalid digest", "right", ExpectedContent{"invalid", 5}},
		{"negative size", "right", ExpectedContent{hexOf("right"), -1}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			d := newTestDir(t)
			_, err := d.PutVerified(context.Background(), strings.NewReader(tc.content), tc.expected)
			if !errors.Is(err, ErrIntegrityMismatch) {
				t.Fatalf("error = %v", err)
			}
			err = filepath.WalkDir(d.Root(), func(path string, entry os.DirEntry, err error) error {
				if err != nil {
					return err
				}
				if !entry.IsDir() {
					t.Errorf("unexpected file %s", path)
				}
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestPutVerifiedCancelledCleansStaging(t *testing.T) {
	d := newTestDir(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := d.PutVerified(ctx, strings.NewReader("original"), ExpectedContent{hexOf("original"), 8})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v", err)
	}
	entries, err := os.ReadDir(filepath.Join(d.Root(), "tmp"))
	if err != nil || len(entries) != 0 {
		t.Fatalf("staging = %v, %v", entries, err)
	}
}
