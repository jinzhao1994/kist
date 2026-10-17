package library

import (
	"encoding/json"
	"testing"

	"github.com/jinzhao1994/kist/internal/domain"
	"github.com/jinzhao1994/kist/internal/oplog"
)

func TestHistorySummaryFrozenNamesAndBatches(t *testing.T) {
	cases := []struct {
		action        oplog.Action
		raw           string
		name          string
		count         int
		before, after string
	}{
		{oplog.ActionCreateBox, `{"name":"Trip"}`, "Trip", 1, "", ""},
		{oplog.ActionCreateTag, `{"name":"Receipt"}`, "Receipt", 1, "", ""},
		{oplog.ActionCreateFolder, `{"folder":{"name":"Photos"}}`, "Photos", 1, "", ""},
		{oplog.ActionDeleteEntryContents, `{"name":"Gone.pdf","files":[]}`, "Gone.pdf", 1, "", ""},
		{oplog.ActionDeleteBoxContents, `{"box":{"Name":"Old box"}}`, "Old box", 1, "", ""},
		{oplog.ActionImportFiles, `{"files":[{"name":"a.txt"},{"name":"b.txt"}]}`, "a.txt", 2, "", ""},
		{oplog.ActionDeleteFiles, `{"files":[{"name":"a.txt"}]}`, "a.txt", 1, "", ""},
		{oplog.ActionRenameEntry, `{"before":"old","after":"new"}`, "old", 1, "old", "new"},
		{oplog.ActionUpdateBox, `{"name":{"before":"Old box","after":"New box"}}`, "Old box", 1, "Old box", "New box"},
		{oplog.ActionCreateSavedSearch, `{"after":{"name":"Invoices"}}`, "Invoices", 1, "", ""},
		{oplog.ActionDeleteSavedSearch, `{"before":{"name":"Invoices"}}`, "Invoices", 1, "", ""},
		{oplog.ActionUploadFiles, `{"import":{"files":[{"name":"a.txt"}]},"replacements":[{"after":{"name":"b.txt"}}]}`, "a.txt", 2, "", ""},
		{oplog.ActionRequestReceipt, `{}`, "", 0, "", ""},
	}
	for _, tt := range cases {
		t.Run(string(tt.action), func(t *testing.T) {
			got, err := summarizeHistory(nil, tt.action, []byte(tt.raw))
			if err != nil || got.Name != tt.name || got.Count != tt.count || got.Before != tt.before || got.After != tt.after || got.CurrentNames {
				t.Fatalf("%+v %v", got, err)
			}
		})
	}
}

func TestHistorySummaryCurrentReferencesAndReadOnly(t *testing.T) {
	l, box := openWithBox(t)
	ctx := t.Context()
	tag, err := l.CreateTag(ctx, "Receipts", nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = l.TagBoxes(ctx, tag.ID, []domain.BoxID{box}); err != nil {
		t.Fatal(err)
	}
	entries, _, err := l.History(ctx, ListOpsOptions{Action: "tag_boxes"})
	if err != nil {
		t.Fatal(err)
	}
	before := snapshotOf(t, l)
	summary, err := l.HistorySummary(ctx, entries[0].ID)
	if err != nil || summary.Name == "" || summary.Tag != "Receipts" || summary.Count != 1 || !summary.CurrentNames {
		t.Fatalf("%+v %v", summary, err)
	}
	if before != snapshotOf(t, l) {
		t.Fatal("summary mutated archive")
	}
	if err = l.RenameTag(ctx, tag.ID, "Renamed"); err != nil {
		t.Fatal(err)
	}
	summary, err = l.HistorySummary(ctx, entries[0].ID)
	if err != nil || summary.Tag != "Renamed" || !summary.CurrentNames {
		t.Fatalf("%+v %v", summary, err)
	}
	// Summaries never expose a payload or metadata, even when a batch is large.
	raw, _ := json.Marshal(summary)
	if len(raw) > 1000 {
		t.Fatalf("unbounded summary: %s", raw)
	}
}
