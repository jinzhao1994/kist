package derive

import (
	"encoding/binary"
	"image"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestVideoSampling(t *testing.T) {
	times := sampleTimes(3600)
	seen := map[float64]bool{}
	for _, v := range times {
		if seen[v] || v < 180 || v > 3420 {
			t.Fatalf("invalid sample %v", v)
		}
		seen[v] = true
	}
	if len(times) != 100 || times[1] >= 1800 || times[2] <= 1800 || len(sampleTimes(2)) != 4 {
		t.Fatalf("sampling distribution: %v", times)
	}
	red, blue := [48]float64{1}, [48]float64{0, 1}
	candidates := []videoCandidate{{black: true}, {histogram: red, time: 1}, {histogram: blue, time: 4}, {histogram: blue, time: 6}, {histogram: blue, time: 8}}
	if got := representative(candidates, 12); got != 3 {
		t.Fatalf("picked intro/black instead of representative body: %d", got)
	}
	if representative([]videoCandidate{{black: true}, {black: true}}, 10) != 0 {
		t.Fatal("all-black fallback")
	}
}

func testAtom(kind string, payload []byte) []byte {
	b := make([]byte, 8+len(payload))
	binary.BigEndian.PutUint32(b, uint32(len(b)))
	copy(b[4:], kind)
	copy(b[8:], payload)
	return b
}

func TestQuickTimePoster(t *testing.T) {
	for _, version := range []byte{0, 1} {
		for _, tc := range []struct {
			name, brand  string
			ticks, scale uint32
			valid        bool
		}{
			{"valid", "qt  ", 2500, 1000, true}, {"unset", "qt  ", 0, 1000, false}, {"outside", "qt  ", 10000, 1000, false}, {"zero scale", "qt  ", 2500, 0, false}, {"reserved MP4", "isom", 2500, 1000, false}, {"negative", "qt  ", 0xffffffff, 1000, false},
		} {
			t.Run(tc.name+string(rune('0'+version)), func(t *testing.T) {
				mvhd := make([]byte, 108)
				mvhd[0] = version
				scale, poster := 12, 80
				if version == 1 {
					scale, poster = 20, 92
				}
				binary.BigEndian.PutUint32(mvhd[scale:], tc.scale)
				binary.BigEndian.PutUint32(mvhd[poster:], tc.ticks)
				raw := append(testAtom("ftyp", append([]byte(tc.brand), 0, 0, 0, 0)), testAtom("moov", testAtom("mvhd", mvhd))...)
				path := filepath.Join(t.TempDir(), "movie")
				if err := os.WriteFile(path, raw, 0600); err != nil {
					t.Fatal(err)
				}
				got, ok := quickTimePoster(path, 10)
				if ok != tc.valid || ok && got != 2.5 {
					t.Fatalf("%v %v", got, ok)
				}
				for _, bad := range [][]byte{raw[:len(raw)-1], testAtom("moov", []byte{0, 0, 0, 1, 'm', 'v', 'h', 'd'}), {0, 0, 0, 1, 'm', 'o', 'o', 'v', 255, 255, 255, 255, 255, 255, 255, 255}} {
					if err := os.WriteFile(path, bad, 0600); err != nil {
						t.Fatal(err)
					}
					if _, ok := quickTimePoster(path, 10); ok {
						t.Fatal("accepted malformed atom")
					}
				}
			})
		}
	}
}

func TestRealVideoThumbnailSelection(t *testing.T) {
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("ffmpeg not installed")
	}
	if _, err = exec.LookPath("ffprobe"); err != nil {
		t.Skip("ffprobe not installed")
	}
	dir := t.TempDir()
	command := func(args ...string) {
		t.Helper()
		args = append([]string{"-v", "error", "-y"}, args...)
		if out, err := exec.Command(ffmpeg, args...).CombinedOutput(); err != nil {
			t.Fatalf("fixture: %v %s", err, out)
		}
	}
	movie := filepath.Join(dir, "movie.mov")
	command("-f", "lavfi", "-i", "color=black:s=320x180:r=2:d=1", "-f", "lavfi", "-i", "color=red:s=320x180:r=2:d=1", "-f", "lavfi", "-i", "color=blue:s=320x180:r=2:d=58", "-filter_complex", "[0:v][1:v][2:v]concat=n=3:v=1:a=0[v]", "-map", "[v]", "-c:v", "libx264", "-threads", "1", "-pix_fmt", "yuv420p", movie)
	check := func(input, recipe, want string) {
		t.Helper()
		out := filepath.Join(t.TempDir(), "thumb.jpg")
		start := time.Now()
		if _, err := (Renderer{}).Render(t.Context(), input, out, recipe); err != nil {
			t.Fatal(err)
		}
		t.Logf("%s: %s", want, time.Since(start))
		f, err := os.Open(out)
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		img, _, err := image.Decode(f)
		if err != nil {
			t.Fatal(err)
		}
		r, g, b, _ := img.At(img.Bounds().Dx()/2, img.Bounds().Dy()/2).RGBA()
		ok := false
		switch want {
		case "blue":
			ok = b > 40000 && r < 5000 && g < 5000
		case "red":
			ok = r > 40000 && g < 5000 && b < 5000
		case "black":
			ok = r < 5000 && g < 5000 && b < 5000
		}
		if !ok {
			t.Fatalf("want %s got RGB %d %d %d", want, r, g, b)
		}
	}
	check(movie, Thumbnail, "black")
	check(movie, VideoThumbnail, "blue")
	// An explicit QuickTime poster inside the intro takes priority over sampling.
	raw, err := os.ReadFile(movie)
	if err != nil {
		t.Fatal(err)
	}
	for i := 4; i+88 < len(raw); i++ {
		if string(raw[i:i+4]) == "mvhd" {
			scale := binary.BigEndian.Uint32(raw[i+16:])
			binary.BigEndian.PutUint32(raw[i+84:], scale+scale/2)
			break
		}
	}
	poster := filepath.Join(dir, "poster.mov")
	if err = os.WriteFile(poster, raw, 0600); err != nil {
		t.Fatal(err)
	}
	check(poster, VideoThumbnail, "red")
	cover := filepath.Join(dir, "cover.jpg")
	command("-f", "lavfi", "-i", "color=red:s=120x180", "-frames:v", "1", "-threads", "1", cover)
	for _, ext := range []string{"mp4", "mkv"} {
		attached := filepath.Join(dir, "attached."+ext)
		if ext == "mp4" {
			command("-i", movie, "-i", cover, "-map", "0:v", "-map", "1:v", "-c", "copy", "-disposition:v:1", "attached_pic", attached)
		} else {
			command("-i", movie, "-c", "copy", "-attach", cover, "-metadata:s:t", "mimetype=image/jpeg", "-metadata:s:t", "filename=cover.jpg", attached)
		}
		check(attached, VideoThumbnail, "red")
	}
}
