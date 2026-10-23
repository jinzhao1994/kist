package main

import (
	"context"
	"github.com/jinzhao1994/kist/internal/api"
	"io"
)

// Windows has no SIGHUP; settings saved through the API still reload directly.
func watchConfigReload(context.Context, *api.Server, io.Writer) func() { return func() {} }
