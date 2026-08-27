package domain

import (
	"fmt"
	"strings"
)

// SplitName cuts a file name into the part a user would call its name and the
// part a file manager uses to pick an icon.
//
// The dot has to be at index > 0: ".gitignore" is a name, not an extension,
// and treating it as one would produce " (2).gitignore" on a collision.
func SplitName(name string) (stem, ext string) {
	i := strings.LastIndex(name, ".")
	if i <= 0 {
		return name, ""
	}
	return name[:i], name[i:]
}

// NumberedName is the nth alternative for a taken name: "发票.jpg" becomes
// "发票 (2).jpg".
//
// Import uses it when a name is already in use by *different* content
// (spec 1.5). Identical content is skipped instead, which is what makes a
// re-run after an interrupted import converge rather than pile up copies
// (spec 3.7.1 ②).
func NumberedName(name string, n int) string {
	stem, ext := SplitName(name)
	return fmt.Sprintf("%s (%d)%s", stem, n, ext)
}
