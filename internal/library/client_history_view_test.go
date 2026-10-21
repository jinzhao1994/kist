package library

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/jinzhao1994/kist/internal/domain"
)

func TestClientHistoryUsesConfirmedPrefixWithoutPendingEdits(t *testing.T) {
	nas, box := openWithBox(t)
	file := importOneFile(t, nas, box, "original.txt", "original content")
	cutoff, err := nas.SyncHead(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := nas.RenameEntry(t.Context(), box, file.ID.ID, false, "original.txt", "NAS.txt"); err != nil {
		t.Fatal(err)
	}
	c := newSyncClient(t, nas)
	if _, err := c.RenameEntry(t.Context(), box, file.ID.ID, false, "NAS.txt", "laptop.txt"); err != nil {
		t.Fatal(err)
	}
	var escaped *ClientHistoricalView
	err = c.ViewAt(t.Context(), cutoff.OpID, func(v *ClientHistoricalView) error {
		escaped = v
		got, found, err := v.File(t.Context(), file.ID)
		if err != nil || !found || got.Name != "original.txt" {
			t.Fatalf("historic=%+v found=%v err=%v", got, found, err)
		}
		if _, _, err := v.OpenContent(t.Context(), file.ID); !errors.Is(err, ErrNotCached) {
			t.Fatalf("missing original=%v", err)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := escaped.File(t.Context(), file.ID); !errors.Is(err, ErrRequestScope) {
		t.Fatalf("escaped view=%v", err)
	}
	current, found, err := c.File(t.Context(), file.ID)
	if err != nil || !found || current.Name != "laptop.txt" {
		t.Fatal(current, err)
	}
	latest, err := nas.SyncHead(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if err := c.ViewAt(t.Context(), latest.OpID, func(v *ClientHistoricalView) error {
		got, found, err := v.File(t.Context(), file.ID)
		if err != nil || !found || got.Name != "NAS.txt" {
			t.Fatal(got, err)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := c.ViewAt(t.Context(), latest.OpID+1, func(*ClientHistoricalView) error { t.Fatal("overlay ordinal accepted as NAS history"); return nil }); !errors.Is(err, ErrNoSuchOp) {
		t.Fatal(err)
	}
	if _, err := c.PutOriginal(t.Context(), strings.NewReader("original content")); err != nil {
		t.Fatal(err)
	}
	var handle io.ReadSeekCloser
	if err := c.ViewAt(t.Context(), cutoff.OpID, func(v *ClientHistoricalView) error {
		_, reader, err := v.OpenContent(t.Context(), file.ID)
		handle = reader
		return err
	}); err != nil {
		t.Fatal(err)
	}
	defer handle.Close()
	data, err := io.ReadAll(handle)
	if err != nil || string(data) != "original content" {
		t.Fatal(string(data), err)
	}
}

func TestClientHistoryRejectsCancelledAndUnavailableVersions(t *testing.T) {
	nas := openTestLibrary(t)
	c := newSyncClient(t, nas)
	if _, err := c.CreateBox(t.Context(), "pending", "", domain.LocalDate{}, domain.LocalDate{}); err != nil {
		t.Fatal(err)
	}
	if err := c.ViewAt(t.Context(), 1, func(*ClientHistoricalView) error { t.Fatal("pending exposed as history"); return nil }); !errors.Is(err, ErrNoSuchOp) {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if err := c.ViewAt(ctx, 1, func(*ClientHistoricalView) error { return nil }); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}
