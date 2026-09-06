// Package library owns a kist library: its directory layout, and — from M1b
// onward — the only path through which anything is written (spec 2.6).
package library

import (
	"fmt"
	"os"
	"path/filepath"
)

// Layout locates everything inside a library root.
type Layout struct {
	Root string
}

func (l Layout) DB() string         { return filepath.Join(l.Root, "kist.db") }
func (l Layout) ConfigPath() string { return filepath.Join(l.Root, "kist.toml") }
func (l Layout) Blobs() string      { return filepath.Join(l.Root, "blobs") }
func (l Layout) Ops() string        { return filepath.Join(l.Root, "ops") }
func (l Layout) Boxes() string      { return filepath.Join(l.Root, "boxes") }
func (l Layout) Derived() string    { return filepath.Join(l.Root, "derived") }
func (l Layout) Notice() string     { return filepath.Join(l.Root, "DO-NOT-EDIT.txt") }

// LockPath is the write lock (spec 2.7). Its contents are diagnostic only —
// the lock itself is held by the kernel, not by anything written in the file.
func (l Layout) LockPath() string { return filepath.Join(l.Root, "kist.lock") }

// OpenLayout resolves root and checks it actually holds a library.
func OpenLayout(root string) (Layout, error) {
	abs, err := filepath.Abs(root)
	if err != nil {
		return Layout{}, fmt.Errorf("resolve library path: %w", err)
	}
	l := Layout{Root: abs}
	if _, err := os.Stat(l.ConfigPath()); err != nil {
		return Layout{}, fmt.Errorf("%s is not a kist library (no kist.toml): %w", abs, ErrNotALibrary)
	}
	return l, nil
}

// isLibrary reports whether root already holds one.
func isLibrary(root string) bool {
	_, err := os.Stat(filepath.Join(root, "kist.toml"))
	return err == nil
}
