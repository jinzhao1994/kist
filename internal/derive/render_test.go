package derive

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"hash/crc32"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func fixture(t *testing.T, w, h int) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "input.png")
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	img.Set(0, 0, color.NRGBA{R: 255, A: 255})
	f, e := os.Create(p)
	if e != nil {
		t.Fatal(e)
	}
	if e = png.Encode(f, img); e != nil {
		t.Fatal(e)
	}
	f.Close()
	return p
}
func dimensions(t *testing.T, path string) image.Config {
	t.Helper()
	f, e := os.Open(path)
	if e != nil {
		t.Fatal(e)
	}
	defer f.Close()
	cfg, _, e := image.DecodeConfig(f)
	if e != nil {
		t.Fatal(e)
	}
	return cfg
}
func TestNativeRecipesWithoutExternalTools(t *testing.T) {
	r := Renderer{FFmpeg: filepath.Join(t.TempDir(), "absent"), FFprobe: "absent"}
	input := fixture(t, 1200, 800)
	for _, tc := range []struct {
		recipe string
		w, h   int
	}{{Thumbnail, 512, 341}, {Preview, 1200, 800}} {
		t.Run(tc.recipe, func(t *testing.T) {
			out := filepath.Join(t.TempDir(), "output")
			result, e := r.Render(t.Context(), input, out, tc.recipe)
			if e != nil {
				t.Fatal(e)
			}
			cfg := dimensions(t, out)
			if cfg.Width != tc.w || cfg.Height != tc.h {
				t.Fatal(cfg)
			}
			if result.Bytes <= 0 || result.GenerationMS < 0 || result.Width != cfg.Width {
				t.Fatal(result)
			}
			f, _ := os.Open(out)
			img, e := jpeg.Decode(f)
			f.Close()
			if e != nil {
				t.Fatal(e)
			}
			red, green, blue, _ := img.At(cfg.Width/2, cfg.Height/2).RGBA()
			if red < 65000 || green < 65000 || blue < 65000 {
				t.Fatal("transparent pixels were not composited onto white")
			}
		})
	}
}

// Build a JPEG with a single EXIF orientation entry, without personal photos.
func orientedJPEG(t *testing.T, orientation uint16) string {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 80, 40))
	for y := 0; y < 40; y++ {
		for x := 0; x < 80; x++ {
			c := color.RGBA{B: 255, A: 255}
			if x < 40 {
				c = color.RGBA{R: 255, A: 255}
			}
			img.Set(x, y, c)
		}
	}
	var data bytes.Buffer
	if e := jpeg.Encode(&data, img, &jpeg.Options{Quality: 100}); e != nil {
		t.Fatal(e)
	}
	exif := []byte{'E', 'x', 'i', 'f', 0, 0, 'I', 'I', 42, 0, 8, 0, 0, 0, 1, 0, 0x12, 0x01, 3, 0, 1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}
	binary.LittleEndian.PutUint16(exif[24:26], orientation)
	marker := []byte{0xff, 0xe1, 0, 0}
	binary.BigEndian.PutUint16(marker[2:], uint16(len(exif)+2))
	raw := append([]byte{}, data.Bytes()[:2]...)
	raw = append(raw, marker...)
	raw = append(raw, exif...)
	raw = append(raw, data.Bytes()[2:]...)
	path := filepath.Join(t.TempDir(), "oriented.jpg")
	if e := os.WriteFile(path, raw, 0600); e != nil {
		t.Fatal(e)
	}
	return path
}
func TestEXIFOrientation(t *testing.T) {
	for _, orientation := range []uint16{1, 2, 3, 4, 5, 6, 7, 8} {
		out := filepath.Join(t.TempDir(), "derived.jpg")
		if _, e := (Renderer{}).Render(t.Context(), orientedJPEG(t, orientation), out, Thumbnail); e != nil {
			t.Fatal(e)
		}
		cfg := dimensions(t, out)
		w, h := 80, 40
		if orientation >= 5 {
			w, h = 40, 80
		}
		if cfg.Width != w || cfg.Height != h {
			t.Fatalf("orientation %d: %v", orientation, cfg)
		}
		f, _ := os.Open(out)
		img, _ := jpeg.Decode(f)
		f.Close()
		x, y := 10, 10
		if orientation >= 5 {
			y = 5
		}
		red, _, blue, _ := img.At(x, y).RGBA()
		expectRed := orientation == 1 || orientation == 4 || orientation == 5 || orientation == 6
		if (red > blue) != expectRed {
			t.Fatalf("orientation %d: wrong corner color", orientation)
		}
	}
}
func TestLimitsCancellationAndMissingTools(t *testing.T) {
	if validDimensions(0, 1) || validDimensions(1<<30, 1<<30) || !validDimensions(8000, 6000) {
		t.Fatal("pixel guard")
	}
	input := fixture(t, 2, 2)
	out := filepath.Join(t.TempDir(), "output")
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, e := (Renderer{}).Render(ctx, input, out, Thumbnail); Code(e) != "cancelled" {
		t.Fatal(e)
	}
	if _, e := os.Stat(out); !os.IsNotExist(e) {
		t.Fatal("cancelled render published bytes")
	}
	if _, e := (Renderer{}).Render(t.Context(), input, out, "../../escape"); Code(e) != "unsupported" {
		t.Fatal(e)
	}
	if _, e := (Renderer{}).Render(t.Context(), input, out, Video); Code(e) != "unsupported" {
		t.Fatal(e)
	}
	raw := filepath.Join(t.TempDir(), "heic")
	os.WriteFile(raw, []byte("not a Go image"), 0600)
	if _, e := (Renderer{FFmpeg: filepath.Join(t.TempDir(), "absent")}).Render(t.Context(), raw, out, Thumbnail); Code(e) != "dependency_missing" {
		t.Fatal(e)
	}
	// Header dimensions are sufficient to reject a huge image before allocation.
	// A JPEG SOF mutation preserves a valid header without checksums.
	src := orientedJPEG(t, 1)
	data, e := os.ReadFile(src)
	if e != nil {
		t.Fatal(e)
	}
	for i := 0; i < len(data)-10; i++ {
		if data[i] == 0xff && data[i+1] == 0xc0 {
			binary.BigEndian.PutUint16(data[i+5:i+7], 65000)
			binary.BigEndian.PutUint16(data[i+7:i+9], 65000)
			break
		}
	}
	os.WriteFile(src, data, 0600)
	if _, e := (Renderer{}).Render(t.Context(), src, out, Thumbnail); Code(e) != "limit_exceeded" {
		t.Fatal(e)
	}
}
func TestRealFFmpegVideo(t *testing.T) {
	ffmpeg, e := exec.LookPath("ffmpeg")
	if e != nil {
		t.Skip("ffmpeg not installed")
	}
	if _, e = exec.LookPath("ffprobe"); e != nil {
		t.Skip("ffprobe not installed")
	}
	for _, extension := range []string{".mp4", ".mov"} {
		input := filepath.Join(t.TempDir(), "original"+extension)
		cmd := exec.Command(ffmpeg, "-v", "error", "-f", "lavfi", "-i", "color=c=blue:s=640x360:r=10", "-f", "lavfi", "-i", "sine=frequency=440", "-t", "0.5", "-c:v", "libx264", "-threads", "1", "-pix_fmt", "yuv420p", "-c:a", "aac", input)
		if output, e := cmd.CombinedOutput(); e != nil {
			t.Fatalf("fixture: %v %s", e, output)
		}
		for _, recipe := range []string{Thumbnail, Video} {
			out := filepath.Join(t.TempDir(), "result")
			result, e := (Renderer{}).Render(t.Context(), input, out, recipe)
			if e != nil {
				t.Fatal(e)
			}
			if result.Bytes == 0 || result.Width > 640 || result.Height > 360 || result.Engine != "ffmpeg" {
				t.Fatal(result)
			}
		}
	}
	// A playlist must not be dereferenced, even when ffmpeg supports HLS.
	playlist := filepath.Join(t.TempDir(), "untrusted.m3u8")
	os.WriteFile(playlist, []byte("#EXTM3U\n#EXT-X-TARGETDURATION:1\n#EXTINF:1,\nhttp://127.0.0.1:9/private\n#EXT-X-ENDLIST\n"), 0600)
	if _, e := (Renderer{}).Render(t.Context(), playlist, filepath.Join(t.TempDir(), "out"), Thumbnail); e == nil {
		t.Fatal("playlist accepted")
	}
}

func TestProcessCancellationAndBoundedDiagnostics(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	timer := time.AfterFunc(100*time.Millisecond, cancel)
	defer timer.Stop()
	start := time.Now()
	err := run(ctx, os.Args[0], []string{"-test.run=TestDeriveProcessHelper", "--", "derive-child"}, io.Discard)
	if !errors.Is(err, context.Canceled) || time.Since(start) > 3*time.Second {
		t.Fatalf("child was not stopped: %v", err)
	}
	b := &limitedBuffer{limit: 64}
	raw := bytes.Repeat([]byte("x"), 1024)
	if n, e := b.Write(raw); e != nil || n != len(raw) || b.Len() != 64 {
		t.Fatal(n, e, b.Len())
	}
}
func TestMissingDecoderPlugin(t *testing.T) {
	err := run(t.Context(), os.Args[0], []string{"-test.run=TestDeriveProcessHelper", "--", "derive-missing-codec"}, io.Discard)
	if Code(err) != "dependency_missing" {
		t.Fatalf("missing codec classified as file damage: %v", err)
	}
}

func TestDeriveProcessHelper(t *testing.T) {
	if os.Args[len(os.Args)-1] == "derive-missing-codec" {
		os.Stderr.WriteString("Could not decode image Unsupported feature: Unsupported codec")
		os.Exit(1)
	}
	if os.Args[len(os.Args)-1] != "derive-child" {
		return
	}
	time.Sleep(30 * time.Second)
	os.Exit(0)
}

func TestRealHEIC(t *testing.T) {
	if _, e := exec.LookPath("heif-info"); e != nil {
		t.Skip("libheif tools not installed")
	}
	for _, tc := range []struct {
		name string
		w, h int
	}{{"pattern.heic", 128, 96}, {"tiled.heic", 2048, 1365}} {
		t.Run(tc.name, func(t *testing.T) {
			out := filepath.Join(t.TempDir(), "preview.jpg")
			result, e := (Renderer{}).Render(t.Context(), filepath.Join("testdata", tc.name), out, Preview)
			if e != nil {
				t.Fatal(e)
			}
			if result.Width != tc.w || result.Height != tc.h || result.Engine != "libheif" {
				t.Fatal(result)
			}
			f, e := os.Open(out)
			if e != nil {
				t.Fatal(e)
			}
			defer f.Close()
			img, e := jpeg.Decode(f)
			if e != nil {
				t.Fatal(e)
			}
			if img.At(result.Width/4, result.Height/3) == img.At(3*result.Width/4, result.Height/3) {
				t.Fatal("test pattern lost its pixels")
			}
		})
	}
}
func TestHEIFMissingDependency(t *testing.T) {
	_, e := (Renderer{HeifInfo: filepath.Join(t.TempDir(), "absent")}).Render(t.Context(), "testdata/pattern.heic", filepath.Join(t.TempDir(), "out"), Thumbnail)
	if Code(e) != "dependency_missing" {
		t.Fatal(e)
	}
}

func TestPNGEXIFOrientation(t *testing.T) {
	source := fixture(t, 80, 40)
	data, e := os.ReadFile(source)
	if e != nil {
		t.Fatal(e)
	}
	jpg, e := os.ReadFile(orientedJPEG(t, 6))
	if e != nil {
		t.Fatal(e)
	}
	tiff := jpg[12 : 4+int(binary.BigEndian.Uint16(jpg[4:6]))]
	chunk := make([]byte, 12+len(tiff))
	binary.BigEndian.PutUint32(chunk[:4], uint32(len(tiff)))
	copy(chunk[4:8], "eXIf")
	copy(chunk[8:], tiff)
	binary.BigEndian.PutUint32(chunk[len(chunk)-4:], crc32.ChecksumIEEE(chunk[4:len(chunk)-4]))
	body := append([]byte{}, data[:33]...)
	body = append(body, chunk...)
	body = append(body, data[33:]...)
	os.WriteFile(source, body, 0600)
	out := filepath.Join(t.TempDir(), "out.jpg")
	result, e := (Renderer{}).Render(t.Context(), source, out, Thumbnail)
	if e != nil {
		t.Fatal(e)
	}
	if result.Width != 40 || result.Height != 80 {
		t.Fatal(result)
	}
}
