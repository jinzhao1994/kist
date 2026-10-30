package api

import (
	"net/http/httptest"
	"net/url"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jinzhao1994/kist/internal/domain"
	"github.com/jinzhao1994/kist/internal/library"
	"github.com/jinzhao1994/kist/internal/wire"
)

func TestHLSHTTPOnDemandRangesAndVersion(t *testing.T) {
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("ffmpeg unavailable")
	}
	if _, err = exec.LookPath("ffprobe"); err != nil {
		t.Skip("ffprobe unavailable")
	}
	s, l := testServer(t)
	ctx := t.Context()
	box, err := l.CreateBox(ctx, "synthetic video", "", domain.LocalDate{}, domain.LocalDate{})
	if err != nil {
		t.Fatal(err)
	}
	input := filepath.Join(t.TempDir(), "synthetic.mp4")
	if out, err := exec.CommandContext(ctx, ffmpeg, "-v", "error", "-f", "lavfi", "-i", "testsrc2=size=160x90:rate=30", "-t", "7", "-c:v", "mpeg4", "-threads", "1", input).CombinedOutput(); err != nil {
		t.Fatalf("fixture: %v %s", err, out)
	}
	if _, err = l.Import(ctx, input, library.ImportOptions{Box: box.ID}); err != nil {
		t.Fatal(err)
	}
	files := decodeBody[wire.FileList](t, s, "/files")
	f := files.Files[0]
	base := Prefix + "/files/" + f.ID + "/playback/"
	w := call(s, "GET", base+"index.m3u8?v="+url.QueryEscape(f.BlobKey), "", "")
	if w.Code != 200 || w.Header().Get("Content-Type") != "application/vnd.apple.mpegurl" || !strings.Contains(w.Body.String(), "#EXTINF:1.000000,") {
		t.Fatal(w.Code, w.Body.String())
	}
	usage, err := l.StorageUsage(ctx)
	if err != nil || usage.VideoBytes != 0 {
		t.Fatal("playlist generated segments", usage, err)
	}
	lines := strings.Split(w.Body.String(), "\n")
	var last string
	for _, line := range lines {
		if strings.HasPrefix(line, "segments/") {
			last = line
		}
	}
	if !strings.HasPrefix(last, "segments/1?") {
		t.Fatal(last)
	}
	r := httptest.NewRequest("GET", "http://localhost:8473"+base+last, nil)
	r.Header.Set("Range", "bytes=0-187")
	w = httptest.NewRecorder()
	s.ServeHTTP(w, r)
	if w.Code != 206 || w.Body.Len() != 188 || w.Body.Bytes()[0] != 0x47 || w.Header().Get("Content-Type") != "video/mp2t" {
		t.Fatal(w.Code, w.Body.String())
	}
	if w.Header().Get("Cache-Control") != "private, no-cache" {
		t.Fatal(w.Header())
	}
	conditional := httptest.NewRequest("GET", "http://localhost:8473"+base+last, nil)
	conditional.Header.Set("If-None-Match", w.Header().Get("ETag"))
	validated := httptest.NewRecorder()
	s.ServeHTTP(validated, conditional)
	if validated.Code != 304 {
		t.Fatal(validated.Code, validated.Body.String())
	}
	if got := call(s, "HEAD", base+last, "", ""); got.Code != 200 || got.Body.Len() != 0 {
		t.Fatal(got.Code, got.Body.String())
	}
	if got := call(s, "GET", base+"index.m3u8?v=old", "", ""); got.Code != 409 {
		t.Fatal(got.Code, got.Body.String())
	}
	if got := call(s, "GET", base+"segments/0?v=old", "", ""); got.Code != 409 {
		t.Fatal(got.Code, got.Body.String())
	}
	if got := call(s, "POST", Prefix+"/settings/video-cache/clear", "", `{}`); got.Code != 200 {
		t.Fatal(got.Code, got.Body.String())
	}
}
