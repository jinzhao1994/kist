package library

import (
	"errors"
	"testing"

	"github.com/jinzhao1994/kist/internal/domain"
	"github.com/jinzhao1994/kist/internal/query"
)

func TestClientTagQueriesTrackPendingTreeChanges(t *testing.T) {
	nas, box := openWithBox(t)
	file := importOneFile(t, nas, box, "photo.png", "uncached")
	c := newSyncClient(t, nas)
	ctx := t.Context()
	parent, err := c.CreateTag(ctx, "Travel", nil)
	if err != nil {
		t.Fatal(err)
	}
	child, err := c.CreateTag(ctx, "Photos", &parent.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.TagFiles(ctx, child.ID, []domain.FileID{file.ID}); err != nil {
		t.Fatal(err)
	}
	for _, ref := range []string{parent.Name, parent.ID.String()} {
		condition, err := c.TagCondition(ctx, []string{ref})
		if err != nil {
			t.Fatal(err)
		}
		files, _, err := c.Files(ctx, query.Query{Tags: condition})
		if err != nil || len(files) != 1 {
			t.Fatalf("include=%+v error=%v", files, err)
		}
		excluded, err := c.ExcludeTags(ctx, []string{ref})
		if err != nil {
			t.Fatal(err)
		}
		files, _, err = c.Files(ctx, query.Query{Tags: excluded})
		if err != nil || len(files) != 0 {
			t.Fatalf("exclude=%+v error=%v", files, err)
		}
	}
	if err := c.MoveTag(ctx, child.ID, nil); err != nil {
		t.Fatal(err)
	}
	condition, err := c.TagCondition(ctx, []string{parent.Name})
	if err != nil {
		t.Fatal(err)
	}
	files, _, err := c.Files(ctx, query.Query{Tags: condition})
	if err != nil || len(files) != 0 {
		t.Fatalf("moved subtree=%+v error=%v", files, err)
	}
	if err := c.RenameTag(ctx, child.ID, "Pictures"); err != nil {
		t.Fatal(err)
	}
	got, err := c.ResolveTag(ctx, "Pictures")
	if err != nil || got.ID != child.ID {
		t.Fatalf("resolve=%+v error=%v", got, err)
	}
	if _, err := c.ResolveTag(ctx, "Photos"); !errors.Is(err, ErrNoSuchTag) {
		t.Fatalf("old name=%v", err)
	}
	if empty, err := c.TagCondition(ctx, nil); err != nil || empty != nil {
		t.Fatalf("empty=%+v error=%v", empty, err)
	}
	if _, err := c.ExcludeTags(ctx, []string{"missing"}); !errors.Is(err, ErrNoSuchTag) {
		t.Fatalf("missing=%v", err)
	}
}
