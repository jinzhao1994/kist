package extract

import (
	"io"
	"net/http"
)

// DetectMedia recognizes file bytes without trusting the filename. The ISO BMFF
// brands extend net/http's sniffer for HEIF images and QuickTime videos.
func DetectMedia(r io.Reader) (string, error) {
	var head [512]byte
	n, err := io.ReadFull(r, head[:])
	if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
		return "", err
	}
	if n == 0 {
		return "application/octet-stream", nil
	}
	if n >= 12 && string(head[4:8]) == "ftyp" {
		switch string(head[8:12]) {
		case "qt  ":
			return "video/quicktime", nil
		case "heic", "heix", "hevc", "hevx", "heim", "heis", "hevm", "hevs":
			return "image/heic", nil
		case "mif1", "msf1":
			return "image/heif", nil
		case "avif", "avis":
			return "image/avif", nil
		}
	}
	return http.DetectContentType(head[:n]), nil
}
