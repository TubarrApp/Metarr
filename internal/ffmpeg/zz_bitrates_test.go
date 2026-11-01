package ffmpeg

import (
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"metarr/internal/abstractions"
	"metarr/internal/domain/consts"
	"metarr/internal/domain/keys"

	"github.com/TubarrApp/gocommon/sharedconsts"
	"github.com/TubarrApp/gocommon/sharedmodels"
)

// TestBitrateFor tests a video gets the bitrate for the largest resolution at or below its shorter side.
func TestBitrateFor(t *testing.T) {
	ladder := map[int]int64{720: 1_500_000, 1080: 3_000_000, 2160: 10_000_000}
	tests := []struct {
		bitrates    map[int]int64
		shorterSide int
		want        int64
	}{
		{ladder, 1080, 3_000_000},
		{ladder, 1440, 3_000_000},
		{ladder, 2160, 10_000_000},
		{ladder, 480, 1_500_000}, // Smaller than every entry.
		{ladder, 0, 1_500_000},   // Size unknown.
		{map[int]int64{0: 2_000_000}, 4320, 2_000_000},
		{nil, 1080, 0},
	}
	for _, tt := range tests {
		if got := bitrateFor(tt.bitrates, tt.shorterSide); got != tt.want {
			t.Errorf("bitrateFor(%v, %d) = %d, want %d", tt.bitrates, tt.shorterSide, got, tt.want)
		}
	}
}

// TestVideoBitrateCapped tests the bitrate is lowered to an MKV source's own, read from the whole file as its stream has none.
func TestVideoBitrateCapped(t *testing.T) {
	if !strings.Contains(availableEncoders(t.Context()), " libx264 ") {
		t.Skip("FFmpeg with libx264 isn't installed")
	}
	in := filepath.Join(t.TempDir(), "in.mkv")
	if out, err := exec.Command("ffmpeg", "-v", "error", "-f", "lavfi", "-i", "testsrc=size=160x90:rate=10", "-t", "2",
		"-c:v", "libx264", "-b:v", "100k", in).CombinedOutput(); err != nil {
		t.Fatalf("making the input: %v\n%s", err, out)
	}

	shorterSide, sourceBitrate := probeVideoSize(t.Context(), in)
	if shorterSide != 90 || sourceBitrate <= 0 {
		t.Fatalf("probeVideoSize = %d, %d; want 90 and the file's bitrate", shorterSide, sourceBitrate)
	}
	if got := videoBitrate(t.Context(), in, map[int]int64{0: 10_000_000}); got != sourceBitrate {
		t.Errorf("videoBitrate = %d, want the source's %d", got, sourceBitrate)
	}
	if got := videoBitrate(t.Context(), in, map[int]int64{0: 1_000}); got != 1_000 {
		t.Errorf("videoBitrate = %d, want 1000, which is under the source's", got)
	}
}

// TestSetVideoEncoderArgsBitrate tests the chosen bitrate is sent for the main video stream, before the other args.
func TestSetVideoEncoderArgsBitrate(t *testing.T) {
	t.Cleanup(func() { abstractions.Set(keys.TranscodeVideoArgsMap, map[string]sharedmodels.EncoderArgs{}) })
	abstractions.Set(keys.TranscodeVideoArgsMap, map[string]sharedmodels.EncoderArgs{
		sharedconsts.VCodecH264: {Bitrates: map[int]int64{720: 2_000_000, 1080: 4_000_000}, Args: []string{"-preset", "slow"}},
	})

	// No input file, so its size is unknown and the smallest resolution's bitrate is used.
	b := &ffCommandBuilder{builder: &strings.Builder{}, videoCodecSoftware: []string{consts.FFmpegCV0, "libx264"}}
	if err := b.setVideoEncoderArgs(t.Context(), "", sharedconsts.VCodecH264); err != nil {
		t.Fatal(err)
	}
	if want := []string{consts.FFmpegBitrate, strconv.Itoa(2_000_000), "-preset", "slow"}; !slices.Equal(b.encoderArgs, want) {
		t.Errorf("got %q, want %q", b.encoderArgs, want)
	}
}
