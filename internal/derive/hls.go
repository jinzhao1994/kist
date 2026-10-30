package derive

import (
	"context"
	"fmt"
	"io"
	"math"
	"os"
	"strconv"
	"time"
)

const (
	HLSRecipe                = "hls_1080p_v1"
	HLSSegmentSeconds        = 6
	MaxHLSSegmentBytes int64 = 32 << 20
)

// VideoDuration probes only a finite local input, using the same format and
// protocol restrictions as the ordinary derived renderer.
func (r Renderer) VideoDuration(ctx context.Context, input string) (float64, error) {
	info, err := r.probe(ctx, input)
	if err != nil {
		return 0, err
	}
	visual := false
	for _, stream := range info.Streams {
		if stream.movingVideo() {
			if !validDimensions(stream.Width, stream.Height) {
				return 0, fail("limit_exceeded", "source dimensions exceed the pixel limit")
			}
			visual = true
			break
		}
	}
	duration, e := strconv.ParseFloat(info.Format.Duration, 64)
	if !visual || e != nil || math.IsNaN(duration) || math.IsInf(duration, 0) || duration <= 0 {
		return 0, fail("unsupported", "source is not a finite video")
	}
	if duration > 6*60*60 {
		return 0, fail("limit_exceeded", "video duration exceeds six hours")
	}
	return duration, nil
}

// RenderHLSSegment writes one independently decodable segment. Presentation
// timestamps retain their position in the full VOD timeline, including after a
// random seek. The caller owns cancellation, temporary files and publication.
func (r Renderer) RenderHLSSegment(ctx context.Context, input, output string, duration float64, segment int) error {
	start := float64(segment * HLSSegmentSeconds)
	if segment < 0 || math.IsNaN(duration) || math.IsInf(duration, 0) || duration <= start || duration > 6*60*60 {
		return fail("unsupported", "invalid video segment")
	}
	path, err := tool(r.FFmpeg, "ffmpeg")
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, 90*time.Second)
	defer cancel()
	seconds := func(n float64) string { return strconv.FormatFloat(n, 'f', 6, 64) }
	args := []string{"-nostdin", "-hide_banner", "-v", "error", "-y", "-max_alloc", "268435456", "-threads", "2", "-filter_threads", "1", "-protocol_whitelist", "file,pipe", "-format_whitelist", containers,
		"-ss", seconds(start), "-i", input, "-t", seconds(math.Min(HLSSegmentSeconds, duration-start)), "-map", "0:v:0", "-map", "0:a:0?", "-map_metadata", "-1",
		"-vf", "scale=w='min(1920,iw)':h='min(1080,ih)':force_original_aspect_ratio=decrease:force_divisible_by=2,setsar=1", "-r", "30", "-c:v", "libx264", "-preset", "veryfast", "-crf", "23", "-maxrate", "8M", "-bufsize", "16M", "-pix_fmt", "yuv420p", "-g", "180", "-keyint_min", "180", "-sc_threshold", "0", "-bf", "0",
		"-c:a", "aac", "-b:a", "128k", "-ar", "48000", "-ac", "2", "-threads", "2", "-output_ts_offset", seconds(start), "-avoid_negative_ts", "disabled", "-muxdelay", "0", "-muxpreload", "0", "-mpegts_flags", "+resend_headers", "-f", "mpegts", output}
	done := make(chan struct{})
	watched := make(chan struct{})
	oversized := false
	go func() {
		defer close(watched)
		tick := time.NewTicker(50 * time.Millisecond)
		defer tick.Stop()
		for {
			select {
			case <-done:
				return
			case <-ctx.Done():
				return
			case <-tick.C:
				if st, e := os.Stat(output); e == nil && st.Size() > MaxHLSSegmentBytes {
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
		return fail("limit_exceeded", "video segment exceeds output limit")
	}
	if err != nil {
		return err
	}
	f, err := os.Open(output)
	if err != nil {
		return err
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return err
	}
	var head [1]byte
	_, err = io.ReadFull(f, head[:])
	if err != nil || st.Size() <= 0 || st.Size() > MaxHLSSegmentBytes || st.Size()%188 != 0 || head[0] != 0x47 {
		return fail("decode_failed", fmt.Sprintf("invalid MPEG-TS output (%d bytes)", st.Size()))
	}
	return nil
}
