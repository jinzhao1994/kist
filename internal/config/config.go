// Package config reads and writes kist.toml, the library's configuration.
package config

import (
	"fmt"
	"os"

	"github.com/BurntSushi/toml"

	"github.com/jinzhao1994/kist/internal/blob"
	"github.com/jinzhao1994/kist/internal/domain"
)

// CurrentVersion is the config schema this build writes.
const CurrentVersion = 1

type Config struct {
	Version int     `toml:"version"`
	Library Library `toml:"library"`
	Storage Storage `toml:"storage"`
}

type Library struct {
	// ID identifies this library. Nothing uses it in v0.1; it exists because
	// op ids become (instance_id, seq) when devices start syncing (spec 3.12),
	// and minting it at init is free while backfilling it later is not.
	ID domain.ID `toml:"id"`
}

type Storage struct {
	// LinkMode is what init probed. Rechecked at startup: an external drive
	// can come back mounted differently.
	LinkMode blob.LinkMode `toml:"link_mode"`
}

// Default builds the configuration for a fresh library.
func Default(linkMode blob.LinkMode) *Config {
	c := &Config{Version: CurrentVersion}
	c.Library.ID = domain.NewID()
	c.Storage.LinkMode = linkMode
	return c
}

func Load(path string) (*Config, error) {
	var c Config
	meta, err := toml.DecodeFile(path, &c)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	if undecoded := meta.Undecoded(); len(undecoded) > 0 {
		return nil, fmt.Errorf("read %s: unknown setting %q", path, undecoded[0].String())
	}
	if c.Version > CurrentVersion {
		return nil, fmt.Errorf("read %s: config version %d was written by a newer kist; upgrade to open this library",
			path, c.Version)
	}
	switch c.Storage.LinkMode {
	case blob.LinkReflink, blob.LinkHardlink, blob.LinkSymlink, blob.LinkCopy, "":
	default:
		return nil, fmt.Errorf("read %s: unknown link_mode %q", path, c.Storage.LinkMode)
	}
	return &c, nil
}

func Save(path string, c *Config) error {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	defer f.Close()

	if _, err := f.WriteString("# kist library configuration.\n# Managed by kist; hand edits are read back on the next run.\n\n"); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	if err := toml.NewEncoder(f).Encode(c); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return f.Close()
}
