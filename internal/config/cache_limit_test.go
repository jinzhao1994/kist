package config

import (
	"os"
	"testing"
)

func TestPreviewCacheLimitsAndDefaultSource(t *testing.T) {
	path := writeConfig(t, "version = 3\n")
	c, err := Load(path)
	if err != nil || c.Previews.CacheMaxBytes != 32_000_000_000 || c.Previews.CacheMaxAge != "inf" {
		t.Fatal(c, err)
	}
	for _, raw := range []string{"inf", "INF", " inf "} {
		normal, err := NormalizeChanges("library", map[string]any{"previews.cache_max_bytes": raw, "previews.cache_max_age": raw})
		if err != nil {
			t.Fatal(err)
		}
		if err = Update(path, "", normal); err != nil {
			t.Fatal(err)
		}
		c, err = Load(path)
		if err != nil || c.Previews.CacheMaxBytes != 0 || c.Previews.CacheMaxAge != "inf" {
			t.Fatal(c, err)
		}
		bytes, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		fields, err := Fields(bytes)
		if err != nil {
			t.Fatal(err)
		}
		for _, f := range fields {
			if f.Key == "previews.cache_max_bytes" {
				if f.Value != "inf" || !f.Explicit || !f.Unlimited {
					t.Fatal(f)
				}
			}
		}
	}
	if err = Update(path, "", map[string]any{"previews.cache_max_bytes": nil, "previews.cache_max_age": nil}); err != nil {
		t.Fatal(err)
	}
	bytes, _ := os.ReadFile(path)
	fields, err := Fields(bytes)
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range fields {
		if f.Key == "previews.cache_max_age" || f.Key == "previews.cache_max_bytes" {
			if f.Explicit {
				t.Fatal("reset pinned default", f)
			}
		}
	}
	if err = Update(path, "", map[string]any{"previews.cache_max_bytes": int64(32_000_000_000)}); err != nil {
		t.Fatal(err)
	}
	bytes, _ = os.ReadFile(path)
	fields, err = Fields(bytes)
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range fields {
		if f.Key == "previews.cache_max_bytes" && !f.Explicit {
			t.Fatal("explicit default became inherited")
		}
	}
	for _, value := range []any{int64(0), int64(-1), "wrong", float64(1.2)} {
		if err = Update(path, "", map[string]any{"previews.cache_max_bytes": value}); err == nil {
			t.Fatalf("accepted %v", value)
		}
	}
	if err = Update(path, "", map[string]any{"previews.cache_max_age": "0d"}); err == nil {
		t.Fatal("zero age accepted as infinity")
	}
}
