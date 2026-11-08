package ffmpeg

import (
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/TubarrApp/gocommon/sharedconsts"
)

// TestGetCodecsWithoutStreamBitrate tests MKV and WebM codecs are read, though neither stores a bitrate per stream.
func TestGetCodecsWithoutStreamBitrate(t *testing.T) {
	encoders := availableEncoders(t.Context())
	dir := t.TempDir()

	tests := []struct {
		file, video, audio string
		args               []string
	}{
		{"in.mp4", "h264", "aac", []string{"-c:v", "libx264", "-c:a", "aac"}},
		{"in.mkv", "h264", "aac", []string{"-c:v", "libx264", "-c:a", "aac"}},
		{"in.webm", "vp9", "opus", []string{"-c:v", "libvpx-vp9", "-deadline", "realtime", "-c:a", "libopus"}},
	}
	for _, tt := range tests {
		t.Run(tt.file, func(t *testing.T) {
			if !strings.Contains(encoders, " "+tt.args[1]+" ") || !strings.Contains(encoders, " "+tt.args[len(tt.args)-1]+" ") {
				t.Skipf("FFmpeg can't encode %s with %v", tt.file, tt.args)
			}
			in := filepath.Join(dir, tt.file)
			args := append([]string{"-v", "error", "-f", "lavfi", "-i", "testsrc=size=320x180:rate=25", "-f", "lavfi", "-i", "sine",
				"-t", "2"}, tt.args...)
			if out, err := exec.Command(sharedconsts.ProgramFFmpeg, append(args, in)...).CombinedOutput(); err != nil {
				t.Fatalf("making %s: %v\n%s", tt.file, err, out)
			}

			video, audio, err := getCodecs(t.Context(), in)
			if err != nil {
				t.Fatal(err)
			}
			if video.Codec != tt.video || audio.Codec != tt.audio {
				t.Errorf("got video %q, audio %q; want %q, %q", video.Codec, audio.Codec, tt.video, tt.audio)
			}
			if video.Compression == sharedconsts.CompressionBlank {
				t.Errorf("got no video compression level, want one from the file's bitrate")
			}
		})
	}
}
