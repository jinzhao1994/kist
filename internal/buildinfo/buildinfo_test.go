package buildinfo

import (
	"runtime"
	"strings"
	"testing"
)

func TestGetFillsDefaults(t *testing.T) {
	i := Get()

	// version is empty unless -ldflags injected it; Get must not leak that.
	if i.Version == "" {
		t.Error("Version is empty, want a fallback")
	}
	if i.Commit == "" {
		t.Error("Commit is empty, want a fallback")
	}
	if want := runtime.GOOS + "/" + runtime.GOARCH; i.Platform != want {
		t.Errorf("Platform = %q, want %q", i.Platform, want)
	}
	if i.Go != runtime.Version() {
		t.Errorf("Go = %q, want %q", i.Go, runtime.Version())
	}
}

func TestInfoString(t *testing.T) {
	i := Info{Version: "v0.1.0", Commit: "abc123def456", Go: "go1.26.1", Platform: "linux/amd64"}

	if got, want := i.String(), "kist v0.1.0 (abc123def456) go1.26.1 linux/amd64"; got != want {
		t.Errorf("String() = %q, want %q", got, want)
	}

	i.Dirty = true
	if got := i.String(); !strings.Contains(got, "abc123def456-dirty") {
		t.Errorf("String() = %q, want it to mark the build dirty", got)
	}
}
