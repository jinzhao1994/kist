package main

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/jinzhao1994/kist/internal/library"
)

// libraryFlag is the value of --library, shared by every command that needs an
// open library.
var libraryFlag string

// openLibrary resolves --library and opens it.
//
// The error when the path is not a library names `kist init`, because that is
// the next thing the user has to do and making them go look it up is a small
// unkindness a self-hosted tool cannot afford to repeat.
func openLibrary(cmd *cobra.Command) (*library.Library, error) {
	return open(cmd)
}

// openForChecks opens without Open's crash recovery, for the three commands
// that promise not to write to the library (spec 3.8). They report a database
// that is behind the log rather than quietly finishing the write.
func openForChecks(cmd *cobra.Command) (*library.Library, error) {
	return open(cmd, library.WithoutRecovery())
}

func open(cmd *cobra.Command, opts ...library.Option) (*library.Library, error) {
	path := libraryFlag
	if path == "" {
		def, err := defaultLibraryPath()
		if err != nil {
			return nil, err
		}
		path = def
	}
	path, err := expandHome(path)
	if err != nil {
		return nil, err
	}

	l, err := library.Open(path, opts...)
	if err != nil {
		if errors.Is(err, library.ErrNotALibrary) {
			return nil, fmt.Errorf("%w\n\nIf you have not made a library yet, run: kist init", err)
		}
		return nil, err
	}
	return l, nil
}
