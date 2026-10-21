package library

import (
	"testing"
	"time"

	"github.com/jinzhao1994/kist/internal/domain"
)

func TestHistoryPreservesOccurrenceAndAcceptanceOrder(t *testing.T) {
	nas := openTestLibrary(t)
	if _, err := nas.CreateBox(t.Context(), "NAS local", "", domain.LocalDate{}, domain.LocalDate{}); err != nil {
		t.Fatal(err)
	}
	proposal := syncBox(t, "earlier-offline-operation")
	proposal.OccurredAt = time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	receipt, err := nas.AcceptSync(t.Context(), proposal)
	if err != nil {
		t.Fatal(err)
	}
	check := func(entries []HistoryEntry, err error) {
		t.Helper()
		if err != nil || len(entries) != 2 {
			t.Fatal(entries, err)
		}
		if entries[0].ID != receipt.OpID || !entries[0].TS.Equal(proposal.OccurredAt) || entries[0].AcceptedAt == nil || !entries[0].AcceptedAt.Equal(receipt.AcceptedAt) {
			t.Fatal(entries)
		}
		if entries[1].AcceptedAt != nil || !entries[0].TS.Before(entries[1].TS) {
			t.Fatal(entries)
		}
	}
	entries, _, err := nas.History(t.Context(), ListOpsOptions{})
	check(entries, err)
	if err := nas.Rebuild(t.Context()); err != nil {
		t.Fatal(err)
	}
	entries, _, err = nas.History(t.Context(), ListOpsOptions{})
	check(entries, err)
	c := newSyncClient(t, nas)
	entries, _, err = c.History(t.Context(), ListOpsOptions{})
	check(entries, err)
	detail, err := c.HistoryOp(t.Context(), receipt.OpID)
	if err != nil || detail.AcceptedAt == nil || !detail.AcceptedAt.Equal(receipt.AcceptedAt) {
		t.Fatal(detail, err)
	}
}
