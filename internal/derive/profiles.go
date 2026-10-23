package derive

import (
	"path/filepath"
	"slices"
	"strings"
)

// PreviewProfile selects browser preview recipes from catalog facts. The same
// ordered table is generated into the web bundle; changing a recipe cannot
// silently leave offline preparation using a different dependency list.
type PreviewProfile struct {
	ID         string   `json:"id"`
	Extensions []string `json:"extensions"`
	MediaTypes []string `json:"media_types"`
	Kind       string   `json:"kind"`
	Thumbnail  string   `json:"thumbnail"`
	Detail     string   `json:"detail"`
}

func PreviewProfiles() []PreviewProfile {
	profiles := []PreviewProfile{
		{ID: "slides", Extensions: []string{".ppt", ".pptx"}, MediaTypes: []string{"application/vnd.ms-powerpoint", "application/vnd.openxmlformats-officedocument.presentationml.presentation"}, Thumbnail: SlidesThumbnail, Detail: SlidesPDF},
		{ID: "word", Extensions: []string{".doc", ".docx"}, MediaTypes: []string{"application/msword", "application/vnd.openxmlformats-officedocument.wordprocessingml.document"}, Thumbnail: OfficeThumbnail, Detail: OfficePDF},
		{ID: "gpx", Extensions: []string{".gpx"}, MediaTypes: []string{"application/gpx+xml"}, Thumbnail: GPXMap, Detail: GPXMap},
		{ID: "stl", Extensions: []string{".stl"}, MediaTypes: []string{"model/stl"}, Thumbnail: STLThumbnail, Detail: STLPreview},
		{ID: "pdf", Extensions: []string{".pdf"}, MediaTypes: []string{"application/pdf"}, Thumbnail: Thumbnail},
		{ID: "native-image", Extensions: []string{".jpg", ".jpeg", ".png", ".gif", ".webp", ".bmp", ".ico"}, Kind: "image", MediaTypes: []string{"image/jpeg", "image/png", "image/gif", "image/webp", "image/bmp", "image/x-icon"}, Thumbnail: Thumbnail},
		{ID: "image", Extensions: []string{".heic", ".heif", ".tif", ".tiff", ".avif"}, Kind: "image", Thumbnail: Thumbnail, Detail: Preview},
		{ID: "video", Extensions: []string{".mp4", ".mov", ".m4v", ".webm", ".mkv", ".avi", ".mpeg", ".mpg"}, Kind: "video", Thumbnail: VideoThumbnail, Detail: Video},
	}
	for i := range profiles {
		if profiles[i].Extensions == nil {
			profiles[i].Extensions = []string{}
		}
		if profiles[i].MediaTypes == nil {
			profiles[i].MediaTypes = []string{}
		}
	}
	return profiles
}

func PreviewFor(name, kind, mime string) (PreviewProfile, bool) {
	extension := strings.ToLower(filepath.Ext(name))
	for _, profile := range PreviewProfiles() {
		extensionMatch := slices.Contains(profile.Extensions, extension) && (profile.Kind == "" || mime == "")
		kindMatch := profile.Kind != "" && profile.Kind == kind
		if profile.Kind != "" && !kindMatch && !(kind == "" && extensionMatch) {
			continue
		}
		if len(profile.Extensions)+len(profile.MediaTypes) > 0 && !extensionMatch && !slices.Contains(profile.MediaTypes, mime) && !(kindMatch && len(profile.MediaTypes) == 0) {
			continue
		}
		return profile, true
	}
	return PreviewProfile{}, false
}
