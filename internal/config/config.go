// Package config reads and writes kist.toml, the library's configuration.
package config

import (
	"fmt"
	"os"
	"time"

	"github.com/BurntSushi/toml"

	"github.com/jinzhao1994/kist/internal/blob"
	"github.com/jinzhao1994/kist/internal/domain"
)

// CurrentVersion is the config schema this build writes.
const CurrentVersion = 1

type Config struct {
	Version int      `toml:"version"`
	Library Library  `toml:"library"`
	Storage Storage  `toml:"storage"`
	Remotes []Remote `toml:"remote"`
}

// Remote is somewhere else the same blobs live: another directory, a mounted
// external drive, a network share.
//
// kist never writes to one. Backups are rsync's job (spec 3.11), and a remote
// is only ever read from — which is why it needs nothing but a path, and why
// mounting it read-only is the right thing to do.
//
// There is no `kist remote add`: the file says at the top that hand edits are
// read back, and adding a remote is a once-in-a-library operation.
type Remote struct {
	Name string `toml:"name"`
	Path string `toml:"path"`
}

type Library struct {
	// ID identifies this library. Nothing uses it in v0.1; it exists because
	// op ids become (instance_id, seq) when devices start syncing (spec 3.12),
	// and minting it at init is free while backfilling it later is not.
	ID domain.ID `toml:"id"`

	// Timezone is the IANA name this library assumes when a file's capture
	// time carries no zone of its own — a scan, a screenshot, an old camera
	// (spec 1.5.2).
	//
	// It is written out at init rather than left empty, for the same reason
	// 3.5 writes the link mode out: an empty value means "whatever this
	// machine says", and this one decides the order the user sees. Opening
	// the same library on a laptop set to UTC must not reorder it.
	//
	// Empty falls back to the machine's zone, so a hand-written config that
	// omits it still works.
	Timezone string `toml:"timezone,omitempty"`
}

// Zone resolves Timezone, falling back to the machine's.
//
// A name that will not load is an error rather than a silent fallback: a
// library configured for Asia/Shanghai that quietly behaved as UTC because
// the tzdata was missing would reorder every photo without saying so.
func (l Library) Zone() (*time.Location, error) {
	if l.Timezone == "" {
		return time.Local, nil
	}
	loc, err := time.LoadLocation(l.Timezone)
	if err != nil {
		return nil, fmt.Errorf("library timezone %q: %w", l.Timezone, err)
	}
	return loc, nil
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

	// Remotes are hand-written, so they are checked here rather than trusted.
	// A name is how repair says where a file came from, and two remotes with
	// one name would make that sentence a lie.
	seen := map[string]bool{}
	for i, r := range c.Remotes {
		switch {
		case r.Name == "":
			return nil, fmt.Errorf("read %s: remote %d has no name", path, i+1)
		case r.Path == "":
			return nil, fmt.Errorf("read %s: remote %q has no path", path, r.Name)
		case seen[r.Name]:
			return nil, fmt.Errorf("read %s: two remotes are called %q", path, r.Name)
		}
		seen[r.Name] = true
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
