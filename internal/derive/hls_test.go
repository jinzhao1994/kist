package derive

import (
	"context"
	"encoding/json"
	"math"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"
)

func TestHLSSegmentsSeekTailAndAudio(t *testing.T) {
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("ffmpeg unavailable")
	}
	ffprobe, err := exec.LookPath("ffprobe")
	if err != nil {
		t.Skip("ffprobe unavailable")
	}
	for _, audio := range []bool{true, false} {
		t.Run(strconv.FormatBool(audio), func(t *testing.T) {
			input := filepath.Join(t.TempDir(), "synthetic.mp4")
			args := []string{"-v", "error", "-f", "lavfi", "-i", "testsrc2=size=320x180:rate=30"}
			if audio {
				args = append(args, "-f", "lavfi", "-i", "sine=frequency=440:sample_rate=48000")
			}
			args = append(args, "-t", "12.5", "-c:v", "mpeg4", "-q:v", "5", "-c:a", "aac", "-threads", "1", input)
			if out, err := exec.CommandContext(t.Context(), ffmpeg, args...).CombinedOutput(); err != nil {
				t.Fatalf("fixture: %v %s", err, out)
			}
			r := Renderer{FFmpeg: ffmpeg, FFprobe: ffprobe}
			duration, err := r.VideoDuration(t.Context(), input)
			if err != nil || math.Abs(duration-12.5) > .1 {
				t.Fatal(duration, err)
			}
			for _, index := range []int{2, 0, 1} {
				output := filepath.Join(t.TempDir(), "segment.ts")
				if err := r.RenderHLSSegment(t.Context(), input, output, duration, index); err != nil {
					t.Fatal(err)
				}
				out, err := exec.CommandContext(t.Context(), ffprobe, "-v", "error", "-show_entries", "stream=codec_type,codec_name,start_time,duration", "-of", "json", output).Output()
				if err != nil {
					t.Fatal(err)
				}
				var info struct {
					Streams []struct {
						Type     string `json:"codec_type"`
						Codec    string `json:"codec_name"`
						Start    string `json:"start_time"`
						Duration string `json:"duration"`
					}
				}
				if err = json.Unmarshal(out, &info); err != nil {
					t.Fatal(err)
				}
				hasVideo, hasAudio := false, false
				for _, s := range info.Streams {
					start, _ := strconv.ParseFloat(s.Start, 64)
					if math.Abs(start-float64(index*6)) > .05 {
						t.Fatalf("index %d timestamp: %+v", index, s)
					}
					if s.Type == "video" {
						hasVideo = s.Codec == "h264"
						length, _ := strconv.ParseFloat(s.Duration, 64)
						if math.Abs(length-math.Min(6, duration-float64(index*6))) > .05 {
							t.Fatalf("segment duration: %+v", s)
						}
					}
					if s.Type == "audio" {
						hasAudio = s.Codec == "aac"
					}
				}
				if !hasVideo || hasAudio != audio {
					t.Fatalf("unexpected codecs: %s", out)
				}
			}
			ctx, cancel := context.WithCancel(t.Context())
			cancel()
			if err := r.RenderHLSSegment(ctx, input, filepath.Join(t.TempDir(), "cancelled.ts"), duration, 0); err == nil {
				t.Fatal("cancel ignored")
			}
		})
	}
}
