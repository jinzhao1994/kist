package config

import (
	"testing"
	"time"
)

func TestCacheMaxAge(t *testing.T) {
	for value, want := range map[string]time.Duration{"180d": 180 * 24 * time.Hour, "1d": 24 * time.Hour, "720h": 720 * time.Hour, "1h30m": 90 * time.Minute} {
		got, err := ParseCacheMaxAge(value)
		if err != nil || got != want {
			t.Fatalf("%s: %v, %v", value, got, err)
		}
	}
	for _, value := range []string{"", "0d", "-1d", "1h2d", "106752d", "999999999999999999999d", "NaNd", "-1h"} {
		if _, err := ParseCacheMaxAge(value); err == nil {
			t.Fatalf("accepted %q", value)
		}
	}
	if got := Defaults("client").Client.CacheMaxAge; got != "180d" {
		t.Fatal(got)
	}
}

func TestCacheMaxAgeConfigRoundTrip(t *testing.T) {
	path := writeConfig(t, "version = 2\n[client]\ncache_max_age = '720h'\n")
	c, err := Load(path)
	if err != nil || c.Client.CacheMaxAge != "720h" {
		t.Fatal(c, err)
	}
	if err := Update(path, "", map[string]any{"client.cache_max_age": "180d"}); err != nil {
		t.Fatal(err)
	}
	c, err = Load(path)
	if err != nil || c.Client.CacheMaxAge != "180d" {
		t.Fatal(c, err)
	}
	if err := Update(path, "", map[string]any{"client.cache_max_age": nil}); err != nil {
		t.Fatal(err)
	}
	c, err = Load(path)
	if err != nil || c.Client.CacheMaxAge != "180d" {
		t.Fatal(c, err)
	}
}
