//go:build !windows

package main

import osexec "os/exec"

func containClientRenderer(_ *osexec.Cmd) (func() error, error) {
	return func() error { return nil }, nil
}
