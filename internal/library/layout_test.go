package library

import (
	"path/filepath"
	"testing"
)

func TestLayoutPaths(t *testing.T) {
	l := Layout{Root: filepath.Join("home", "kist")}

	tests := []struct {
		name string
		got  string
		want string
	}{
		{"DB", l.DB(), filepath.Join("home", "kist", "kist.db")},
		{"ConfigPath", l.ConfigPath(), filepath.Join("home", "kist", "kist.toml")},
		{"Blobs", l.Blobs(), filepath.Join("home", "kist", "blobs")},
		{"Boxes", l.Boxes(), filepath.Join("home", "kist", "boxes")},
		{"Derived", l.Derived(), filepath.Join("home", "kist", "derived")},
		{"Notice", l.Notice(), filepath.Join("home", "kist", "DO-NOT-EDIT.txt")},
	}
	for _, tt := range tests {
		if tt.got != tt.want {
			t.Errorf("%s() = %q, want %q", tt.name, tt.got, tt.want)
		}
	}
}
