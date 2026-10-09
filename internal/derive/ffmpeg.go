package derive

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// Playlists and network protocols are deliberately absent. In particular an
// uploaded HLS manifest must not become an HTTP client on the archive's behalf.
const containers = "mov,matroska,webm,avi,mpeg,mpegts,ogg,flv,asf"

type probeResult struct {
	Streams []struct {
		CodecType string `json:"codec_type"`
		Width     int    `json:"width"`
		Height    int    `json:"height"`
	} `json:"streams"`
	Format struct {
		Duration string `json:"duration"`
	} `json:"format"`
}

func tool(explicit, name string) (string, error) {
	if explicit != "" {
		name = explicit
	}
	path, err := exec.LookPath(name)
	if err != nil {
		return "", &Failure{"dependency_missing", fmt.Errorf("%s is unavailable: %w", name, err)}
	}
	return path, nil
}

// limitedBuffer keeps stderr and ffprobe output bounded even for hostile input.
type limitedBuffer struct {
	bytes.Buffer
	limit int
}

func (b *limitedBuffer) Write(p []byte) (int, error) {
	n := len(p)
	if left := b.limit - b.Len(); left > 0 {
		if len(p) > left {
			p = p[:left]
		}
		_, _ = b.Buffer.Write(p)
	}
	return n, nil
}

func run(ctx context.Context, path string, args []string, stdout io.Writer) error {
	log := &limitedBuffer{limit: 16 << 10}
	cmd := exec.CommandContext(ctx, path, args...)
	cmd.Stdout, cmd.Stderr = stdout, log
	cmd.WaitDelay = time.Second
	err := cmd.Run()
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if err != nil {
		code := "decode_failed"
		message := log.String()
		if strings.Contains(message, "Unknown encoder") || strings.Contains(message, "Unsupported codec") || strings.Contains(message, "Decoder") && strings.Contains(message, "not found") {
			code = "dependency_missing"
		}
		return &Failure{code, fmt.Errorf("media tool: %w: %s", err, message)}
	}
	return nil
}

func (r Renderer) probe(ctx context.Context, input string) (probeResult, error) {
	var info probeResult
	path, err := tool(r.FFprobe, "ffprobe")
	if err != nil {
		return info, err
	}
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	out := &limitedBuffer{limit: 1 << 20}
	err = run(ctx, path, []string{"-v", "error", "-max_alloc", "268435456", "-protocol_whitelist", "file,pipe", "-format_whitelist", containers, "-show_entries", "stream=codec_type,width,height:format=duration", "-of", "json", input}, out)
	if err != nil {
		return info, err
	}
	err = json.Unmarshal(out.Bytes(), &info)
	return info, err
}

func (r Renderer) external(ctx context.Context, input, output string, spec Recipe) (Result, error) {
	var result Result
	path, err := tool(r.FFmpeg, "ffmpeg")
	if err != nil {
		return result, err
	}
	info, err := r.probe(ctx, input)
	if err != nil {
		return result, err
	}
	width, height := 0, 0
	for _, stream := range info.Streams {
		if stream.CodecType == "video" {
			width, height = stream.Width, stream.Height
			break
		}
	}
	if width == 0 || height == 0 {
		return result, fail("unsupported", "no visual stream in source")
	}
	if !validDimensions(width, height) {
		return result, fail("limit_exceeded", "source dimensions exceed the pixel limit")
	}
	timeout := 90 * time.Second
	if spec.Name == Video {
		duration, e := strconv.ParseFloat(info.Format.Duration, 64)
		if e != nil || duration <= 0 {
			return result, fail("unsupported", "source is not a finite video")
		}
		if duration > 6*60*60 {
			return result, fail("limit_exceeded", "video duration exceeds six hours")
		}
		timeout = 45 * time.Minute
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	args := []string{"-nostdin", "-hide_banner", "-v", "error", "-y", "-max_alloc", "268435456", "-threads", "2", "-filter_threads", "1", "-protocol_whitelist", "file,pipe", "-format_whitelist", containers, "-i", input, "-map", "0:v:0", "-map_metadata", "-1"}
	if spec.Name == Video {
		args = append(args, "-map", "0:a:0?", "-vf", "scale=w='min(1920,iw)':h='min(1080,ih)':force_original_aspect_ratio=decrease:force_divisible_by=2,setsar=1", "-c:v", "libx264", "-preset", "veryfast", "-crf", "23", "-pix_fmt", "yuv420p", "-c:a", "aac", "-b:a", "128k", "-threads", "2", "-movflags", "+faststart", "-f", "mp4", output)
	} else {
		args = append(args, "-vf", fmt.Sprintf("scale=w='min(%d,iw)':h='min(%d,ih)':force_original_aspect_ratio=decrease,setsar=1", spec.Size, spec.Size), "-frames:v", "1", "-an", "-c:v", "mjpeg", "-q:v", "3", "-threads", "1", "-f", "image2", output)
	}
	// Bound disk use while the encoder is alive, not only after it has finished.
	done := make(chan struct{})
	watched := make(chan struct{})
	oversized := false
	go func() {
		defer close(watched)
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-done:
				return
			case <-ctx.Done():
				return
			case <-ticker.C:
				if stat, e := os.Stat(output); e == nil && stat.Size() > MaxOutputBytes {
					oversized = true
					cancel()
					return
				}
			}
		}
	}()
	err = run(ctx, path, args, io.Discard)
	close(done)
	<-watched
	if oversized {
		return result, fail("limit_exceeded", "derived output exceeds size limit")
	}
	if err != nil {
		return result, err
	}
	f, e := os.OpenFile(output, os.O_RDWR, 0)
	if e != nil {
		return result, e
	}
	e = f.Sync()
	closeErr := f.Close()
	if e == nil {
		e = closeErr
	}
	if e != nil {
		return result, e
	}
	if spec.Name != Video {
		f, e := os.Open(output)
		if e != nil {
			return result, e
		}
		cfg, _, e := image.DecodeConfig(f)
		f.Close()
		if e != nil {
			return result, e
		}
		result.Width, result.Height = cfg.Width, cfg.Height
	} else {
		generated, e := r.probe(ctx, output)
		if e != nil {
			return result, e
		}
		if len(generated.Streams) == 0 {
			return result, errors.New("encoder produced no streams")
		}
		result.Width, result.Height = generated.Streams[0].Width, generated.Streams[0].Height
	}
	result.Engine = "ffmpeg"
	return result, nil
}
