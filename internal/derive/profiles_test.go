package derive

import "testing"

func TestPreviewProfilesMatchSupportedBrowserFormats(t *testing.T) {
	cases := []struct{ name, kind, mime, thumbnail, detail string }{
		{"photo.png", "image", "image/png", Thumbnail, ""},
		{"photo.PNG", "", "", Thumbnail, ""},
		{"photo.heic", "", "", Thumbnail, Preview},
		{"clip.mov", "", "", VideoThumbnail, Video},
		{"scan.pdf", "", "", Thumbnail, ""},
		{"renamed.png", "image", "image/heic", Thumbnail, Preview},
		{"photo.heic", "image", "image/heic", Thumbnail, Preview},
		{"clip.mov", "video", "video/quicktime", VideoThumbnail, Video},
		{"scan.pdf", "document", "application/pdf", Thumbnail, ""},
		{"REPORT.DOCX", "other", "", OfficeThumbnail, OfficePDF},
		{"slides", "other", "application/vnd.ms-powerpoint", SlidesThumbnail, SlidesPDF},
		{"track.GPX", "other", "", GPXMap, GPXMap},
		{"model.stl", "other", "", STLThumbnail, STLPreview},
		{"notes.txt", "text", "text/plain", "", ""},
		{"sheet.xlsx", "other", "", "", ""},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			profile, ok := PreviewFor(test.name, test.kind, test.mime)
			if ok != (test.thumbnail != "") || profile.Thumbnail != test.thumbnail || profile.Detail != test.detail {
				t.Fatal(profile, ok)
			}
		})
	}
	seen := map[string]bool{}
	for _, profile := range PreviewProfiles() {
		if seen[profile.ID] {
			t.Fatal("duplicate profile", profile.ID)
		}
		seen[profile.ID] = true
		for _, name := range []string{profile.Thumbnail, profile.Detail} {
			if name != "" {
				if _, ok := Lookup(name); !ok {
					t.Fatal("unknown preview recipe", name)
				}
			}
		}
	}
}
