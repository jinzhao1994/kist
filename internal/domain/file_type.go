package domain

import (
	"regexp"
	"strings"
)

var fileExtension = regexp.MustCompile(`(?i)\.([a-z0-9]{1,12})$`)

// DisplayFileType is the short type label shared by listings and sorting.
func DisplayFileType(name, mime string) string {
	if match := fileExtension.FindStringSubmatch(name); match != nil {
		return strings.ToUpper(match[1])
	}
	return map[string]string{"text/plain": "TXT", "application/pdf": "PDF", "image/jpeg": "JPG", "image/png": "PNG", "video/mp4": "MP4", "audio/mpeg": "MP3", "application/zip": "ZIP"}[strings.ToLower(strings.TrimSpace(strings.Split(mime, ";")[0]))]
}
