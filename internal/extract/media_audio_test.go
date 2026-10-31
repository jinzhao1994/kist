package extract

import (
	"bytes"
	"testing"
)

func TestAudioContainerDetection(t *testing.T) {
	for _, tc := range []struct{ data, mime string }{
		{"fLaC\x00\x00\x00\x22", "audio/flac"},
		{"\x00\x00\x00\x18ftypM4A \x00\x00\x00\x00", "audio/mp4"},
		{"\x00\x00\x00\x18ftypM4B \x00\x00\x00\x00", "audio/mp4"},
		{"RIFF\x00\x00\x00\x00WAVEfmt ", "audio/wave"},
	} {
		got, err := DetectMedia(bytes.NewBufferString(tc.data))
		if err != nil || got != tc.mime {
			t.Fatalf("got=%s want=%s err=%v", got, tc.mime, err)
		}
	}
}
