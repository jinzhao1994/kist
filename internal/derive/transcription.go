package derive

import (
	"context"
	"io"
	"math"
	"os"
	"strconv"
	"time"
)

const (
	TranscriptionSegmentSeconds       = 10 * 60
	MaxTranscriptionSeconds           = 6 * 60 * 60
	MaxTranscriptionAudioBytes  int64 = 20_000_000
	transcriptionContainers           = containers + ",mp3,wav,flac,aac"
)

// TranscriptionDuration selects only the first audio stream. Cover art, video
// frames and other streams are never sent to the transcription service.
func (r Renderer) TranscriptionDuration(ctx context.Context, input string) (float64, error) {
	info, err := r.probeFormats(ctx, input, transcriptionContainers)
	if err != nil {
		return 0, err
	}
	for _, stream := range info.Streams {
		if stream.CodecType != "audio" {
			continue
		}
		duration, err := strconv.ParseFloat(stream.Duration, 64)
		if err != nil || duration <= 0 {
			duration, err = strconv.ParseFloat(info.Format.Duration, 64)
		}
		if err != nil || math.IsNaN(duration) || math.IsInf(duration, 0) || duration <= 0 {
			return 0, fail("unsupported", "audio duration is unavailable")
		}
		if duration > MaxTranscriptionSeconds {
			return 0, fail("limit_exceeded", "audio duration exceeds six hours")
		}
		return duration, nil
	}
	return 0, fail("unsupported", "file contains no audio stream")
}

// RenderTranscriptionAudio writes one mono PCM segment below the file
// API's 25 MB limit. Input seeking remains sample-accurate while transcoding.
func (r Renderer) RenderTranscriptionAudio(ctx context.Context, input, output string, start, seconds float64) error {
	if math.IsNaN(start) || math.IsInf(start, 0) || start < 0 || math.IsNaN(seconds) || math.IsInf(seconds, 0) || seconds <= 0 || seconds > TranscriptionSegmentSeconds || start+seconds > MaxTranscriptionSeconds {
		return fail("unsupported", "invalid transcription segment")
	}
	path, err := tool(r.FFmpeg, "ffmpeg")
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, 90*time.Second)
	defer cancel()
	number := func(value float64) string { return strconv.FormatFloat(value, 'f', 6, 64) }
	args := []string{"-nostdin", "-hide_banner", "-v", "error", "-y", "-max_alloc", "268435456", "-threads", "2", "-filter_threads", "1", "-protocol_whitelist", "file,pipe", "-format_whitelist", transcriptionContainers,
		"-ss", number(start), "-i", input, "-t", number(seconds), "-map", "0:a:0", "-vn", "-sn", "-dn", "-map_metadata", "-1", "-ac", "1", "-ar", "16000", "-c:a", "pcm_s16le", "-threads", "1", "-f", "wav", output}
	if err := run(ctx, path, args, io.Discard); err != nil {
		return err
	}
	file, err := os.Open(output)
	if err != nil {
		return err
	}
	defer file.Close()
	stat, err := file.Stat()
	if err != nil {
		return err
	}
	var header [12]byte
	if _, err := io.ReadFull(file, header[:]); err != nil || string(header[:4]) != "RIFF" || string(header[8:]) != "WAVE" || stat.Size() < 44 || stat.Size() > MaxTranscriptionAudioBytes {
		return fail("decode_failed", "invalid or oversized transcription audio")
	}
	return nil
}
