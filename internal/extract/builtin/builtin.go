// Package builtin registers the extractors this build ships with.
//
// It exists because registration happens in an init function, and an init
// function in a package nobody imports never runs. Without this, `kist
// extract` reports "extracted 0" on a library full of photographs and gives no
// hint why — which is exactly what happened the first time it was run by hand.
//
// library imports it rather than cmd, so that no binary linking library can
// forget: an API server would have hit the same silence.
package builtin

import (
	_ "github.com/jinzhao1994/kist/internal/extract/exif"
	_ "github.com/jinzhao1994/kist/internal/extract/media"
)
