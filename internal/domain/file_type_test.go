package domain

import "testing"

func TestDisplayFileType(t *testing.T) {
	for _, tc := range []struct{ name, mime, want string }{
		{"photo.JpG", "application/pdf", "JPG"},
		{"report", "application/pdf; charset=utf-8", "PDF"},
		{"README", "text/plain", "TXT"},
		{"unknown", "application/octet-stream", ""},
		{"archive.tar.gz", "", "GZ"},
		{"name.扩展名", "", ""},
	} {
		if got := DisplayFileType(tc.name, tc.mime); got != tc.want {
			t.Errorf("%s: %q != %q", tc.name, got, tc.want)
		}
	}
}
