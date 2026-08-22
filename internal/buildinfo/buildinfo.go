// Package buildinfo reports the version of the running binary.
package buildinfo

import (
	"fmt"
	"runtime"
	"runtime/debug"
)

// Injected by the release workflow via -ldflags "-X". Release builds set
// Version to the git tag; everything else falls back to VCS data that the Go
// toolchain stamps into the binary automatically.
var version string

// Info describes the running binary.
type Info struct {
	Version  string // "v0.1.0" for tagged builds, "dev" otherwise
	Commit   string // short commit hash, "unknown" if not built from a checkout
	Dirty    bool   // built from a working tree with uncommitted changes
	Go       string // toolchain that built it
	Platform string // GOOS/GOARCH
}

// Get reads the build info stamped into the binary.
func Get() Info {
	i := Info{
		Version:  version,
		Commit:   "unknown",
		Go:       runtime.Version(),
		Platform: runtime.GOOS + "/" + runtime.GOARCH,
	}
	if i.Version == "" {
		i.Version = "dev"
	}

	bi, ok := debug.ReadBuildInfo()
	if !ok {
		return i
	}
	for _, s := range bi.Settings {
		switch s.Key {
		case "vcs.revision":
			if len(s.Value) >= 12 {
				i.Commit = s.Value[:12]
			} else if s.Value != "" {
				i.Commit = s.Value
			}
		case "vcs.modified":
			i.Dirty = s.Value == "true"
		}
	}
	return i
}

// String renders one line, e.g. "kist dev (abc123def456-dirty) go1.26.1 darwin/arm64".
func (i Info) String() string {
	commit := i.Commit
	if i.Dirty {
		commit += "-dirty"
	}
	return fmt.Sprintf("kist %s (%s) %s %s", i.Version, commit, i.Go, i.Platform)
}
