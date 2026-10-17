package derive

import (
	"bytes"
	"context"
	"encoding/binary"
	"image"
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/hschendel/stl"
)

func stlFixture(t *testing.T, transform func(*stl.Solid)) string {
	t.Helper()
	solid, err := stl.ReadFile("testdata/cube.stl")
	if err != nil {
		t.Fatal(err)
	}
	solid.IsAscii = false
	if transform != nil {
		transform(solid)
	}
	path := filepath.Join(t.TempDir(), "model")
	if err = solid.WriteFile(path); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestSTLPreview(t *testing.T) {
	binaryPath := stlFixture(t, func(s *stl.Solid) { s.BinaryHeader = []byte("solid binary header is valid") })
	for _, path := range []string{"testdata/cube.stl", binaryPath} {
		for _, recipe := range []string{STLThumbnail, STLPreview} {
			out := filepath.Join(t.TempDir(), "preview.jpg")
			result, err := (Renderer{}).Render(t.Context(), path, out, recipe)
			if err != nil {
				t.Fatal(err)
			}
			size := 512
			if recipe == STLPreview {
				size = 2048
			}
			if result.Width != size || result.Height != size || result.Engine != "fauxgl" || result.Bytes == 0 {
				t.Fatal(result)
			}
			f, err := os.Open(out)
			if err != nil {
				t.Fatal(err)
			}
			img, _, err := image.Decode(f)
			f.Close()
			if err != nil {
				t.Fatal(err)
			}
			// The cube must be visible, shaded, and surrounded by a clean margin.
			colors := map[uint32]bool{}
			colored := 0
			for y := 0; y < size; y += 4 {
				for x := 0; x < size; x += 4 {
					r, g, b, _ := img.At(x, y).RGBA()
					if r < 55000 || g < 55000 || b < 55000 {
						colored++
						colors[(r>>12)<<8|(g>>12)<<4|(b>>12)] = true
						if x < size/20 || x > size*19/20 || y < size/20 || y > size*19/20 {
							t.Fatal("model clipped at border")
						}
					}
				}
			}
			if colored < size*size/160 || len(colors) < 3 {
				t.Fatalf("empty or unshaded model: %d pixels, %d colors", colored, len(colors))
			}
		}
	}
}

func TestSTLNormalization(t *testing.T) {
	var baseline []byte
	for _, scale := range []float32{1, 1e-20, 1e20} {
		path := stlFixture(t, func(s *stl.Solid) {
			for i := range s.Triangles {
				for j := range s.Triangles[i].Vertices {
					for k := range s.Triangles[i].Vertices[j] {
						s.Triangles[i].Vertices[j][k] = (s.Triangles[i].Vertices[j][k] + 16) * scale
					}
				}
			}
		})
		out := filepath.Join(t.TempDir(), "out")
		if _, err := (Renderer{}).Render(t.Context(), path, out, STLThumbnail); err != nil {
			t.Fatal(err)
		}
		raw, err := os.ReadFile(out)
		if err != nil {
			t.Fatal(err)
		}
		if baseline == nil {
			baseline = raw
		} else if !bytes.Equal(baseline, raw) {
			t.Fatal("units or translation changed preview")
		}
	}
}

func TestSTLInvalidAndLimits(t *testing.T) {
	raw, err := os.ReadFile("testdata/cube.stl")
	if err != nil {
		t.Fatal(err)
	}
	cases := map[string][]byte{"empty": {}, "not STL": []byte("some text"), "missing vertex": bytes.Replace(raw, []byte("vertex 0 0 0"), nil, 1), "truncated": raw[:len(raw)/2], "no triangles": []byte("solid empty\nendsolid empty\n")}
	for name, raw := range cases {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "invalid.stl")
			os.WriteFile(path, raw, 0600)
			if _, err := (Renderer{}).Render(t.Context(), path, filepath.Join(t.TempDir(), "out"), STLThumbnail); Code(err) != "decode_failed" {
				t.Fatalf("%v", err)
			}
		})
	}
	for _, bad := range []float32{float32(math.NaN()), float32(math.Inf(1))} {
		path := stlFixture(t, func(s *stl.Solid) { s.Triangles[0].Vertices[0][0] = bad })
		if _, err := (Renderer{}).Render(t.Context(), path, filepath.Join(t.TempDir(), "out"), STLThumbnail); Code(err) != "decode_failed" {
			t.Fatal(err)
		}
	}
	path := filepath.Join(t.TempDir(), "large")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	f.Truncate(maxSTLBytes + 1)
	f.Close()
	if _, err := (Renderer{}).Render(t.Context(), path, filepath.Join(t.TempDir(), "out"), STLThumbnail); Code(err) != "limit_exceeded" {
		t.Fatal(err)
	}
	f, err = os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	header := make([]byte, 84)
	binary.LittleEndian.PutUint32(header[80:], maxSTLTriangles+1)
	f.Write(header)
	f.Truncate(84 + 50*(maxSTLTriangles+1))
	f.Close()
	if _, err := (Renderer{}).Render(t.Context(), path, filepath.Join(t.TempDir(), "out"), STLThumbnail); Code(err) != "limit_exceeded" {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := (Renderer{}).Render(ctx, "testdata/cube.stl", filepath.Join(t.TempDir(), "out"), STLThumbnail); Code(err) != "cancelled" {
		t.Fatal(err)
	}
}
