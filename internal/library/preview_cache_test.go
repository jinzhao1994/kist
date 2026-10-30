package library

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/jinzhao1994/kist/internal/derive"
)

func TestPreviewCacheMissingDirectory(t *testing.T) {
	l, id, _ := mediaFixture(t)
	m := l.Media()
	ctx := t.Context()
	if _, err := m.Request(ctx, id, derive.Thumbnail, false); err != nil {
		t.Fatal(err)
	}
	if err := m.work(ctx); err != nil {
		t.Fatal(err)
	}
	j, err := m.Status(ctx, id, derive.Thumbnail)
	if err != nil {
		t.Fatal(err)
	}
	_, path, err := m.paths(j)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.RemoveAll(filepath.Dir(path)); err != nil {
		t.Fatal(err)
	}
	if err = m.PrunePreviews(ctx); err != nil {
		t.Fatal(err)
	}
	db, err := m.database()
	if err != nil {
		t.Fatal(err)
	}
	var count int
	if err = db.DB.QueryRow(`SELECT count(*) FROM preview_cache`).Scan(&count); err != nil || count != 0 {
		t.Fatal(count, err)
	}
	if _, err = m.Request(ctx, id, derive.Thumbnail, false); err != nil {
		t.Fatal(err)
	}
	if err = m.work(ctx); err != nil {
		t.Fatal(err)
	}
	if j, err = m.Status(ctx, id, derive.Thumbnail); err != nil || j.State != "ready" {
		t.Fatal(j, err)
	}
}

func TestPreviewCacheRetentionLeasesAndRegeneration(t *testing.T) {
	l, id, _ := mediaFixture(t)
	m := l.Media()
	ctx := t.Context()
	for _, recipe := range []string{derive.Thumbnail, derive.Preview} {
		if _, err := m.Request(ctx, id, recipe, false); err != nil {
			t.Fatal(err)
		}
	}
	if err := m.work(ctx); err != nil {
		t.Fatal(err)
	}
	db, err := m.database()
	if err != nil {
		t.Fatal(err)
	}
	var attempts int
	if err = db.DB.QueryRow(`SELECT count(*) FROM job_attempts WHERE kind='media'`).Scan(&attempts); err != nil {
		t.Fatal(err)
	}
	if _, err = db.DB.Exec(`UPDATE preview_cache SET accessed_at=?`, time.Now().Add(-365*24*time.Hour).UnixNano()); err != nil {
		t.Fatal(err)
	}
	if err = m.PrunePreviews(ctx); err != nil {
		t.Fatal(err)
	}
	if j, err := m.Status(ctx, id, derive.Preview); err != nil || j.State != "ready" {
		t.Fatal("default expired preview", j, err)
	}
	if err = updateInstanceConfig(ctx, l.layout.Root, "", map[string]any{"previews.cache_max_age": "1h"}); err != nil {
		t.Fatal(err)
	}
	_, held, err := m.Open(ctx, id, derive.Preview)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.DB.Exec(`UPDATE preview_cache SET accessed_at=?`, time.Now().Add(-2*time.Hour).UnixNano()); err != nil {
		t.Fatal(err)
	}
	if err = l.Media().PrunePreviews(ctx); err != nil {
		t.Fatal(err)
	}
	if j, err := m.Status(ctx, id, derive.Preview); err != nil || j.State != "ready" {
		t.Fatal("evicted active preview", j, err)
	}
	if j, err := m.Status(ctx, id, derive.Thumbnail); err != nil || j.State != "missing" {
		t.Fatal("expired thumbnail retained", j, err)
	}
	if err = held.Close(); err != nil {
		t.Fatal(err)
	}
	if err = m.PrunePreviews(ctx); err != nil {
		t.Fatal(err)
	}
	if j, err := m.Status(ctx, id, derive.Preview); err != nil || j.State != "ready" {
		t.Fatal("close did not renew access", j, err)
	}
	if _, err = m.Request(ctx, id, derive.Thumbnail, false); err != nil {
		t.Fatal(err)
	}
	if err = m.work(ctx); err != nil {
		t.Fatal(err)
	}
	if j, err := m.Status(ctx, id, derive.Thumbnail); err != nil || j.State != "ready" {
		t.Fatal("did not regenerate", j, err)
	}
	var after int
	if err = db.DB.QueryRow(`SELECT count(*) FROM job_attempts WHERE kind='media'`).Scan(&after); err != nil || after != attempts+1 {
		t.Fatal("lost execution history", after, err)
	}
	if report, err := l.Verify(ctx, VerifyOptions{Deep: true}); err != nil || report.Total() != 0 {
		t.Fatal(report, err)
	}
}

func TestPreviewCacheQuotaAndUnlimitedCapacity(t *testing.T) {
	l, id, _ := mediaFixture(t)
	m := l.Media()
	ctx := t.Context()
	if _, err := m.Request(ctx, id, derive.Thumbnail, false); err != nil {
		t.Fatal(err)
	}
	if err := m.work(ctx); err != nil {
		t.Fatal(err)
	}
	j, held, err := m.Open(ctx, id, derive.Thumbnail)
	if err != nil {
		t.Fatal(err)
	}
	defer held.Close()
	if err = updateInstanceConfig(ctx, l.layout.Root, "", map[string]any{"previews.cache_max_bytes": j.Result.Bytes}); err != nil {
		t.Fatal(err)
	}
	if _, err = m.Request(ctx, id, derive.Preview, false); err != nil {
		t.Fatal(err)
	}
	if err = m.work(ctx); err != nil {
		t.Fatal(err)
	}
	if failed, err := m.Status(ctx, id, derive.Preview); err != nil || failed.State != "failed" || failed.Reason != "cache_quota" {
		t.Fatal(failed, err)
	}
	if err = held.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err = m.Request(ctx, id, derive.Preview, true); err != nil {
		t.Fatal(err)
	}
	if err = m.work(ctx); err != nil {
		t.Fatal(err)
	}
	if old, err := m.Status(ctx, id, derive.Thumbnail); err != nil || old.State != "missing" {
		t.Fatal("quota did not evict older content", old, err)
	}
	if err = updateInstanceConfig(ctx, l.layout.Root, "", map[string]any{"previews.cache_max_bytes": "inf"}); err != nil {
		t.Fatal(err)
	}
	if _, err = m.Request(ctx, id, derive.Thumbnail, false); err != nil {
		t.Fatal(err)
	}
	if err = m.work(ctx); err != nil {
		t.Fatal(err)
	}
	for _, recipe := range []string{derive.Thumbnail, derive.Preview} {
		j, err := m.Status(ctx, id, recipe)
		if err != nil || j.State != "ready" {
			t.Fatal("unlimited capacity evicted content", j, err)
		}
	}
}

func TestPreviewCacheAdoptsOldOutputsWithoutLosingHistory(t *testing.T) {
	l, id, _ := mediaFixture(t)
	m := l.Media()
	ctx := t.Context()
	if _, err := m.Request(ctx, id, derive.Thumbnail, false); err != nil {
		t.Fatal(err)
	}
	if err := m.work(ctx); err != nil {
		t.Fatal(err)
	}
	db, err := m.database()
	if err != nil {
		t.Fatal(err)
	}
	// Simulate the pre-index runtime schema's successful job and history.
	if _, err = db.DB.Exec(`DELETE FROM preview_cache; DELETE FROM migrations WHERE source='preview-cache-index'; UPDATE background_jobs SET updated_at='2020-01-01T00:00:00Z' WHERE kind='media'`); err != nil {
		t.Fatal(err)
	}
	before := time.Now()
	if err = adoptPreviewCache(db); err != nil {
		t.Fatal(err)
	}
	var accessed int64
	var attempts int
	if err = db.DB.QueryRow(`SELECT accessed_at FROM preview_cache`).Scan(&accessed); err != nil || accessed < before.UnixNano() {
		t.Fatal(accessed, err)
	}
	if err = db.DB.QueryRow(`SELECT count(*) FROM job_attempts WHERE kind='media'`).Scan(&attempts); err != nil || attempts != 1 {
		t.Fatal(attempts, err)
	}
	if err = updateInstanceConfig(ctx, l.layout.Root, "", map[string]any{"previews.cache_max_age": "1h"}); err != nil {
		t.Fatal(err)
	}
	if err = m.PrunePreviews(ctx); err != nil {
		t.Fatal(err)
	}
	if j, err := m.Status(ctx, id, derive.Thumbnail); err != nil || j.State != "ready" {
		t.Fatal("migration expired old output immediately", j, err)
	}
	if err = adoptPreviewCache(db); err != nil {
		t.Fatal(err)
	}
	var again int64
	if err = db.DB.QueryRow(`SELECT accessed_at FROM preview_cache`).Scan(&again); err != nil || again != accessed {
		t.Fatal("migration renewed access on reopen", again, err)
	}
}
