package derive

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"math"
	"os"
	"sort"
	"strconv"
	"time"
)

const videoSampleLimit = 100

type videoCandidate struct {
	data          []byte
	width, height int
	time          float64
	histogram     [48]float64
	black         bool
}

// frame seeks before opening the input, avoiding a full decode of a long movie.
// Its output and diagnostics are bounded; no filenames from metadata are used.
func frame(ctx context.Context, executable, input string, index, size int, seconds float64) (videoCandidate, error) {
	var c videoCandidate
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	args := []string{"-nostdin", "-hide_banner", "-v", "error", "-max_alloc", "268435456", "-threads", "2", "-filter_threads", "1", "-protocol_whitelist", "file,pipe", "-format_whitelist", containers}
	if seconds >= 0 {
		args = append(args, "-ss", strconv.FormatFloat(seconds, 'f', 6, 64))
	}
	args = append(args, "-i", input, "-map", fmt.Sprintf("0:%d", index), "-map_metadata", "-1", "-vf", fmt.Sprintf("scale=w='min(%d,iw)':h='min(%d,ih)':force_original_aspect_ratio=decrease,setsar=1", size, size), "-frames:v", "1", "-an", "-c:v", "mjpeg", "-q:v", "3", "-threads", "1", "-f", "image2pipe", "pipe:1")
	out := &limitedBuffer{limit: 4 << 20}
	if err := run(ctx, executable, args, out); err != nil {
		return c, err
	}
	if out.Len() == out.limit {
		return c, fail("limit_exceeded", "thumbnail output exceeds limit")
	}
	cfg, format, err := image.DecodeConfig(bytes.NewReader(out.Bytes()))
	if err != nil {
		return c, err
	}
	if format != "jpeg" || !validDimensions(cfg.Width, cfg.Height) || cfg.Width > size || cfg.Height > size {
		return c, fail("limit_exceeded", "thumbnail dimensions exceed recipe")
	}
	img, _, err := image.Decode(bytes.NewReader(out.Bytes()))
	if err != nil {
		return c, err
	}
	c.data, c.width, c.height, c.time = out.Bytes(), cfg.Width, cfg.Height, seconds
	dark, total := 0, 0
	for y := img.Bounds().Min.Y; y < img.Bounds().Max.Y; y += 2 {
		for x := img.Bounds().Min.X; x < img.Bounds().Max.X; x += 2 {
			r, g, b, _ := img.At(x, y).RGBA()
			r >>= 8
			g >>= 8
			b >>= 8
			c.histogram[r/16]++
			c.histogram[16+g/16]++
			c.histogram[32+b/16]++
			if (299*r+587*g+114*b)/1000 < 24 {
				dark++
			}
			total++
		}
	}
	for i := range c.histogram {
		c.histogram[i] /= float64(total)
	}
	c.black = float64(dark)/float64(total) >= 0.98
	return c, nil
}

// sampleTimes visits the center, then recursively covers both halves. A time
// budget therefore never leaves us with only a prefix of the movie sampled.
func sampleTimes(duration float64) []float64 {
	count := videoSampleLimit
	if duration < 50 {
		count = max(1, int(math.Ceil(duration*2)))
	}
	type interval struct{ lo, hi int }
	queue := []interval{{0, count}}
	times := make([]float64, 0, count)
	for len(queue) > 0 {
		span := queue[0]
		queue = queue[1:]
		if span.lo >= span.hi {
			continue
		}
		mid := (span.lo + span.hi) / 2
		times = append(times, duration*(0.05+0.9*(float64(mid)+0.5)/float64(count)))
		queue = append(queue, interval{span.lo, mid}, interval{mid + 1, span.hi})
	}
	return times
}

func representative(candidates []videoCandidate, duration float64) int {
	var mean [48]float64
	count := 0
	for _, c := range candidates {
		if c.black {
			continue
		}
		count++
		for i, v := range c.histogram {
			mean[i] += v
		}
	}
	if count == 0 {
		return 0
	}
	for i := range mean {
		mean[i] /= float64(count)
	}
	best := -1
	distance := math.Inf(1)
	for i, c := range candidates {
		if c.black {
			continue
		}
		d := 0.0
		for j, v := range c.histogram {
			delta := v - mean[j]
			d += delta * delta
		}
		if d < distance || d == distance && (best < 0 || math.Abs(c.time-duration/2) < math.Abs(candidates[best].time-duration/2)) {
			best, distance = i, d
		}
	}
	return best
}

func coverRank(s probeStream) int {
	if s.Disposition.AttachedPic == 0 || s.CodecType != "video" {
		return -1
	}
	switch s.Tags.Filename {
	case "cover.jpg", "cover.png":
		return 0
	case "":
		return 1
	case "cover_land.jpg", "cover_land.png":
		return 2
	case "small_cover.jpg", "small_cover.png":
		return 3
	case "small_cover_land.jpg", "small_cover_land.png":
		return 4
	default:
		return -1
	}
}

func (r Renderer) videoThumbnail(ctx context.Context, executable, input, output string, spec Recipe, info probeResult) (Result, error) {
	ctx, cancel := context.WithTimeout(ctx, 90*time.Second)
	defer cancel()
	// FFmpeg exposes MP4 covr and JPEG/PNG Matroska cover attachments as
	// attached_pic streams. Rank named covers before unnamed attached pictures.
	covers := []probeStream{}
	var moving *probeStream
	for _, s := range info.Streams {
		if !validDimensions(s.Width, s.Height) {
			continue
		}
		if coverRank(s) >= 0 {
			covers = append(covers, s)
		}
		if moving == nil && s.movingVideo() {
			copy := s
			moving = &copy
		}
	}
	sort.SliceStable(covers, func(i, j int) bool { return coverRank(covers[i]) < coverRank(covers[j]) })
	for _, s := range covers[:min(len(covers), 4)] {
		c, err := frame(ctx, executable, input, s.Index, spec.Size, -1)
		if err == nil {
			return publishThumbnail(output, c)
		}
		if ctx.Err() != nil {
			return Result{}, ctx.Err()
		}
	}
	if moving == nil {
		return Result{}, fail("unsupported", "no supported moving video stream")
	}
	duration, err := strconv.ParseFloat(info.Format.Duration, 64)
	finite := err == nil && !math.IsNaN(duration) && !math.IsInf(duration, 0) && duration > 0
	if finite && duration > 6*60*60 {
		return Result{}, fail("limit_exceeded", "video duration exceeds six hours")
	}
	if finite {
		if seconds, ok := quickTimePoster(input, duration); ok {
			c, err := frame(ctx, executable, input, moving.Index, spec.Size, seconds)
			if err == nil {
				return publishThumbnail(output, c)
			}
			if ctx.Err() != nil {
				return Result{}, ctx.Err()
			}
		}
	}
	candidates := []videoCandidate{}
	if finite {
		sampleCtx, stop := context.WithTimeout(ctx, 60*time.Second)
		for _, seconds := range sampleTimes(duration) {
			if sampleCtx.Err() != nil {
				break
			}
			c, err := frame(sampleCtx, executable, input, moving.Index, 160, seconds)
			if err == nil {
				candidates = append(candidates, c)
			}
		}
		stop()
	}
	if ctx.Err() != nil {
		return Result{}, ctx.Err()
	}
	if len(candidates) > 0 {
		selected := candidates[representative(candidates, duration)]
		full, err := frame(ctx, executable, input, moving.Index, spec.Size, selected.time)
		if err == nil {
			return publishThumbnail(output, full)
		}
		if ctx.Err() != nil {
			return Result{}, ctx.Err()
		}
		// The already decoded sample is usable even if the larger retry fails.
		return publishThumbnail(output, selected)
	}
	c, err := frame(ctx, executable, input, moving.Index, spec.Size, 0)
	if err != nil {
		return Result{}, err
	}
	return publishThumbnail(output, c)
}

func publishThumbnail(output string, c videoCandidate) (Result, error) {
	f, err := os.Create(output)
	if err != nil {
		return Result{}, err
	}
	_, err = f.Write(c.data)
	if err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err == nil {
		err = closeErr
	}
	if err != nil {
		return Result{}, err
	}
	return Result{Width: c.width, Height: c.height, Engine: "ffmpeg"}, nil
}
