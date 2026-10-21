package library

import (
	"strings"
	"testing"
	"time"

	"github.com/jinzhao1994/kist/internal/domain"
)

func TestClientProtectedStorageIsSeparateFromCacheBudget(t *testing.T) {
	nas := openTestLibrary(t)
	c := newSyncClient(t, nas)
	ctx := t.Context()
	before, err := c.ProtectedStorage(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.PutOriginal(ctx, strings.NewReader("protected original")); err != nil {
		t.Fatal(err)
	}
	if _, err := c.PutOriginal(ctx, strings.NewReader("protected original")); err != nil {
		t.Fatal(err)
	}
	if _, err := c.CreateBox(ctx, "offline", "", domain.LocalDate{}, domain.LocalDate{}); err != nil {
		t.Fatal(err)
	}
	stored, err := c.ProtectedStorage(ctx)
	if err != nil || stored.OriginalsBytes-before.OriginalsBytes != int64(len("protected original")) || stored.PendingJournalBytes <= before.PendingJournalBytes {
		t.Fatal(before, stored, err)
	}
	if _, err := c.SetCacheLimits(ctx, time.Nanosecond, 1); err != nil {
		t.Fatal(err)
	}
	if _, err := c.PruneCache(ctx); err != nil {
		t.Fatal(err)
	}
	after, err := c.ProtectedStorage(ctx)
	if err != nil || after != stored {
		t.Fatal(after, stored, err)
	}
	cache, err := c.CacheStatus(ctx)
	if err != nil || cache.CachedBytes != 0 || cache.PartialBytes != 0 {
		t.Fatal(cache, err)
	}
}
