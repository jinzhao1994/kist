package domain

import "testing"

// The numbered form is what import falls back to when a name is taken by
// different content (spec 1.5). The number goes before the extension, because
// "发票.jpg (2)" is not a jpeg as far as any file manager is concerned.
func TestNumberedName(t *testing.T) {
	cases := []struct {
		name string
		n    int
		want string
	}{
		{"发票.jpg", 2, "发票 (2).jpg"},
		{"发票.jpg", 10, "发票 (10).jpg"},
		{"README", 2, "README (2)"},
		{"archive.tar.gz", 2, "archive.tar (2).gz"},
		// A leading dot is the whole name, not an extension: ".gitignore (2)"
		// is right and " (2).gitignore" is not.
		{".gitignore", 2, ".gitignore (2)"},
		{"..", 2, ". (2)."},
	}
	for _, c := range cases {
		if got := NumberedName(c.name, c.n); got != c.want {
			t.Errorf("NumberedName(%q, %d) = %q, want %q", c.name, c.n, got, c.want)
		}
	}
}

func TestSplitName(t *testing.T) {
	cases := []struct{ in, stem, ext string }{
		{"发票.jpg", "发票", ".jpg"},
		{"README", "README", ""},
		{"archive.tar.gz", "archive.tar", ".gz"},
		{".gitignore", ".gitignore", ""},
		{"trailing.", "trailing", "."},
	}
	for _, c := range cases {
		stem, ext := SplitName(c.in)
		if stem != c.stem || ext != c.ext {
			t.Errorf("SplitName(%q) = (%q, %q), want (%q, %q)", c.in, stem, ext, c.stem, c.ext)
		}
	}
}
