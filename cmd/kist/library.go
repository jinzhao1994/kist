package main

import (
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

	l, err := library.Open(path)
	if err != nil {
		return nil, fmt.Errorf("%w\n\nIf you have not made a library yet, run: kist init", err)
	}
	return l, nil
}
