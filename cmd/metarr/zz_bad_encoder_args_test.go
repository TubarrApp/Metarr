//go:build unix

package main

import (
	"os"
	"os/exec"
	"strings"
	"testing"

	"metarr/internal/domain/consts"
	"metarr/internal/domain/keys"

	"github.com/TubarrApp/gocommon/sharedconsts"
)

// TestBadEncoderArgsAtRunTime runs Metarr through its real flags: settings that can't work stop before FFmpeg runs and leave
// the video as it was, saying why, while anything that might work is left alone.
func TestBadEncoderArgsAtRunTime(t *testing.T) {
	encoders, err := exec.Command(sharedconsts.ProgramFFmpeg, "-hide_banner", "-encoders").Output()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(encoders), " "+consts.FFVCodecKeyH264+" ") {
		t.Skip("FFmpeg can't encode H.264")
	}

	const (
		videoCodecs = "--" + keys.TranscodeVideoCodecInput + "="
		audioCodecs = "--" + keys.TranscodeAudioCodecInput + "="
		videoArgs   = "--" + keys.TranscodeVideoArgs + "="
		extraArgs   = "--" + keys.ExtraFFmpegArgs + "="
		gpu         = "--" + keys.TranscodeGPU + "="
	)

	for _, c := range []struct {
		name     string
		flags    []string
		noAudio  bool
		encoder  string // Skipped without this encoder.
		wantStop bool
		want     string // In the output.
	}{
		// Refused at startup, whatever the video.
		{name: "copy line", flags: []string{videoArgs + "copy:-crf 20"}, wantStop: true, want: "does nothing, as copied video isn't encoded"},
		{name: "another encoder's option", flags: []string{videoArgs + "h264:-x265-params log-level=error"}, wantStop: true, want: "-x265-params does nothing on h264"},
		{name: "line for copied video", flags: []string{videoCodecs + "copy", videoArgs + "h264:-crf 20"}, wantStop: true, want: "h264 encoder args do nothing"},
		{name: "AV1 preset name", flags: []string{videoArgs + "av1:-preset medium"}, wantStop: true, want: "av1 -preset is a number"},
		{name: "VP9 profile name", flags: []string{videoArgs + "vp9:-profile:v main"}, wantStop: true, want: "vp9 -profile:v is a number"},
		{name: "audio filter on copied audio", flags: []string{extraArgs + "-af volume=0.5"}, wantStop: true, want: "-af in extra FFmpeg args"},
		{name: "audio option on copied audio", flags: []string{extraArgs + "-b:a 32k"}, wantStop: true, want: "-b:a in extra FFmpeg args"},

		// Only known for each video: audio left copied by a pair for another codec, and a GPU encoder the build doesn't have.
		{name: "audio filter on audio a pair leaves copied", flags: []string{audioCodecs + "dts:ac3", extraArgs + "-af volume=0.5"}, wantStop: true, want: "-af in extra FFmpeg args"},
		{name: "VP9 profile name, falling back to software", flags: []string{gpu + "cuda", videoCodecs + "h264:vp9", videoArgs + "vp9:-profile:v main"}, encoder: consts.FFVCodecKeyVP9, wantStop: true, want: "vp9 -profile:v is a number"},

		// Fine.
		{name: "working encoder args", flags: []string{videoCodecs + "h264:h264", videoArgs + "h264:-crf 30 -preset ultrafast"}},
		{name: "audio filter on encoded audio", flags: []string{audioCodecs + "ac3", extraArgs + "-af volume=0.5"}},
		{name: "audio filter with a pair on a file with no audio", flags: []string{audioCodecs + "dts:ac3", extraArgs + "-af volume=0.5"}, noAudio: true},
		{name: "line that might work, for a codec not reached", flags: []string{videoArgs + "vp8:-pix_fmt yuv420p10le"}},
		{name: "GPU encoder's preset name, for a codec not reached", flags: []string{gpu + "cuda", videoCodecs + "h264:mpeg2", videoArgs + "av1:-preset p5"}, encoder: consts.FFVCodecKeyMPEG2},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			if c.encoder != "" && !strings.Contains(string(encoders), " "+c.encoder+" ") {
				t.Skipf("FFmpeg can't encode with %s", c.encoder)
			}

			// Make the video, with audio unless the case has none.
			video, meta := makeReencodeVideo(t)
			if !c.noAudio {
				withAudio := video + ".audio.mkv"
				if out, err := exec.Command(sharedconsts.ProgramFFmpeg, "-v", "error", "-y", "-i", video, "-f", "lavfi", "-i", "sine=d=0.2:r=48000",
					"-map", "0:v", "-map", "1:a", "-c:v", "copy", "-c:a", "aac", "-shortest", withAudio).CombinedOutput(); err != nil {
					t.Fatalf("adding audio: %v\n%s", err, out)
				}
				if err := os.Rename(withAudio, video); err != nil {
					t.Fatal(err)
				}
			}
			before, err := os.ReadFile(video)
			if err != nil {
				t.Fatal(err)
			}

			// Run Metarr.
			home := t.TempDir()
			cmd := exec.CommandContext(t.Context(), os.Args[0], append([]string{"--" + keys.VideoFiles + "=" + video, "--" + keys.MetaFiles + "=" + meta}, c.flags...)...)
			cmd.Dir = home
			cmd.Env = []string{"PATH=" + os.Getenv("PATH"), "HOME=" + home, "TMPDIR=" + home, metarrAsMainEnv + "=1"}
			out, runErr := cmd.CombinedOutput()
			output := string(out)

			// How it ended, and why.
			if stopped := runErr != nil; stopped != c.wantStop {
				t.Errorf("stopped = %t, want %t: %v\n%s", stopped, c.wantStop, runErr, output)
			}
			if c.want != "" && !strings.Contains(output, c.want) {
				t.Errorf("output doesn't say %q:\n%s", c.want, output)
			}

			// A stopped video never reached FFmpeg, and is left as it was.
			if c.wantStop {
				if sweepCommand.MatchString(output) {
					t.Errorf("FFmpeg ran before stopping:\n%s", output)
				}
				after, err := os.ReadFile(video)
				if err != nil {
					t.Fatalf("the original is gone: %v", err)
				}
				if string(before) != string(after) {
					t.Error("the original changed")
				}
			}
		})
	}
}
