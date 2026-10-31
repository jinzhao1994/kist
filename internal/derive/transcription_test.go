package derive

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"sync/atomic"
	"testing"
)

func TestTranscriptionExtractsBoundedAudioAndRejectsSilentVideo(t *testing.T) {
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("ffmpeg unavailable")
	}
	ffprobe, err := exec.LookPath("ffprobe")
	if err != nil {
		t.Skip("ffprobe unavailable")
	}
	dir := t.TempDir()
	input := filepath.Join(dir, "video.mp4")
	silent := filepath.Join(dir, "silent.mp4")
	for _, file := range []string{input, silent} {
		args := []string{"-nostdin", "-v", "error", "-f", "lavfi", "-i", "color=c=blue:s=32x32:r=5"}
		if file == input {
			args = append(args, "-f", "lavfi", "-i", "sine=frequency=440:sample_rate=48000")
		}
		args = append(args, "-t", "2", "-c:v", "libx264", "-threads", "1", "-pix_fmt", "yuv420p", "-c:a", "aac", file)
		if out, err := exec.CommandContext(t.Context(), ffmpeg, args...).CombinedOutput(); err != nil {
			t.Fatalf("generate sample: %v %s", err, out)
		}
	}
	renderer := Renderer{FFmpeg: ffmpeg, FFprobe: ffprobe}
	duration, err := renderer.TranscriptionDuration(t.Context(), input)
	if err != nil || duration < 1.9 || duration > 2.1 {
		t.Fatal(duration, err)
	}
	output := filepath.Join(dir, "audio.wav")
	if err := renderer.RenderTranscriptionAudio(t.Context(), input, output, 0.5, 0.75); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(output)
	if err != nil || len(data) > 25000 || len(data) < 23000 || string(data[:4]) != "RIFF" {
		t.Fatal("wrong bounded PCM output", len(data), err)
	}
	seconds, err := renderer.TranscriptionDuration(t.Context(), output)
	if err != nil || seconds < 0.74 || seconds > 0.76 {
		t.Fatal(seconds, err)
	}
	if _, err := renderer.TranscriptionDuration(t.Context(), silent); Code(err) != "unsupported" {
		t.Fatal("video without audio accepted", err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if err := renderer.RenderTranscriptionAudio(ctx, input, output, 0, 1); !errors.Is(err, context.Canceled) {
		t.Fatal("cancelled extraction ran", err)
	}
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls.Add(1) }))
	defer server.Close()
	playlist := filepath.Join(dir, "remote.m3u8")
	if err := os.WriteFile(playlist, []byte("#EXTM3U\n#EXT-X-TARGETDURATION:1\n#EXTINF:1,\n"+server.URL+"/audio.ts\n#EXT-X-ENDLIST\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := renderer.TranscriptionDuration(t.Context(), playlist); err == nil || calls.Load() != 0 {
		t.Fatal("playlist caused a remote media read", err)
	}
}
