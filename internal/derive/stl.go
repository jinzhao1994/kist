package derive

import (
	"bytes"
	"context"
	"encoding/binary"
	"image/jpeg"
	"io"
	"math"
	"os"
	"time"

	"github.com/disintegration/imaging"
	"github.com/fogleman/fauxgl"
	"github.com/hschendel/stl"
)

const (
	maxSTLBytes     = 32 << 20
	maxSTLTriangles = 200_000
	maxSTLPixelWork = 128_000_000
)

// stlInput also receives parser callbacks. Stop retaining triangles immediately
// on a limit or invalid coordinate; the next buffered read stops the parser.
// Names, embedded normals and vendor color attributes are not trusted or used.
type stlInput struct {
	*os.File
	ctx       context.Context
	err       error
	triangles [][3]fauxgl.Vector
}

func (s *stlInput) Read(p []byte) (int, error) {
	if err := s.ctx.Err(); err != nil {
		return 0, err
	}
	if s.err != nil {
		return 0, s.err
	}
	return s.File.Read(p)
}
func (*stlInput) SetName(string)         {}
func (*stlInput) SetBinaryHeader([]byte) {}
func (*stlInput) SetASCII(bool)          {}
func (s *stlInput) SetTriangleCount(n uint32) {
	if n > maxSTLTriangles {
		s.err = fail("limit_exceeded", "STL exceeds triangle limit")
	}
}
func (s *stlInput) AppendTriangle(t stl.Triangle) {
	if s.err != nil {
		return
	}
	if err := s.ctx.Err(); err != nil {
		s.err = err
		return
	}
	if len(s.triangles) >= maxSTLTriangles {
		s.err = fail("limit_exceeded", "STL exceeds triangle limit")
		return
	}
	var points [3]fauxgl.Vector
	for i, v := range t.Vertices {
		for _, n := range v {
			if math.IsNaN(float64(n)) || math.IsInf(float64(n), 0) {
				s.err = fail("decode_failed", "STL has non-finite vertices")
				return
			}
		}
		points[i] = fauxgl.V(float64(v[0]), float64(v[1]), float64(v[2]))
	}
	s.triangles = append(s.triangles, points)
}

func readSTL(ctx context.Context, f *os.File) ([][3]fauxgl.Vector, error) {
	info, err := f.Stat()
	if err != nil {
		return nil, err
	}
	if info.Size() > maxSTLBytes {
		return nil, fail("limit_exceeded", "STL exceeds input limit")
	}
	var header [84]byte
	n, _ := f.ReadAt(header[:], 0)
	binaryFile := n == 84 && 84+50*int64(binary.LittleEndian.Uint32(header[80:])) == info.Size()
	if binaryFile {
		if binary.LittleEndian.Uint32(header[80:]) > maxSTLTriangles {
			return nil, fail("limit_exceeded", "STL exceeds triangle limit")
		}
	} else if !bytes.HasPrefix(header[:n], []byte("solid ")) {
		return nil, fail("decode_failed", "invalid STL header or binary length")
	}
	if _, err = f.Seek(0, io.SeekStart); err != nil {
		return nil, err
	}
	input := &stlInput{File: f, ctx: ctx}
	err = stl.CopyAll(input, input)
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	if input.err != nil {
		return nil, input.err
	}
	if err != nil {
		return nil, &Failure{"decode_failed", err}
	}
	if len(input.triangles) == 0 {
		return nil, fail("decode_failed", "STL contains no triangles")
	}
	return input.triangles, nil
}

// stl renders a static, orthographic view. DrawTriangle is deliberately serial:
// cancellation is checked between bounded triangles, and upstream DrawTriangles
// would create CPU-wide goroutines and race on its optimistic depth reads.
func (r Renderer) stl(ctx context.Context, input, output string, spec Recipe) (Result, error) {
	ctx, cancel := context.WithTimeout(ctx, 90*time.Second)
	defer cancel()
	f, err := os.Open(input)
	if err != nil {
		return Result{}, err
	}
	defer f.Close()
	triangles, err := readSTL(ctx, f)
	if err != nil {
		return Result{}, err
	}
	lo, hi := triangles[0][0], triangles[0][0]
	for _, t := range triangles {
		for _, p := range t {
			lo = lo.Min(p)
			hi = hi.Max(p)
		}
	}
	span := hi.Sub(lo)
	extent := math.Max(span.X, math.Max(span.Y, span.Z))
	if extent <= 0 {
		return Result{}, fail("decode_failed", "STL has no surface")
	}
	center := lo.Add(hi).MulScalar(0.5)
	// Normalize before projection to make units, offsets and tiny/large models
	// irrelevant, and to keep all downstream arithmetic finite.
	view := fauxgl.LookAt(fauxgl.V(3, -4, 3), fauxgl.Vector{}, fauxgl.V(0, 0, 1))
	pmin, pmax := fauxgl.V(math.Inf(1), math.Inf(1), math.Inf(1)), fauxgl.V(math.Inf(-1), math.Inf(-1), math.Inf(-1))
	for i, t := range triangles {
		if i%256 == 0 && ctx.Err() != nil {
			return Result{}, ctx.Err()
		}
		for j, p := range t {
			p = view.MulPosition(p.Sub(center).DivScalar(extent))
			triangles[i][j] = p
			pmin = pmin.Min(p)
			pmax = pmax.Max(p)
		}
	}
	mid := pmin.Add(pmax).MulScalar(0.5)
	width := math.Max(pmax.X-pmin.X, pmax.Y-pmin.Y)
	if width <= 0 {
		return Result{}, fail("decode_failed", "STL has no visible surface")
	}
	scale := 1.6 / width
	// Supersample small thumbnails, keeping the largest buffer at 2048 square.
	size := max(1024, spec.Size)
	dc := fauxgl.NewContext(size, size)
	dc.ClearColorBufferWith(fauxgl.HexColor("#f3f4f6"))
	dc.Cull = fauxgl.CullNone
	dc.AlphaBlend = false
	shader := fauxgl.NewSolidColorShader(fauxgl.Identity(), fauxgl.White)
	dc.Shader = shader
	light := fauxgl.V(-0.4, 0.6, 1).Normalize()
	work, drawn := int64(0), uint64(0)
	for _, t := range triangles {
		if err = ctx.Err(); err != nil {
			return Result{}, err
		}
		for j, p := range t {
			p = p.Sub(mid)
			t[j] = fauxgl.V(p.X*scale, p.Y*scale, -p.Z*0.5)
		}
		normal := t[1].Sub(t[0]).Cross(t[2].Sub(t[0]))
		if normal.Length() == 0 {
			continue
		}
		normal = normal.Normalize()
		if normal.Z < 0 {
			normal = normal.Negate()
		}
		boxLo, boxHi := t[0].Min(t[1]).Min(t[2]), t[0].Max(t[1]).Max(t[2])
		work += int64(math.Ceil((boxHi.X-boxLo.X)*float64(size)/2)+2) * int64(math.Ceil((boxHi.Y-boxLo.Y)*float64(size)/2)+2)
		if work > maxSTLPixelWork {
			return Result{}, fail("limit_exceeded", "STL raster work exceeds limit")
		}
		shade := 0.35 + 0.65*math.Max(0, normal.Dot(light))
		shader.Color = fauxgl.HexColor("#6796b5").MulScalar(shade).Alpha(1)
		drawn += dc.DrawTriangle(fauxgl.NewTriangleForPoints(t[0], t[1], t[2])).UpdatedPixels
	}
	if drawn == 0 {
		return Result{}, fail("decode_failed", "STL has no visible surface")
	}
	if err = ctx.Err(); err != nil {
		return Result{}, err
	}
	img := dc.Image()
	if size != spec.Size {
		img = imaging.Resize(img, spec.Size, spec.Size, imaging.Lanczos)
	}
	dst, err := os.Create(output)
	if err != nil {
		return Result{}, err
	}
	err = jpeg.Encode(dst, img, &jpeg.Options{Quality: 85})
	if err == nil {
		err = dst.Sync()
	}
	closeErr := dst.Close()
	if err == nil {
		err = closeErr
	}
	if err != nil {
		return Result{}, err
	}
	return Result{Width: spec.Size, Height: spec.Size, Engine: "fauxgl"}, nil
}
