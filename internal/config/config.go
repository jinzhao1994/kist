// Package config reads and writes kist.toml, the library's configuration.
package config

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/BurntSushi/toml"

	"github.com/jinzhao1994/kist/internal/ai"
	"github.com/jinzhao1994/kist/internal/blob"
	"github.com/jinzhao1994/kist/internal/domain"
)

// CurrentVersion is the config schema this build writes.
const CurrentVersion = 2

type AIOptions struct {
	AutoDesc    bool   `toml:"auto_desc" json:"auto_desc"`
	DescAfterOp int64  `toml:"desc_after_op" json:"-"`
	Model       string `toml:"model" json:"model"`
	AutoOCR     bool   `toml:"auto_ocr" json:"auto_ocr"`
	AfterOp     int64  `toml:"after_op" json:"-"`
	ProxyURL    string `toml:"proxy_url" json:"proxy_url"`
}

type Config struct {
	AIOptions  AIOptions      `toml:"ai_options" json:"-"`
	AI         ai.Credentials `toml:"ai" json:"-"`
	Version    int            `toml:"version"`
	Mode       string         `toml:"mode,omitempty"`
	Appearance Appearance     `toml:"appearance"`
	Client     Client         `toml:"client"`
	Server     Server         `toml:"server"`
	Sync       Sync           `toml:"sync"`
	Library    Library        `toml:"library"`
	Storage    Storage        `toml:"storage"`
	Remotes    []Remote       `toml:"remote"`
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
	Language string `toml:"language"`
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
	// Legacy empty values resolve against the machine during migration.
	// Version 2 defaults to UTC and rejects an explicitly empty zone.
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
//
// The timezone is written out rather than left empty, for the same reason 3.5
// writes the link mode out: an empty value means "whatever this machine says",
// and this one decides the order the user sees. A library created on a laptop
// and later opened on a server set to UTC must not reorder itself.
func Default(linkMode blob.LinkMode) *Config {
	c := Defaults("library")
	c.Library.ID = domain.NewID()
	if zone := localZoneName(); zone != "" {
		c.Library.Timezone = zone
	}
	c.Storage.LinkMode = linkMode
	return c
}

// localZoneName is the machine's zone as an IANA name, or "" when it cannot be
// named.
//
// time.Local is no help: its String is always "Local", and time.Now().Zone()
// gives an abbreviation like "CST" that several real zones share. So the name
// has to come from where the operating system keeps it — $TZ, or what
// /etc/localtime points at.
//
// Initialization uses UTC if no portable IANA name can be detected.
func localZoneName() string {
	if tz := os.Getenv("TZ"); tz != "" {
		if _, err := time.LoadLocation(tz); err == nil {
			return tz
		}
	}
	target, err := os.Readlink("/etc/localtime")
	if err != nil {
		return ""
	}
	// .../zoneinfo/Asia/Shanghai -> Asia/Shanghai
	i := strings.Index(target, "zoneinfo/")
	if i < 0 {
		return ""
	}
	name := target[i+len("zoneinfo/"):]
	if _, err := time.LoadLocation(name); err != nil {
		return ""
	}
	return name
}

func Load(path string) (*Config, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return Decode(raw)
}

func Decode(raw []byte) (*Config, error) {
	c := *Defaults("library")
	// Configurations predating explicit versions follow the legacy migration.
	c.Version = 1
	meta, err := toml.Decode(string(raw), &c)
	path := "kist.toml"
	if err != nil {
		return nil, fmt.Errorf("read %s: invalid TOML syntax or value type", path)
	}
	if undecoded := meta.Undecoded(); len(undecoded) > 0 {
		return nil, fmt.Errorf("read %s: unknown setting %q", path, undecoded[0].String())
	}
	if c.Version < 2 && (!meta.IsDefined("library", "timezone") || c.Library.Timezone == "") {
		c.Library.Timezone = localZoneName()
	}
	if c.Version < 1 {
		return nil, fmt.Errorf("read %s: unsupported config version", path)
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
	if c.Mode == "client" && !meta.IsDefined("server", "listen") {
		c.Server.Listen = "127.0.0.1:8474"
	}
	if err := c.Validate(); err != nil {
		return nil, err
	}
	return &c, nil
}

// Save initializes a configuration file. Existing configurations should be
// modified through Update to preserve unrelated settings and detect conflicts.
func Save(path string, c *Config) error {
	raw, err := sparse(c)
	if err != nil {
		return err
	}
	return WriteAtomic(path, raw)
}
