package ffmpeg

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"metarr/internal/abstractions"
	"metarr/internal/domain/consts"
	"metarr/internal/domain/keys"

	"github.com/TubarrApp/gocommon/sharedconsts"
	"github.com/TubarrApp/gocommon/sharedmodels"
)

// TestFFmpegPass tests each pass gets FFmpeg's pass number: the first writes stats, middle ones refine them, the last makes the output.
func TestFFmpegPass(t *testing.T) {
	for passes, want := range map[int][]int{2: {1, 2}, 3: {1, 3, 2}} {
		var got []int
		for i := 1; i <= passes; i++ {
			got = append(got, ffmpegPass(i, passes))
		}
		if !slices.Equal(got, want) {
			t.Errorf("%d passes: got FFmpeg passes %v, want %v", passes, got, want)
		}
	}
}

// TestSetVideoEncoderArgsPasses tests software encoders keep their passes, while GPU encoders and copies make one.
func TestSetVideoEncoderArgsPasses(t *testing.T) {
	t.Cleanup(func() { abstractions.Set(keys.TranscodeVideoArgsMap, map[string]sharedmodels.EncoderArgs{}) })
	abstractions.Set(keys.TranscodeVideoArgsMap, map[string]sharedmodels.EncoderArgs{
		sharedconsts.VCodecH264: {Passes: 3, Args: []string{"-b:v", "3M"}},
	})

	tests := []struct {
		name      string
		accelType string
		gpuCodec  []string
		softCodec []string
		want      int
	}{
		{"software", "", nil, []string{consts.FFmpegCV0, "libx264"}, 3},
		{"gpu", sharedconsts.AccelTypeVAAPI, []string{consts.FFmpegCV0, "h264_vaapi"}, nil, 1},
		{"copy", "", nil, []string{consts.FFmpegCV0, consts.FFVCodecKeyCopy}, 1},
	}
	for _, tt := range tests {
		b := &ffCommandBuilder{builder: &strings.Builder{}, videoCodecGPU: tt.gpuCodec, videoCodecSoftware: tt.softCodec}
		if err := b.setVideoEncoderArgs(t.Context(), tt.accelType, sharedconsts.VCodecH264); err != nil {
			t.Fatal(err)
		}
		if b.passes != tt.want {
			t.Errorf("%s: got %d passes, want %d", tt.name, b.passes, tt.want)
		}
	}
}

// TestRunPasses tests one to three passes write the output and leave no pass stats behind.
func TestRunPasses(t *testing.T) {
	if !strings.Contains(availableEncoders(t.Context()), " libx264 ") {
		t.Skip("FFmpeg with libx264 isn't installed")
	}
	dir := t.TempDir()
	t.Setenv("TMPDIR", dir) // Pass stats folders go here, so leftovers can be found.
	in := filepath.Join(dir, "in.mp4")
	if out, err := exec.Command("ffmpeg", "-v", "error", "-f", "lavfi", "-i", "testsrc=size=160x90:rate=10", "-t", "1",
		"-c:v", "libx264", in).CombinedOutput(); err != nil {
		t.Fatalf("making the input: %v\n%s", err, out)
	}

	for _, passes := range []int{1, 2, 3} {
		out := filepath.Join(dir, fmt.Sprintf("out%d.mkv", passes))
		args := []string{"-y", "-i", in, consts.FFmpegCV0, "libx264", "-b:v", "100k", out}
		if stderr, err := runPasses(t.Context(), args, passes, 1_000_000, in); err != nil {
			t.Fatalf("%d passes: %v\n%s", passes, err, stderr)
		}
		if info, err := os.Stat(out); err != nil || info.Size() == 0 {
			t.Errorf("%d passes: no output written (%v)", passes, err)
		}
	}
	if leftovers, _ := filepath.Glob(filepath.Join(dir, "metarr-passes-*")); len(leftovers) > 0 {
		t.Errorf("pass stats left behind: %v", leftovers)
	}
}
