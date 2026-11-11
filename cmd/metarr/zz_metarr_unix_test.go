//go:build unix

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"math"
	"math/rand/v2"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"regexp"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"metarr/internal/domain/consts"
	"metarr/internal/domain/keys"
	"metarr/internal/ffprobe"
	"metarr/internal/parsing"

	"github.com/TubarrApp/gocommon/sharedconsts"
	"github.com/TubarrApp/gocommon/sharedmodels"
	"github.com/TubarrApp/gocommon/sharedparsing"
	"github.com/TubarrApp/gocommon/sharedvalidation"
)

// ******************************** Bad encoder args **********************************************************************************

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
		audioArgs   = "--" + keys.TranscodeAudioArgs + "="
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
		{name: "audio copy line", flags: []string{audioArgs + "copy:-b:a 128k"}, wantStop: true, want: "does nothing, as copied audio isn't encoded"},
		{name: "line for copied audio", flags: []string{audioCodecs + "copy", audioArgs + "aac:-b:a 128k"}, wantStop: true, want: "aac encoder args do nothing"},
		{name: "audio filter in extra args", flags: []string{extraArgs + "-af volume=0.5"}, wantStop: true, want: "-af in extra FFmpeg args"},
		{name: "audio option in extra args", flags: []string{extraArgs + "-b:a 32k"}, wantStop: true, want: "-b:a in extra FFmpeg args"},
		{name: "audio filter in extra args with audio encoded", flags: []string{audioCodecs + "ac3", extraArgs + "-af volume=0.5"}, wantStop: true, want: "-af in extra FFmpeg args"},

		// Only known for each video: a GPU encoder the build doesn't have.
		{name: "VP9 profile name, falling back to software", flags: []string{gpu + "cuda", videoCodecs + "h264:vp9", videoArgs + "vp9:-profile:v main"}, encoder: consts.FFVCodecKeyVP9, wantStop: true, want: "vp9 -profile:v is a number"},

		// Fine.
		{name: "working encoder args", flags: []string{videoCodecs + "h264:h264", videoArgs + "h264:-crf 30 -preset ultrafast"}},
		{name: "audio filter on encoded audio", flags: []string{audioCodecs + "ac3", audioArgs + "ac3:-af volume=0.5"}},
		{name: "audio args for a pair, on a file with no audio", flags: []string{audioCodecs + "dts:ac3", audioArgs + "ac3:-af volume=0.5"}, noAudio: true},
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
				addReencodeAudio(t, video)
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

// ******************************** Re-encoding ***************************************************************************************

// reencodeRun is what one Metarr run on a video did.
type reencodeRun struct {
	command string // The FFmpeg command run, "" if the video needed no processing.
	output  string
}

// TestReencodeWithFFmpeg runs Metarr twice on an 8-bit H.264 video through its real flags, as a second run shows whether a
// video would be encoded again every time it's processed.
func TestReencodeWithFFmpeg(t *testing.T) {
	encoders, err := exec.Command(sharedconsts.ProgramFFmpeg, "-hide_banner", "-encoders").Output()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(encoders), " "+consts.FFVCodecKeyH264+" ") {
		t.Skip("FFmpeg can't encode H.264")
	}

	const (
		tenBitArgs = "h264:-pix_fmt yuv420p10le"
		filterNote = "so its -vf isn't applied"
		encodeH264 = consts.FFmpegCV0 + " " + consts.FFVCodecKeyH264
	)

	for _, c := range []struct {
		name        string
		codecs      string
		args        string
		wantEncoded [2]bool
		wantPixFmt  string   // What the video should be in after both runs.
		wantInFirst []string // In the first run's command.
		wantWarning [2]bool
	}{
		{name: "nothing named, pixel format differs", args: tenBitArgs, wantEncoded: [2]bool{true, false}, wantPixFmt: "yuv420p10le"},
		{name: "plain own codec, pixel format differs", codecs: "h264", args: tenBitArgs, wantEncoded: [2]bool{true, false}, wantPixFmt: "yuv420p10le"},
		{name: "pair for another source, pixel format differs", codecs: "hevc:mpeg2", args: tenBitArgs, wantEncoded: [2]bool{true, false}, wantPixFmt: "yuv420p10le"},
		{name: "pixel format matches", args: "h264:-pix_fmt yuv420p", wantPixFmt: "yuv420p"},
		{name: "already the target, other args only", codecs: "hevc:h264", args: "h264:-crf 30 -preset ultrafast", wantPixFmt: "yuv420p"},
		{name: "pair to copy, with another source encoded to it", codecs: "h264:copy,hevc:h264", args: tenBitArgs, wantPixFmt: "yuv420p"},
		{name: "plain copy, with another source encoded to it", codecs: "copy,hevc:h264", args: tenBitArgs, wantPixFmt: "yuv420p"},
		{name: "pair to own codec", codecs: "h264:h264", wantEncoded: [2]bool{true, true}, wantPixFmt: "yuv420p"},
		{
			name: "pair to own codec uses its args", codecs: "h264:h264", args: "h264:-crf 30 -preset ultrafast", wantEncoded: [2]bool{true, true}, wantPixFmt: "yuv420p",
			wantInFirst: []string{"-crf 30", "-preset ultrafast"},
		},
		{name: "filter alone", args: "h264:-vf hflip", wantPixFmt: "yuv420p", wantWarning: [2]bool{true, true}},
		{name: "filter on a pair to copy, with another source encoded to it", codecs: "h264:copy,hevc:h264", args: "h264:-vf hflip", wantPixFmt: "yuv420p"},
		{
			name: "filter runs with a differing pixel format", args: tenBitArgs + " -vf hflip", wantEncoded: [2]bool{true, false}, wantPixFmt: "yuv420p10le",
			wantInFirst: []string{"hflip"}, wantWarning: [2]bool{false, true},
		},
		{name: "filter runs with a pair to own codec", codecs: "h264:h264", args: "h264:-vf hflip", wantEncoded: [2]bool{true, true}, wantPixFmt: "yuv420p", wantInFirst: []string{"hflip"}},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			video, meta := makeReencodeVideo(t)

			// Build the flags.
			flags := []string{"--" + keys.VideoFiles + "=" + video, "--" + keys.MetaFiles + "=" + meta}
			if c.codecs != "" {
				flags = append(flags, "--"+keys.TranscodeVideoCodecInput+"="+c.codecs)
			}
			if c.args != "" {
				flags = append(flags, "--"+keys.TranscodeVideoArgs+"="+c.args)
			}

			for i := range 2 {
				run := runReencodeMetarr(t, flags)

				// Encoded or not.
				if got := strings.Contains(run.command, encodeH264); got != c.wantEncoded[i] {
					t.Errorf("run %d: encoded = %t, want %t\ncommand: %s", i+1, got, c.wantEncoded[i], run.command)
				}

				// The warning for a filter that won't run.
				if got := strings.Contains(run.output, filterNote); got != c.wantWarning[i] {
					t.Errorf("run %d: warned = %t, want %t", i+1, got, c.wantWarning[i])
				}

				// The args of the run that encodes.
				if i == 0 {
					for _, want := range c.wantInFirst {
						if !strings.Contains(run.command, want) {
							t.Errorf("want %q in: %s", want, run.command)
						}
					}
				}

				// The video that's left.
				if got := reencodePixFmt(t, video); got != c.wantPixFmt {
					t.Errorf("run %d: pixel format = %s, want %s", i+1, got, c.wantPixFmt)
				}
			}
		})
	}

	// A video encoded to another codec stops being encoded once it's there.
	t.Run("another codec is encoded once", func(t *testing.T) {
		t.Parallel()
		if !strings.Contains(string(encoders), " "+consts.FFVCodecKeyMPEG2+" ") {
			t.Skip("FFmpeg can't encode MPEG-2")
		}
		video, meta := makeReencodeVideo(t)
		flags := []string{
			"--" + keys.VideoFiles + "=" + video, "--" + keys.MetaFiles + "=" + meta,
			"--" + keys.TranscodeVideoCodecInput + "=h264:mpeg2", "--" + keys.TranscodeVideoArgs + "=mpeg2:-pix_fmt yuv422p",
		}
		encodeMPEG2 := consts.FFmpegCV0 + " " + consts.FFVCodecKeyMPEG2

		if first := runReencodeMetarr(t, flags); !strings.Contains(first.command, encodeMPEG2) {
			t.Errorf("first run: want %q in: %s", encodeMPEG2, first.command)
		}
		if second := runReencodeMetarr(t, flags); strings.Contains(second.command, encodeMPEG2) {
			t.Errorf("second run encoded again: %s", second.command)
		}
		if got := reencodePixFmt(t, video); got != "yuv422p" {
			t.Errorf("pixel format = %s, want yuv422p", got)
		}
	})
}

// TestReencodeAudioWithFFmpeg runs Metarr twice on a video with stereo 48 kHz AAC through its real flags, as a second run
// shows whether its audio would be encoded again every time it's processed.
func TestReencodeAudioWithFFmpeg(t *testing.T) {
	encoders, err := exec.Command(sharedconsts.ProgramFFmpeg, "-hide_banner", "-encoders").Output()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(encoders), " "+consts.FFVCodecKeyH264+" ") {
		t.Skip("FFmpeg can't encode H.264")
	}

	const (
		filterNote = "so its -af isn't applied"
		encodeAAC  = consts.FFmpegCA + " " + consts.FFACodecKeyAAC
	)

	for _, c := range []struct {
		name        string
		codecs      string
		args        string
		wantEncoded [2]bool
		wantFormat  string // What the audio should be in after both runs.
		wantWarning [2]bool
	}{
		{name: "nothing named, channels differ", args: "aac:-ac 1", wantEncoded: [2]bool{true, false}, wantFormat: "1ch 48000Hz"},
		{name: "plain own codec, sample rate differs", codecs: "aac", args: "aac:-ar 44100", wantEncoded: [2]bool{true, false}, wantFormat: "2ch 44100Hz"},
		{name: "channels and sample rate match", args: "aac:-ac 2 -ar 48000", wantFormat: "2ch 48000Hz"},
		{name: "other args only", args: "aac:-b:a 64k", wantFormat: "2ch 48000Hz"},
		{name: "pair to copy, with another source encoded to it", codecs: "aac:copy,mp3:aac", args: "aac:-ac 1", wantFormat: "2ch 48000Hz"},
		{name: "pair to own codec", codecs: "aac:aac", wantEncoded: [2]bool{true, true}, wantFormat: "2ch 48000Hz"},
		{name: "filter alone", args: "aac:-af volume=0.5", wantFormat: "2ch 48000Hz", wantWarning: [2]bool{true, true}},
		{
			name: "filter runs with differing channels", args: "aac:-ac 1 -af volume=0.5", wantEncoded: [2]bool{true, false}, wantFormat: "1ch 48000Hz",
			wantWarning: [2]bool{false, true},
		},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			video, meta := makeReencodeVideo(t)
			addReencodeAudio(t, video)

			// Build the flags.
			flags := []string{"--" + keys.VideoFiles + "=" + video, "--" + keys.MetaFiles + "=" + meta}
			if c.codecs != "" {
				flags = append(flags, "--"+keys.TranscodeAudioCodecInput+"="+c.codecs)
			}
			if c.args != "" {
				flags = append(flags, "--"+keys.TranscodeAudioArgs+"="+c.args)
			}

			for i := range 2 {
				run := runReencodeMetarr(t, flags)

				// Encoded or not.
				if got := strings.Contains(run.command, encodeAAC); got != c.wantEncoded[i] {
					t.Errorf("run %d: encoded = %t, want %t\ncommand: %s", i+1, got, c.wantEncoded[i], run.command)
				}

				// The warning for a filter that won't run.
				if got := strings.Contains(run.output, filterNote); got != c.wantWarning[i] {
					t.Errorf("run %d: warned = %t, want %t", i+1, got, c.wantWarning[i])
				}

				// The audio that's left.
				channels, sampleRate := ffprobe.ProbeAudioFormat(t.Context(), video)
				if got := fmt.Sprintf("%dch %dHz", channels, sampleRate); got != c.wantFormat {
					t.Errorf("run %d: audio = %s, want %s", i+1, got, c.wantFormat)
				}
			}
		})
	}
}

// makeReencodeVideo writes an 8-bit H.264 video and its metadata.
func makeReencodeVideo(t *testing.T) (video, meta string) {
	t.Helper()
	dir := t.TempDir()
	video = filepath.Join(dir, "video.mkv")
	meta = filepath.Join(dir, "video.json")
	if out, err := exec.Command(sharedconsts.ProgramFFmpeg, "-v", "error", "-y", "-f", "lavfi", "-i", "testsrc2=s=64x64:d=0.2:r=10",
		"-c:v", consts.FFVCodecKeyH264, "-preset", "ultrafast", "-pix_fmt", "yuv420p", video).CombinedOutput(); err != nil {
		t.Fatalf("making video: %v\n%s", err, out)
	}
	info, err := json.Marshal(map[string]any{
		"id":          "reencode",
		"title":       "Encoded Again",
		"fulltitle":   "Encoded Again",
		"description": "About encoding again",
		"uploader":    "An Uploader",
		"upload_date": "20240102",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(meta, info, 0o644); err != nil {
		t.Fatal(err)
	}
	return video, meta
}

// addReencodeAudio gives a video from makeReencodeVideo stereo 48 kHz AAC audio.
func addReencodeAudio(t *testing.T, video string) {
	t.Helper()
	withAudio := video + ".audio.mkv"
	if out, err := exec.Command(sharedconsts.ProgramFFmpeg, "-v", "error", "-y", "-i", video, "-f", "lavfi", "-i", "sine=d=0.2:r=48000",
		"-map", "0:v", "-map", "1:a", "-c:v", "copy", "-c:a", "aac", "-ac", "2", "-shortest", withAudio).CombinedOutput(); err != nil {
		t.Fatalf("adding audio: %v\n%s", err, out)
	}
	if err := os.Rename(withAudio, video); err != nil {
		t.Fatal(err)
	}
}

// runReencodeMetarr runs Metarr, failing the test if it fails.
func runReencodeMetarr(t *testing.T, flags []string) reencodeRun {
	t.Helper()
	home := t.TempDir()
	cmd := exec.CommandContext(t.Context(), os.Args[0], flags...)
	cmd.Dir = home
	cmd.Env = []string{"PATH=" + os.Getenv("PATH"), "HOME=" + home, "TMPDIR=" + home, metarrAsMainEnv + "=1"}
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("running Metarr with %q: %v\n%s", flags, err, out)
	}

	// Read the command it ran.
	run := reencodeRun{output: string(out)}
	if matches := sweepCommand.FindAllStringSubmatch(run.output, -1); len(matches) > 0 {
		run.command = matches[len(matches)-1][1]
	}
	return run
}

// reencodePixFmt returns a video's pixel format.
func reencodePixFmt(t *testing.T, path string) string {
	t.Helper()
	out, err := exec.Command(sharedconsts.ProgramFFprobe, "-v", "error", "-select_streams", "V:0", "-show_entries", "stream=pix_fmt", "-of", "default=nw=1:nk=1", path).Output()
	if err != nil {
		t.Fatalf("probing %s: %v", path, err)
	}
	return strings.TrimSpace(string(out))
}

// ******************************** Transcode sweep ***********************************************************************************

// Settings for the sweep, which is skipped unless told how many cases to run, as it takes minutes and needs the real FFmpeg.
const (
	sweepRunsEnv    = "METARR_SWEEP_RUNS"    // How many random cases to run.
	sweepSeedEnv    = "METARR_SWEEP_SEED"    // The seed the cases come from, to repeat a sweep (default 1).
	sweepWorkersEnv = "METARR_SWEEP_WORKERS" // How many cases to run at once (default one per CPU).
	sweepReportEnv  = "METARR_SWEEP_REPORT"  // File to write the findings to, as well as logging them.
	sweepClipEnv    = "METARR_SWEEP_CLIP"    // Seconds each source lasts, as some problems only build up over time (default 2).
)

// sweepClipSeconds is how long each source lasts.
var sweepClipSeconds = "2"

// How a case ended, when it didn't find a problem.
const (
	sweepFine         = "fine"
	sweepRefusedFlags = "refused its flags"
	sweepRefusedRun   = "refused the video"
	sweepUnmakeable   = "source can't be made"
	sweepProblem      = "problem"
)

// Limits of a case.
const (
	sweepMaxDrift     = 0.3 // Seconds an output's streams may differ from the source's by.
	sweepMaxShrink    = 40  // Runs spent making one problem's case smaller.
	sweepArgSeparator = "|"
	sweepExamples     = 12
)

// Levels a black picture decodes to: Y is at most this, and U and V within this of the middle.
const (
	sweepBlackMaxY    = 24.0
	sweepBlackMidUV   = 128.0
	sweepBlackUVRange = 10.0
	sweepSilenceDB    = -60.0 // The test tone is well above this.
)

// sweepSource is a black test video to make. A blank Video or Audio encoder leaves that stream out.
type sweepSource struct {
	Ext         string
	Video       string
	Audio       string
	PixFmt      string
	Size        string
	Rate        string
	SampleRate  string
	Layout      string
	Subtitles   string // Subtitle encoder, blank for none.
	Thumbnail   bool
	SecondAudio bool
}

// sweepCase is one run of Metarr: a source video and the flags it's given.
type sweepCase struct {
	sweepSource
	OutputExt      string
	VideoCodecs    string
	AudioCodecs    string
	ArgsCodec      string // The codec VideoArgs are for.
	VideoArgs      string // Joined by sweepArgSeparator, so each can be dropped alone when shrinking.
	ExtraArgs      string
	GPU            string
	ThumbnailFile  string // Extension of the image given with --thumbnail.
	StripThumbnail bool
	PurgeOriginal  bool
	NoOverwrite    bool
}

// sweepPlainCase is what cases vary from, and shrink back to.
var sweepPlainCase = sweepCase{sweepSource: sweepSource{
	Ext:        sharedconsts.ExtMP4,
	Video:      consts.FFVCodecKeyH264,
	Audio:      consts.FFACodecKeyAAC,
	PixFmt:     "yuv420p",
	Size:       "128x72",
	Rate:       "25",
	SampleRate: "48000",
	Layout:     "stereo",
}}

// Source encoders Metarr's tables don't name, by container, for the paths unknown codecs take.
var (
	sweepOddVideo = map[string][]string{
		sharedconsts.ExtASF:  {"wmv1", "mpeg4"},
		sharedconsts.ExtAVI:  {"mpeg4", "msmpeg4", "mjpeg"},
		sharedconsts.ExtFLV:  {"flv"},
		sharedconsts.ExtMKV:  {"mpeg4", "ffv1", "mjpeg", "prores_ks"},
		sharedconsts.ExtMOV:  {"mpeg4", "prores_ks", "mjpeg"},
		sharedconsts.ExtMP4:  {"mpeg4"},
		sharedconsts.ExtMPEG: {"mpeg1video"},
		sharedconsts.ExtMPG:  {"mpeg1video"},
		sharedconsts.ExtRM:   {"rv10", "rv20"},
		sharedconsts.ExtRMVB: {"rv20"},
		sharedconsts.ExtWMV:  {"wmv2"},
	}
	sweepOddAudio = map[string][]string{
		sharedconsts.ExtASF:  {"wmav1"},
		sharedconsts.ExtAVI:  {"adpcm_ima_wav", "pcm_s24le"},
		sharedconsts.ExtFLV:  {"adpcm_swf"},
		sharedconsts.ExtMKV:  {"pcm_s24le", "wavpack"},
		sharedconsts.ExtMOV:  {"pcm_s24le", "adpcm_ima_qt"},
		sharedconsts.ExtRM:   {"ac3"},
		sharedconsts.ExtRMVB: {"ac3"},
		sharedconsts.ExtWMV:  {"wmav2"},
	}
)

// Values cases pick from.
var (
	sweepPixFmts      = []string{"yuv420p10le", "yuv422p", "yuv422p10le", "yuv444p", "yuv444p10le", "yuv420p12le", "yuvj420p", "gray", "gray10le", "nv12", "gbrp", "yuva420p"}
	sweepSizes        = []string{"129x73", "130x74", "127x128", "72x128", "320x240", "64x64", "1280x720"}
	sweepRates        = []string{"30000/1001", "24000/1001", "24", "60", "15", "5", "120"}
	sweepSampleRates  = []string{"44100", "32000", "22050", "8000", "96000"}
	sweepLayouts      = []string{"mono", "2.1", "quad", "5.1(side)", "5.1", "7.1"}
	sweepSubtitleEncs = map[string][]string{
		sharedconsts.ExtM4V:  {"mov_text"},
		sharedconsts.ExtMKV:  {"srt", "ass", "webvtt"},
		sharedconsts.ExtMOV:  {"mov_text"},
		sharedconsts.ExtMP4:  {"mov_text"},
		sharedconsts.ExtWEBM: {"webvtt"},
	}

	sweepVideoAliases = []string{"x264", "avc1", "x265", "libx265", "hvc1", "svtav1", "libsvtav1", "mpeg2video", "mpg2", "vpx", "libvpx", "vpx9", "libvpx-vp9", "auto", "none"}
	sweepAudioAliases = []string{"libmp3lame", "ddp", "eac3", "dca", "aaclc", "libopus", "libvorbis", "pcm_s16le", "mp2", "auto", "none"}
	sweepLevels       = []string{"lc_", "mc_", "hc_"}

	// Args any encoder can take, then those for one encoder, which are nonsense when sent to another.
	sweepVideoArgs = []string{
		"-crf 0", "-crf 1", "-crf 18", "-crf 28", "-crf 51", "-crf 63", "-crf -5", "-crf 99",
		"-pix_fmt yuv420p", "-pix_fmt yuv420p10le", "-pix_fmt yuv420p10", "-pix_fmt yuv422p", "-pix_fmt yuv422p10le", "-pix_fmt yuv444p", "-pix_fmt yuv444p10le",
		"-pix_fmt yuv420p12le", "-pix_fmt yuvj420p", "-pix_fmt nv12", "-pix_fmt p010le", "-pix_fmt gray", "-pix_fmt gray10le", "-pix_fmt gbrp", "-pix_fmt yuva420p", "-pix_fmt rgb24", "-pix_fmt bgra",
		"-vf hflip", "-vf scale=64:-2", "-vf scale=-2:48", "-vf scale=65:65", "-vf fps=10", "-vf format=yuv420p", "-vf format=yuv444p", "-vf null", "-filter:v vflip", "-vf crop=64:64",
		"-pass 0", "-pass 1", "-pass 2", "-pass 3", "-pass 9",
		"-b:v 0", "-b:v 1", "-b:v 1500", "-b:v 100k", "-b:v 1M", "-b:v 1.5M", "-b:v:0 300k", "-b:v 72=100k,720=1M", "-b:v 720=1M,1080=3M", "-b:v 4k=10M",
		"-g 12", "-bf 0", "-r 30", "-s 64x64", "-aspect 16:9", "-maxrate 500k -bufsize 1M", "-threads 1",
		"-color_primaries bt709 -color_trc bt709 -colorspace bt709", "-movflags +faststart",
	}
	sweepCodecArgs = map[string][]string{
		sharedconsts.VCodecAV1:   {"-preset 0", "-preset 8", "-preset 12", "-svtav1-params tune=0", "-svtav1-params film-grain=8"},
		sharedconsts.VCodecH264:  {"-preset ultrafast", "-preset veryslow", "-tune film", "-tune zerolatency", "-profile:v baseline", "-profile:v main", "-profile:v high", "-level 31", "-x264-params keyint=12", "-x264opts no-scenecut", "-coder 0", "-tag:v avc1"},
		sharedconsts.VCodecHEVC:  {"-preset ultrafast", "-preset veryslow", "-tune grain", "-profile:v main", "-profile:v main10", "-x265-params log-level=error", "-x265-params lossless=1", "-tag:v hvc1"},
		sharedconsts.VCodecMPEG2: {"-q:v 4", "-qscale:v 5", "-trellis 1", "-profile:v main", "-level 8"},
		sharedconsts.VCodecVP8:   {"-deadline realtime -cpu-used 8", "-auto-alt-ref 0", "-qmin 10 -qmax 40"},
		sharedconsts.VCodecVP9:   {"-row-mt 1", "-deadline realtime -cpu-used 8", "-lossless 1", "-tile-columns 1", "-profile:v 2"},
	}
	sweepExtraArgs = []string{
		"-threads 1", "-movflags +faststart", "-map_metadata -1", "-fflags +bitexact", "-shortest", "-avoid_negative_ts make_zero",
		"-max_muxing_queue_size 1024", "-metadata:s:a:0 language=eng", "-disposition:v:0 default", "-strict -2",
		"-ac 1", "-ar 44100", "-b:a 128k", "-af volume=0.5", "-filter:a volume=0.5", "-q:a 2",
		"-an", "-vn", "-sn", "-dn", "-c:v libx264", "-c:a aac", "-c copy", "-f matroska", "-tag:v hvc1", "-bsf:v h264_mp4toannexb", "-bsf:a aac_adtstoasc",
		"-vsync cfr", "-fps_mode cfr", "-copyts", "-start_at_zero",
	}
	sweepThumbnailExts = []string{"jpg", "png", "bmp"}
)

// sweepTitle is the title in every case's metadata, which also gets written to the outputs.
const sweepTitle = `Black Clip — 日本語: "Encore" & <more>`

// sweepPick returns a random item.
func sweepPick(r *rand.Rand, items []string) string {
	return items[r.IntN(len(items))]
}

// sweepChance is true percent of the time.
func sweepChance(r *rand.Rand, percent int) bool {
	return r.IntN(100) < percent
}

// sweepSourceEncoders returns the video and audio encoders that can make a file of extension ext.
func sweepSourceEncoders(ext string) (video, audio []string) {
	for _, c := range sharedconsts.ContainerCompatibleVideoCodecs[ext] {
		video = append(video, consts.VCodecToFFVCodec[c])
	}
	for _, c := range sharedconsts.ContainerCompatibleAudioCodecs[ext] {
		if encoder := consts.ACodecToFFACodec[c]; !slices.Contains(audio, encoder) {
			audio = append(audio, encoder)
		}
	}
	return append(video, sweepOddVideo[ext]...), append(audio, sweepOddAudio[ext]...)
}

// randomSweepCase makes the case for r. Each setting usually stays plain, so a problem's cause is easy to see.
func randomSweepCase(r *rand.Rand) sweepCase {
	c := sweepPlainCase
	exts := slices.Sorted(maps.Keys(sharedconsts.AllVidExtensions))

	// The source's container and codecs.
	if sweepChance(r, 75) {
		c.Ext = sweepPick(r, exts)
		video, audio := sweepSourceEncoders(c.Ext)
		c.Video = sweepPick(r, video)
		c.Audio = sweepPick(r, audio)
	}
	if c.Video == "rv10" || c.Video == "rv20" {
		c.Size = "128x96" // RealVideo takes few sizes.
	}

	// The source's other settings.
	if sweepChance(r, 12) {
		c.PixFmt = sweepPick(r, sweepPixFmts)
	}
	if sweepChance(r, 12) && c.Size == sweepPlainCase.Size {
		c.Size = sweepPick(r, sweepSizes)

		// FFmpeg's H.264 and HEVC encoders refuse odd sizes, which real videos can still have, so another encoder makes the source.
		oddSize := c.Size == "129x73" || c.Size == "127x128"
		if oddSize && (c.Video == consts.FFVCodecKeyH264 || c.Video == consts.FFVCodecKeyH265) {
			c.Ext = sharedconsts.ExtMKV
			c.Video = consts.FFVCodecKeyVP9
		}
	}
	if sweepChance(r, 12) {
		c.Rate = sweepPick(r, sweepRates)
	}
	if sweepChance(r, 12) {
		c.SampleRate = sweepPick(r, sweepSampleRates)
	}
	if sweepChance(r, 15) {
		c.Layout = sweepPick(r, sweepLayouts)
	}
	if encoders := sweepSubtitleEncs[c.Ext]; len(encoders) > 0 && sweepChance(r, 20) {
		c.Subtitles = sweepPick(r, encoders)
	}
	if parsing.HoldsThumbnail(c.Ext) && sweepChance(r, 20) {
		c.Thumbnail = true
	}
	if sweepChance(r, 8) {
		c.SecondAudio = true
	}
	if sweepChance(r, 6) {
		c.Audio = ""
		c.SecondAudio = false
	}
	if sweepChance(r, 3) {
		c.Video = ""
	}

	// The output container, rarely one Metarr can't write.
	if sweepChance(r, 70) {
		c.OutputExt = sweepPick(r, exts)
		if parsing.ReadOnlyExt(c.OutputExt) && sweepChance(r, 90) {
			c.OutputExt = sharedconsts.ExtMKV
		}
	}
	outExt := c.OutputExt
	if outExt == "" {
		outExt = c.Ext
	}

	// The codecs asked for, usually ones the output container holds.
	videoCodecs := []string{sharedconsts.VCodecAV1, sharedconsts.VCodecH264, sharedconsts.VCodecHEVC, sharedconsts.VCodecMPEG2, sharedconsts.VCodecVP8, sharedconsts.VCodecVP9}
	audioCodecs := slices.DeleteFunc(slices.Sorted(maps.Keys(sharedconsts.ValidAudioCodecs)), func(s string) bool { return s == sharedconsts.ACodecCopy })
	if fits := sharedconsts.ContainerCompatibleVideoCodecs[outExt]; len(fits) > 0 && sweepChance(r, 90) {
		videoCodecs = fits
	}
	if fits := sharedconsts.ContainerCompatibleAudioCodecs[outExt]; len(fits) > 0 && sweepChance(r, 90) {
		audioCodecs = fits
	}
	sourceVideo, _ := sharedvalidation.ValidateVideoCodec(c.Video)
	sourceAudio, _ := sharedvalidation.ValidateAudioCodec(c.Audio)
	c.VideoCodecs = randomSweepCodecs(r, videoCodecs, sweepVideoAliases, sourceVideo, sharedconsts.VCodecCopy)
	c.AudioCodecs = randomSweepCodecs(r, audioCodecs, sweepAudioAliases, sourceAudio, sharedconsts.ACodecCopy)

	// The encoder args, usually for the codec asked for.
	if sweepChance(r, 50) {
		c.ArgsCodec = sweepPick(r, videoCodecs)
		if asked, err := sharedvalidation.ValidateVideoCodec(sweepCodecOutput(c.VideoCodecs)); err == nil && asked != "" && asked != sharedconsts.VCodecCopy {
			c.ArgsCodec = asked
		}
		c.VideoArgs = randomSweepArgs(r, c.ArgsCodec)
	}

	// The remaining flags.
	if sweepChance(r, 20) {
		c.ExtraArgs = sweepPick(r, sweepExtraArgs)
	}
	if sweepChance(r, 8) {
		c.GPU = sharedconsts.AccelTypeAuto
	}
	if sweepChance(r, 10) {
		c.ThumbnailFile = sweepPick(r, sweepThumbnailExts)
	}
	if sweepChance(r, 10) && c.ThumbnailFile == "" {
		c.StripThumbnail = true
	}
	if sweepChance(r, 10) {
		c.PurgeOriginal = true
	}
	if sweepChance(r, 5) {
		c.NoOverwrite = true
	}
	return c
}

// sweepTableCases returns a case for every video and audio pair the container tables say fit each container, from
// sources with codecs to copy and to encode, and with other sample rates and channels (some pairs only go wrong with those).
func sweepTableCases() []sweepCase {
	hevcSource := sweepPlainCase
	hevcSource.Ext = sharedconsts.ExtMKV
	hevcSource.Video = consts.FFVCodecKeyH265
	hevcSource.Audio = consts.FFACodecKeyOpus

	cdRateSource := sweepPlainCase
	cdRateSource.SampleRate = "44100"

	monoSource := sweepPlainCase
	monoSource.Layout = "mono"

	surroundSource := sweepPlainCase
	surroundSource.Layout = "5.1"

	var cases []sweepCase
	for _, ext := range slices.Sorted(maps.Keys(sharedconsts.ContainerCompatibleVideoCodecs)) {
		for _, video := range sharedconsts.ContainerCompatibleVideoCodecs[ext] {
			for _, audio := range sharedconsts.ContainerCompatibleAudioCodecs[ext] {
				for _, source := range []sweepCase{sweepPlainCase, hevcSource, cdRateSource, monoSource, surroundSource} {
					c := source
					c.OutputExt = ext
					c.VideoCodecs = video
					c.AudioCodecs = audio
					cases = append(cases, c)
				}
			}
		}
	}
	return cases
}

// sweepCopyCases returns a case for copying each codec out of each container into every container holding it, as how a
// stream comes out can depend on the container it came from.
func sweepCopyCases() []sweepCase {
	var cases []sweepCase
	exts := slices.Sorted(maps.Keys(sharedconsts.ContainerCompatibleVideoCodecs))
	for _, source := range exts {
		videos := sharedconsts.ContainerCompatibleVideoCodecs[source]
		audios := sharedconsts.ContainerCompatibleAudioCodecs[source]
		if len(videos) == 0 || len(audios) == 0 {
			continue
		}
		for _, out := range exts {
			// Each audio codec with the container's first video codec, and each other video codec with its first audio codec.
			var pairs [][2]string
			for _, audio := range audios {
				pairs = append(pairs, [2]string{videos[0], audio})
			}
			for _, video := range videos[1:] {
				pairs = append(pairs, [2]string{video, audios[0]})
			}
			for _, pair := range pairs {
				if !sharedvalidation.VideoCodecFits(out, pair[0]) || !sharedvalidation.AudioCodecFits(out, pair[1]) || parsing.ReadOnlyExt(out) {
					continue
				}
				c := sweepPlainCase
				c.Ext = source
				c.Video = consts.VCodecToFFVCodec[pair[0]]
				c.Audio = consts.ACodecToFFACodec[pair[1]]
				c.OutputExt = out
				cases = append(cases, c)
			}
		}
	}
	return cases
}

// randomSweepCodecs makes a transcode codec flag's value, blank for none.
func randomSweepCodecs(r *rand.Rand, codecs, aliases []string, source, copyCodec string) string {
	if source == "" || !slices.Contains(codecs, source) {
		source = sweepPick(r, codecs)
	}
	switch roll := r.IntN(100); {
	case roll < 25:
		return ""
	case roll < 40:
		return copyCodec
	case roll < 80:
		return sweepPick(r, codecs)
	case roll < 85:
		return sweepPick(r, aliases)
	case roll < 95:
		return source + ":" + sweepPick(r, codecs)
	}
	return sweepPick(r, sweepLevels) + source + ":" + sweepPick(r, codecs)
}

// randomSweepArgs picks encoder args for a codec, each option once, as Metarr refuses most given twice.
func randomSweepArgs(r *rand.Rand, codec string) string {
	var chosen []string
	used := map[string]bool{}
	for range 1 + r.IntN(3) {
		var arg string
		switch roll := r.IntN(100); {
		case roll < 55:
			arg = sweepPick(r, sweepVideoArgs)
		case roll < 90:
			arg = sweepPick(r, sweepCodecArgs[codec])
		default: // Another codec's, which Metarr can't tell apart.
			arg = sweepPick(r, sweepCodecArgs[sweepPick(r, slices.Sorted(maps.Keys(sweepCodecArgs)))])
		}
		option := strings.Fields(arg)[0]
		if !used[option] {
			used[option] = true
			chosen = append(chosen, arg)
		}
	}

	// Most encoders need a bitrate instead of a quality to make more than one pass.
	_, needsBitrate := sharedconsts.MultiPassNeedsBitrate[codec]
	if needsBitrate && (used["-pass"] && !slices.Contains(chosen, "-pass 0") && !slices.Contains(chosen, "-pass 1")) {
		chosen = slices.DeleteFunc(chosen, func(arg string) bool { return strings.HasPrefix(arg, "-crf ") || strings.HasPrefix(arg, "-b:v") })
		chosen = append(chosen, "-b:v "+sweepPick(r, []string{"1500", "100k", "1M", "72=100k,720=1M"}))
	}
	return strings.Join(chosen, sweepArgSeparator)
}

// sweepCodecOutput returns what a codec flag's value transcodes to.
func sweepCodecOutput(flag string) string {
	parts := strings.Split(flag, ":")
	return parts[len(parts)-1]
}

// simpler returns c with one setting put back to plain, for each setting that isn't.
func (c sweepCase) simpler() []sweepCase {
	var simpler []sweepCase

	// A plain container needs plain codecs, as it may not hold the others.
	if c.Ext != sweepPlainCase.Ext || c.Video != sweepPlainCase.Video || c.Audio != sweepPlainCase.Audio {
		s := c
		s.Ext = sweepPlainCase.Ext
		s.Video = sweepPlainCase.Video
		s.Audio = sweepPlainCase.Audio
		simpler = append(simpler, s)
	}

	// Each encoder arg alone.
	args := strings.Split(c.VideoArgs, sweepArgSeparator)
	for i := range args {
		if len(args) > 1 {
			s := c
			s.VideoArgs = strings.Join(slices.Delete(slices.Clone(args), i, i+1), sweepArgSeparator)
			simpler = append(simpler, s)
		}
	}

	// Each setting alone.
	plain := reflect.ValueOf(sweepPlainCase)
	for _, field := range reflect.VisibleFields(reflect.TypeFor[sweepCase]()) {
		if field.Anonymous || reflect.ValueOf(c).FieldByIndex(field.Index).Equal(plain.FieldByIndex(field.Index)) {
			continue
		}
		s := c
		reflect.ValueOf(&s).Elem().FieldByIndex(field.Index).Set(plain.FieldByIndex(field.Index))
		simpler = append(simpler, s)
	}
	return simpler
}

// flags returns the flags Metarr gets beyond its input files.
func (c sweepCase) flags(assets string) []string {
	values := []struct{ key, value string }{
		{keys.OutputFiletype, c.OutputExt},
		{keys.TranscodeVideoCodecInput, c.VideoCodecs},
		{keys.TranscodeAudioCodecInput, c.AudioCodecs},
		{keys.ExtraFFmpegArgs, c.ExtraArgs},
		{keys.TranscodeGPU, c.GPU},
	}
	if c.VideoArgs != "" {
		args := strings.ReplaceAll(c.VideoArgs, sweepArgSeparator, " ")
		values = append(values, struct{ key, value string }{keys.TranscodeVideoArgs, c.ArgsCodec + ":" + args})
	}
	if c.ThumbnailFile != "" {
		values = append(values, struct{ key, value string }{keys.Thumbnail, filepath.Join(assets, "thumb."+c.ThumbnailFile)})
	}

	var flags []string
	for _, v := range values {
		if v.value != "" {
			flags = append(flags, "--"+v.key+"="+v.value)
		}
	}
	switches := []struct {
		key string
		on  bool
	}{
		{keys.StripThumbnails, c.StripThumbnail},
		{keys.PurgeOriginalVideo, c.PurgeOriginal},
		{keys.NoFileOverwrite, c.NoOverwrite},
	}
	for _, s := range switches {
		if s.on {
			flags = append(flags, "--"+s.key)
		}
	}
	return flags
}

// describe returns a case's source and flags, for the report.
func (c sweepCase) describe(assets string) string {
	source := fmt.Sprintf("source %s video=%q audio=%q %s %s@%s %sHz %s", c.Ext, c.Video, c.Audio, c.PixFmt, c.Size, c.Rate, c.SampleRate, c.Layout)
	if c.Subtitles != "" {
		source += " subtitles=" + c.Subtitles
	}
	if c.Thumbnail {
		source += " thumbnail"
	}
	if c.SecondAudio {
		source += " two-audio-streams"
	}
	return source + "\n    flags  " + strings.Join(c.flags(assets), " ")
}

// makeArgs returns the FFmpeg arguments making the source at out.
func (src sweepSource) makeArgs(assets, out string) []string {
	args := []string{"-v", "error", "-y"}
	var maps []string
	inputs := 0

	// Inputs, and the streams to take from them.
	if src.Video != "" {
		args = append(args, "-f", "lavfi", "-i", fmt.Sprintf("color=black:s=%s:r=%s:d=%s", src.Size, src.Rate, sweepClipSeconds))
		maps = append(maps, "-map", strconv.Itoa(inputs)+":v")
		inputs++
	}
	if src.Audio != "" {
		args = append(args, "-f", "lavfi", "-i", fmt.Sprintf("sine=d=%s:r=%s", sweepClipSeconds, src.SampleRate))
		maps = append(maps, "-map", strconv.Itoa(inputs)+":a")
		if src.SecondAudio {
			maps = append(maps, "-map", strconv.Itoa(inputs)+":a")
		}
		inputs++
	}
	if src.Subtitles != "" {
		args = append(args, "-i", filepath.Join(assets, "subtitles.srt"))
		maps = append(maps, "-map", strconv.Itoa(inputs)+":s")
		inputs++
	}
	embeddedThumbnail := src.Thumbnail && src.Video != "" && src.Ext != sharedconsts.ExtMKV
	if embeddedThumbnail {
		args = append(args, "-i", filepath.Join(assets, "thumb.jpg"))
		maps = append(maps, "-map", strconv.Itoa(inputs)+":v")
	}
	args = append(args, maps...)

	// Encoders.
	if src.Video != "" {
		args = append(args, "-c:v:0", src.Video, "-pix_fmt:v:0", src.PixFmt)
	}
	if src.Audio != "" {
		args = append(args, "-c:a", src.Audio, "-af", "aformat=channel_layouts="+src.Layout, "-strict", "experimental")
	}
	if src.Subtitles != "" {
		args = append(args, "-c:s", src.Subtitles)
	}
	if embeddedThumbnail {
		args = append(args, "-c:v:1", "copy", "-disposition:v:1", "attached_pic")
	}
	if src.Thumbnail && src.Ext == sharedconsts.ExtMKV {
		args = append(args, "-attach", filepath.Join(assets, "thumb.jpg"), "-metadata:s:t:0", "mimetype=image/jpeg", "-metadata:s:t:0", "filename=cover.jpg")
	}

	// Muxers FFmpeg can't pick from the extension.
	switch src.Ext {
	case sharedconsts.ExtOGM:
		args = append(args, "-f", "ogv")
	case sharedconsts.ExtRMVB:
		args = append(args, "-f", "rm")
	}
	return append(args, out)
}

// sweepProbed is what FFprobe reports of a file's streams.
type sweepProbed struct {
	Streams []struct {
		CodecType   string `json:"codec_type"`
		CodecName   string `json:"codec_name"`
		PixFmt      string `json:"pix_fmt"`
		Width       int    `json:"width"`
		Height      int    `json:"height"`
		Disposition struct {
			AttachedPic int `json:"attached_pic"`
		} `json:"disposition"`
	} `json:"streams"`
}

// sweepProbe reads a file's streams.
func sweepProbe(path string) (sweepProbed, error) {
	var probed sweepProbed
	out, err := exec.Command(sharedconsts.ProgramFFprobe, "-v", "error", "-show_entries",
		"stream=codec_type,codec_name,pix_fmt,width,height:stream_disposition=attached_pic", "-of", "json", path).Output()
	if err != nil {
		return probed, err
	}
	return probed, json.Unmarshal(out, &probed)
}

// video returns the main video stream's codec, pixel format and size, blank if there's none.
func (p sweepProbed) video() (codec, pixFmt, size string) {
	for _, s := range p.Streams {
		if s.CodecType == "video" && s.Disposition.AttachedPic == 0 {
			return s.CodecName, s.PixFmt, fmt.Sprintf("%dx%d", s.Width, s.Height)
		}
	}
	return "", "", ""
}

// audio returns the audio streams' codecs.
func (p sweepProbed) audio() []string {
	var codecs []string
	for _, s := range p.Streams {
		if s.CodecType == "audio" {
			codecs = append(codecs, s.CodecName)
		}
	}
	return codecs
}

// count returns how many streams there are of a type.
func (p sweepProbed) count(codecType string) int {
	n := 0
	for _, s := range p.Streams {
		if s.CodecType == codecType {
			n++
		}
	}
	return n
}

// thumbnails returns how many embedded thumbnails there are.
func (p sweepProbed) thumbnails() int {
	n := 0
	for _, s := range p.Streams {
		if s.Disposition.AttachedPic == 1 {
			n++
		}
	}
	return n
}

// sweepDecoded is what decoding a file's main streams found.
type sweepDecoded struct {
	err         error
	errors      string // What FFmpeg printed as errors while decoding.
	videoFrames int
	audioFrames int
	videoStart  float64
	videoEnd    float64
	audioStart  float64
	audioEnd    float64
	maxY        float64
	worstUV     float64 // The furthest U or V got from the middle.
	loudestDB   float64
}

// sweepDecode decodes a file's first video and audio streams in full, reading the picture and sound levels of each frame.
func sweepDecode(path, scratch string) sweepDecoded {
	videoLevels := filepath.Join(scratch, "video.levels")
	audioLevels := filepath.Join(scratch, "audio.levels")
	cmd := exec.Command(sharedconsts.ProgramFFmpeg, "-hide_banner", "-nostats", "-v", "error", "-y", "-i", path,
		"-map", "0:V:0?", "-map", "0:a:0?",
		"-vf", "format=yuv420p,signalstats,metadata=print:file="+videoLevels,
		"-af", "astats=metadata=1:reset=1,ametadata=print:key=lavfi.astats.Overall.RMS_level:file="+audioLevels,
		"-f", "null", "-")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	decoded := sweepDecoded{err: cmd.Run(), loudestDB: math.Inf(-1)}
	decoded.errors = strings.TrimSpace(stderr.String())

	// Video levels, as a frame's time followed by its values.
	lines, _ := os.ReadFile(videoLevels)
	for line := range strings.Lines(string(lines)) {
		line = strings.TrimSpace(line)
		if _, pts, found := strings.Cut(line, "pts_time:"); found {
			when, _ := strconv.ParseFloat(pts, 64)
			if decoded.videoFrames == 0 {
				decoded.videoStart = when
			}
			decoded.videoEnd = when
			decoded.videoFrames++
			continue
		}
		key, value, _ := strings.Cut(line, "=")
		level, err := strconv.ParseFloat(value, 64)
		if err != nil {
			continue
		}
		switch key {
		case "lavfi.signalstats.YAVG":
			decoded.maxY = max(decoded.maxY, level)
		case "lavfi.signalstats.UAVG", "lavfi.signalstats.VAVG":
			decoded.worstUV = max(decoded.worstUV, math.Abs(level-sweepBlackMidUV))
		}
	}

	// Audio levels, likewise.
	lines, _ = os.ReadFile(audioLevels)
	for line := range strings.Lines(string(lines)) {
		line = strings.TrimSpace(line)
		if _, pts, found := strings.Cut(line, "pts_time:"); found {
			when, _ := strconv.ParseFloat(pts, 64)
			if decoded.audioFrames == 0 {
				decoded.audioStart = when
			}
			decoded.audioEnd = when
			decoded.audioFrames++
			continue
		}
		if _, value, found := strings.Cut(line, "RMS_level="); found {
			if level, err := strconv.ParseFloat(value, 64); err == nil && !math.IsNaN(level) {
				decoded.loudestDB = max(decoded.loudestDB, level)
			}
		}
	}
	return decoded
}

// sweepFixture is a source video, made once.
type sweepFixture struct {
	once    sync.Once
	data    []byte
	probed  sweepProbed
	decoded sweepDecoded
	err     error
}

// sweepIssue is something wrong with how a case ended. Cases with the same class and detail share a cause.
type sweepIssue struct {
	class  string
	detail string
	note   string // Measurements, which differ case to case.
}

// key returns what the issue is grouped by.
func (i sweepIssue) key() string {
	if i.detail == "" {
		return i.class
	}
	return i.class + ": " + i.detail
}

// sweepResult is how a case ended.
type sweepResult struct {
	outcome string
	reason  string // Why it was refused, or couldn't be made.
	issues  []sweepIssue
	command string // The last FFmpeg command Metarr ran.
}

// has reports whether the result found the issue.
func (r sweepResult) has(key string) bool {
	return slices.ContainsFunc(r.issues, func(i sweepIssue) bool { return i.key() == key })
}

// sweep holds what every case shares.
type sweep struct {
	t        *testing.T
	root     string
	assets   string
	exe      string
	logURL   string
	fixtures sync.Map // sweepSource to *sweepFixture.
}

// fixture makes a source the first time it's asked for.
func (sw *sweep) fixture(src sweepSource) *sweepFixture {
	entry, _ := sw.fixtures.LoadOrStore(src, &sweepFixture{})
	fx, ok := entry.(*sweepFixture)
	if !ok {
		return &sweepFixture{err: errors.New("fixture map holds the wrong type")}
	}
	fx.once.Do(func() {
		dir, err := os.MkdirTemp(sw.root, "source-")
		if err != nil {
			fx.err = err
			return
		}
		defer func() { _ = os.RemoveAll(dir) }()

		path := filepath.Join(dir, "source"+src.Ext)
		if out, err := exec.Command(sharedconsts.ProgramFFmpeg, src.makeArgs(sw.assets, path)...).CombinedOutput(); err != nil {
			fx.err = fmt.Errorf("%s", sweepCleanLine(sweepFirstError(string(out))))
			return
		}
		if fx.data, err = os.ReadFile(path); err != nil {
			fx.err = err
			return
		}
		if fx.probed, err = sweepProbe(path); err != nil {
			fx.err = fmt.Errorf("source can't be probed: %w", err)
			return
		}
		fx.decoded = sweepDecode(path, dir)
		if fx.decoded.err != nil {
			fx.err = fmt.Errorf("source can't be decoded: %w", fx.decoded.err)
		}
	})
	return fx
}

// Patterns found in Metarr's output.
var (
	sweepFinalPath   = regexp.MustCompile(`(?m)^final video path: (.+)$`)
	sweepMetarrError = regexp.MustCompile(`(?m)^metarr error: (.+)$`)
	sweepCommand     = regexp.MustCompile(`Constructed FFmpeg command for [^\n]*\n\n([^\n]+)`)
	sweepAddress     = regexp.MustCompile(`0x[0-9a-f]+`)
	sweepTempPath    = regexp.MustCompile(`/[^\s'"]*(/case-|/source-)[^\s'"]*`)
	sweepSizeNumbers = regexp.MustCompile(`\b\d+x\d+\b`)
	sweepNumbers     = regexp.MustCompile(`\b\d+(\.\d+)?\b`)
	sweepMetadata    = regexp.MustCompile(`(?: -metadata \S+?=.*?)+( -\S| <file>)`)
	sweepErrorWords  = regexp.MustCompile(`(?i)\b(error|invalid|could not|cannot|can't|not supported|unsupported|unrecognized|unknown|unable|not divisible|must be|too (low|high|many|few)|failed|requires?|incorrect|not allowed|not implemented|only .*supported|at most|not (a )?valid|no such)\b`)
)

// sweepCleanRefusals are how Metarr words refusing a video it can't write, rather than FFmpeg failing on it.
var sweepCleanRefusals = []string{"can't go in a", "can't be written into any container", "can't encode pixel format", "isn't installed", "no encoder for a codec", "can't take the odd size"}

// sweepNoise are lines that sound like the cause of a failure, but aren't.
var sweepNoise = []string{
	"Failed to set thread priority", "Incompatible pixel format", "deprecated", "Error while", "Task finished", "Terminating thread",
	"Nothing was written", "Could not open encoder before EOF", "Conversion failed", "Error sending frames", "Error submitting",
	"configuration:", "--enable", "Error closing", "Stream specifier", "Multiple -codec", "Stream #", "Svt[warn]", "[info]",
	"Error during demuxing", "Error muxing", "Error writing trailer", "Error opening output file", "Error opening output files",
	"Cannot load lib", "AVHWDeviceContext", "too many threads/slices", "\x1b[", "[warning]", "dynamically load", "Device creation failed",
}

// sweepCoarse words failures sharing a cause alike, whatever muxer, codec or value they name.
var sweepCoarse = []struct {
	pattern *regexp.Regexp
	wording string
}{
	{regexp.MustCompile(`Could not find tag for codec`), "the output container has no tag for the codec"},
	{regexp.MustCompile(`is not supported by the bitstream filter`), "a bitstream filter was given a codec it doesn't take"},
	{regexp.MustCompile(`Unable to parse "\w+" option value`), "the encoder doesn't know an option value"},
	{regexp.MustCompile(`Invalid channel layout`), "libopus can't take the channel layout"},
}

// sweepFirstError returns the first line of FFmpeg's output that looks like what went wrong, else its last line.
func sweepFirstError(output string) string {
	last := ""
	for line := range strings.Lines(output) {
		noisy := slices.ContainsFunc(sweepNoise, func(n string) bool { return strings.Contains(line, n) })
		if noisy {
			continue
		}
		if sweepErrorWords.MatchString(line) {
			return line
		}
		if strings.TrimSpace(line) != "" {
			last = line
		}
	}
	return last
}

// sweepCleanLine strips what differs between runs of the same failure from a line.
func sweepCleanLine(line string) string {
	for _, coarse := range sweepCoarse {
		if coarse.pattern.MatchString(line) {
			return coarse.wording
		}
	}
	line = sweepAddress.ReplaceAllString(line, "0x")
	line = sweepTempPath.ReplaceAllString(line, "<file>")
	line = sweepSizeNumbers.ReplaceAllString(line, "<size>")
	line = sweepNumbers.ReplaceAllString(line, "N")
	return strings.TrimSpace(line)
}

// sweepCleanCommand strips the paths and metadata from an FFmpeg command, to read it easily.
func sweepCleanCommand(command string) string {
	command = sweepTempPath.ReplaceAllString(command, "<file>")
	return sweepMetadata.ReplaceAllString(command, " -metadata ...$1")
}

// sweepReason returns Metarr's stated error, up to the part naming other containers.
func sweepReason(output string) string {
	match := sweepMetarrError.FindStringSubmatch(output)
	if match == nil {
		return "no stated reason"
	}
	reason, _, _ := strings.Cut(sweepCleanLine(match[1]), " It fits:")
	reason, _, _ = strings.Cut(reason, " Supported:")
	return reason
}

// run runs a case and checks how it ended.
func (sw *sweep) run(c sweepCase) sweepResult {
	fx := sw.fixture(c.sweepSource)
	if fx.err != nil {
		return sweepResult{outcome: sweepUnmakeable, reason: fx.err.Error()}
	}

	// Lay out the case's folders.
	dir, err := os.MkdirTemp(sw.root, "case-")
	if err != nil {
		sw.t.Error(err)
		return sweepResult{outcome: sweepUnmakeable, reason: err.Error()}
	}
	defer func() { _ = os.RemoveAll(dir) }()
	work := filepath.Join(dir, "work")
	home := filepath.Join(dir, "home")
	tmp := filepath.Join(dir, "tmp")
	scratch := filepath.Join(dir, "scratch")
	for _, d := range []string{work, home, tmp, scratch} {
		if err := os.Mkdir(d, 0o755); err != nil {
			sw.t.Error(err)
			return sweepResult{outcome: sweepUnmakeable, reason: err.Error()}
		}
	}

	// Lay out the video and its metadata.
	video := filepath.Join(work, "video"+c.Ext)
	meta := filepath.Join(work, "video.json")
	info, err := json.Marshal(map[string]any{
		"id":          "sweep",
		"title":       sweepTitle,
		"fulltitle":   sweepTitle,
		"description": "About " + sweepTitle,
		"uploader":    "Sweep Uploader",
		"channel":     "Sweep Channel",
		"upload_date": "20240102",
		"webpage_url": "https://example.com/watch?v=sweep",
	})
	if err == nil {
		err = errors.Join(os.WriteFile(video, fx.data, 0o644), os.WriteFile(meta, info, 0o644))
	}
	if err != nil {
		sw.t.Error(err)
		return sweepResult{outcome: sweepUnmakeable, reason: err.Error()}
	}

	// Run Metarr.
	ctx, cancel := context.WithTimeout(sw.t.Context(), fuzzRunTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, sw.exe, append([]string{"--" + keys.VideoFiles + "=" + video, "--" + keys.MetaFiles + "=" + meta}, c.flags(sw.assets)...)...)
	cmd.Dir = work
	cmd.Env = []string{"PATH=" + os.Getenv("PATH"), "HOME=" + home, "TMPDIR=" + tmp, "TUBARR_LOG_URL=" + sw.logURL, metarrAsMainEnv + "=1"}
	cmd.WaitDelay = 5 * time.Second

	// A hung run's FFmpeg has to go with Metarr, or it runs on, filling memory and disk.
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	}
	out, runErr := cmd.CombinedOutput()
	output := string(out)

	// Read how it ended.
	result := sweepEnding(ctx, output, runErr)
	if matches := sweepCommand.FindAllStringSubmatch(output, -1); len(matches) > 0 {
		result.command = sweepCleanCommand(matches[len(matches)-1][1])
	}
	if result.outcome == sweepProblem && len(result.issues) > 0 && (result.issues[0].class == "hung" || result.issues[0].class == "panicked") {
		return result
	}

	// A file in the wrong place or an original gone is wrong however the run ended.
	final := ""
	if match := sweepFinalPath.FindStringSubmatch(output); match != nil && runErr == nil {
		final = match[1]
	}
	result.issues = append(result.issues, sweepCheckFiles(c, fx, work, tmp, final, runErr == nil)...)

	// A finished run should have left the video asked for.
	if runErr == nil {
		if final == "" {
			result.issues = append(result.issues, sweepIssue{class: "no-final-path", detail: "Metarr succeeded without naming a video"})
		} else {
			result.issues = append(result.issues, sweepCheckOutput(c, fx, final, scratch)...)
		}
	}
	if len(result.issues) > 0 {
		result.outcome = sweepProblem
	}
	return result
}

// sweepEnding reads how a run of Metarr ended, from its exit and output.
func sweepEnding(ctx context.Context, output string, runErr error) sweepResult {
	var exitErr *exec.ExitError
	switch {
	case runErr == nil:
		return sweepResult{outcome: sweepFine}
	case ctx.Err() != nil:
		return sweepResult{outcome: sweepProblem, issues: []sweepIssue{{class: "hung"}}}
	case strings.Contains(output, "Panic recovered") || strings.Contains(output, "panic:"):
		return sweepResult{outcome: sweepProblem, issues: []sweepIssue{{class: "panicked", detail: sweepCleanLine(sweepFirstError(output))}}}
	case !errors.As(runErr, &exitErr) || exitErr.ExitCode() != 1:
		return sweepResult{outcome: sweepProblem, issues: []sweepIssue{{class: "bad-exit", detail: runErr.Error()}}}
	case strings.Contains(output, "Failed to execute cobra"):
		return sweepResult{outcome: sweepRefusedFlags, reason: sweepReason(output)}
	}

	// FFmpeg ran and failed: what it said comes after this. Checked before refusals, as Metarr logs the same words when it swaps a codec.
	if _, captured, found := strings.Cut(output, "Captured output:"); found {
		// Metarr's log trailer, as "[" then colour, ends FFmpeg's output.
		captured, _, _ = strings.Cut(captured, "\x1b[")
		captured = strings.TrimSuffix(strings.TrimSpace(captured), "[")
		return sweepResult{outcome: sweepProblem, issues: []sweepIssue{{class: "ffmpeg-failed", detail: sweepCleanLine(sweepFirstError(captured))}}}
	}
	if slices.ContainsFunc(sweepCleanRefusals, func(r string) bool { return strings.Contains(output, r) }) {
		return sweepResult{outcome: sweepRefusedRun, reason: sweepReason(output)}
	}
	return sweepResult{outcome: sweepProblem, issues: []sweepIssue{{class: "metarr-failed", detail: sweepReason(output)}}}
}

// sweepCheckFiles checks what's left in the work and temp folders.
func sweepCheckFiles(c sweepCase, fx *sweepFixture, work, tmp, final string, ok bool) []sweepIssue {
	var issues []sweepIssue
	originals := 0
	entries, err := os.ReadDir(work)
	if err != nil {
		return []sweepIssue{{class: "unreadable-folder", detail: err.Error()}}
	}
	for _, e := range entries {
		path := filepath.Join(work, e.Name())
		if data, err := os.ReadFile(path); err == nil && bytes.Equal(data, fx.data) {
			originals++
			continue
		}
		ext := strings.ToLower(filepath.Ext(e.Name()))
		_, isVideo := sharedconsts.AllVidExtensions[ext]
		switch {
		case strings.HasPrefix(e.Name(), sharedconsts.TempTag):
			issues = append(issues, sweepIssue{class: "temp-file-left"})
		case ext == sharedconsts.MExtJSON || ext == sharedconsts.MExtNFO || strings.Contains(e.Name(), sharedconsts.BackupTag) || path == final:
		case isVideo && !ok:
			issues = append(issues, sweepIssue{class: "changed-video-after-failure", note: e.Name()})
		case !isVideo:
			issues = append(issues, sweepIssue{class: "stray-file", detail: ext})
		}
	}

	// The original stays unless it's purged by a run that worked.
	if originals == 0 && (!c.PurgeOriginal || !ok) {
		issues = append(issues, sweepIssue{class: "original-lost"})
	}

	// Metarr's temp folder is emptied.
	if left, err := os.ReadDir(tmp); err == nil && len(left) > 0 {
		issues = append(issues, sweepIssue{class: "temp-folder-left-dirty", note: left[0].Name()})
	}
	return issues
}

// sweepWant returns the codec flags ask for a source's codec: "" if they ask none, unsettled if it depends on how compressed the source is.
func sweepWant(all string, pairs map[sharedmodels.FromCodec]string, source string) (want string, settled bool) {
	for from := range pairs {
		if from.Codec == source && from.Compression != sharedconsts.CompressionBlank {
			return "", false
		}
	}
	if to, ok := pairs[sharedmodels.NewFromCodec(source, sharedconsts.CompressionBlank)]; ok {
		return to, true
	}
	return all, true
}

// sweepNormalVideo returns a probed video codec's name as flags use it, or as it is if flags have none.
func sweepNormalVideo(codec string) string {
	if normal, err := sharedvalidation.ValidateVideoCodec(codec); err == nil && normal != "" {
		return normal
	}
	return codec
}

// sweepNormalAudio returns a probed audio codec's name as flags use it, or as it is if flags have none.
func sweepNormalAudio(codec string) string {
	normal := codec
	if n, err := sharedvalidation.ValidateAudioCodec(codec); err == nil && n != "" {
		normal = n
	}
	if normal == sharedconsts.ACodecWAV {
		return sharedconsts.ACodecPCM // The same thing, as written.
	}
	return normal
}

// sweepEvenSize returns a size with any odd side cut by one, as Metarr does for encoders refusing odd sizes.
func sweepEvenSize(size string) string {
	var width, height int
	if _, err := fmt.Sscanf(size, "%dx%d", &width, &height); err != nil {
		return size
	}
	return fmt.Sprintf("%dx%d", width-width%2, height-height%2)
}

// sweepHasOption reports whether an option appears in args.
func sweepHasOption(args, option string) bool {
	return slices.Contains(strings.Fields(strings.ReplaceAll(args, sweepArgSeparator, " ")), option)
}

// sweepCheckOutput checks the video a successful run left.
func sweepCheckOutput(c sweepCase, fx *sweepFixture, final, scratch string) []sweepIssue {
	var issues []sweepIssue

	// Its extension.
	wantExt := c.Ext
	if parsing.ReadOnlyExt(c.Ext) {
		wantExt = sharedconsts.ExtMKV
	}
	if c.OutputExt != "" {
		wantExt, _ = sharedvalidation.ValidateFFmpegOutputExt(c.OutputExt)
	}
	if !strings.EqualFold(filepath.Ext(final), wantExt) {
		issues = append(issues, sweepIssue{class: "wrong-extension", detail: "asked " + wantExt, note: final})
	}

	// It opens, and decodes in full without complaint beyond what the source's decode said.
	probed, err := sweepProbe(final)
	if err != nil {
		return append(issues, sweepIssue{class: "unreadable", detail: err.Error()})
	}
	decoded := sweepDecode(final, scratch)
	if decoded.err != nil {
		return append(issues, sweepIssue{class: "undecodable", detail: sweepCleanLine(sweepFirstError(decoded.errors))})
	}
	if complaint := sweepDecodeComplaint(decoded.errors); complaint != "" && !strings.Contains(fx.decoded.errors, complaint) {
		issues = append(issues, sweepIssue{class: "decode-errors", detail: sweepCleanLine(complaint)})
	}

	// The video: there, in the codec asked, at the settings asked.
	sourceCodec, _, sourceSize := fx.probed.video()
	gotCodec, gotPixFmt, gotSize := probed.video()
	sourceVideo := sweepNormalVideo(sourceCodec)
	gotVideo := sweepNormalVideo(gotCodec)
	switch {
	case sourceCodec != "" && gotCodec == "":
		issues = append(issues, sweepIssue{class: "video-lost"})
	case gotCodec != "":
		issues = append(issues, sweepCheckVideoCodec(c, wantExt, sourceVideo, gotVideo)...)
		issues = append(issues, sweepCheckPixFmt(c, sourceVideo, gotVideo, gotPixFmt)...)
		resized := sweepHasOption(c.VideoArgs, "-vf") || sweepHasOption(c.VideoArgs, "-filter:v") || sweepHasOption(c.VideoArgs, "-s")
		evenedOut := (gotVideo == sharedconsts.VCodecH264 || gotVideo == sharedconsts.VCodecHEVC) && gotSize == sweepEvenSize(sourceSize)
		if gotSize != sourceSize && !resized && !evenedOut {
			issues = append(issues, sweepIssue{class: "wrong-size", note: sourceSize + " became " + gotSize})
		}
		// Metarr lowers a target bitrate to the source's, which for black is so low the picture really does go grey.
		lowBitrate := strings.Contains(c.VideoArgs, "-b:v")
		switch {
		case decoded.videoFrames == 0:
			issues = append(issues, sweepIssue{class: "no-video-frames"})
		case !lowBitrate && (decoded.maxY > sweepBlackMaxY || decoded.worstUV > sweepBlackUVRange):
			issues = append(issues, sweepIssue{class: "picture-not-black", note: fmt.Sprintf("Y up to %.1f, U/V off by up to %.1f", decoded.maxY, decoded.worstUV)})
		}
		sourceLength := fx.decoded.videoEnd - fx.decoded.videoStart
		gotLength := decoded.videoEnd - decoded.videoStart
		if math.Abs(gotLength-sourceLength) > sweepMaxDrift {
			issues = append(issues, sweepIssue{class: "video-wrong-length", note: fmt.Sprintf("%.2fs became %.2fs", sourceLength, gotLength)})
		}
	}

	// The audio: every stream, in the codec asked, audible and the right length.
	sourceAudio := fx.probed.audio()
	gotAudio := probed.audio()
	switch {
	case sweepHasOption(c.ExtraArgs, "-an"):
	case len(gotAudio) != len(sourceAudio):
		issues = append(issues, sweepIssue{class: "audio-streams-lost", note: fmt.Sprintf("%d became %d", len(sourceAudio), len(gotAudio))})
	case len(gotAudio) > 0:
		issues = append(issues, sweepCheckAudioCodec(c, wantExt, sweepNormalAudio(sourceAudio[0]), sweepNormalAudio(gotAudio[0]))...)
		switch {
		case decoded.audioFrames == 0:
			issues = append(issues, sweepIssue{class: "no-audio-frames"})
		case decoded.loudestDB < sweepSilenceDB:
			issues = append(issues, sweepIssue{class: "audio-silent", note: fmt.Sprintf("%.1f dB", decoded.loudestDB)})
		}
		sourceLength := fx.decoded.audioEnd - fx.decoded.audioStart
		gotLength := decoded.audioEnd - decoded.audioStart
		if math.Abs(gotLength-sourceLength) > sweepMaxDrift {
			issues = append(issues, sweepIssue{class: "audio-wrong-length", note: fmt.Sprintf("%.2fs became %.2fs", sourceLength, gotLength)})
		}
		if decoded.videoFrames > 0 && decoded.audioFrames > 0 && math.Abs(decoded.videoStart-decoded.audioStart) > sweepMaxDrift {
			issues = append(issues, sweepIssue{class: "audio-out-of-sync", note: fmt.Sprintf("video starts at %.2fs, audio at %.2fs", decoded.videoStart, decoded.audioStart)})
		}
	}

	// Subtitles stay where the output can hold them, as Metarr's stream mapping says.
	canHoldSubtitles := wantExt == c.Ext || wantExt == sharedconsts.ExtMKV || wantExt == sharedconsts.ExtMP4 || wantExt == sharedconsts.ExtM4V || wantExt == sharedconsts.ExtMOV || wantExt == sharedconsts.ExtWEBM
	if fx.probed.count("subtitle") > 0 && canHoldSubtitles && !sweepHasOption(c.ExtraArgs, "-sn") && probed.count("subtitle") == 0 {
		issues = append(issues, sweepIssue{class: "subtitles-lost", detail: c.Ext + " to " + wantExt})
	}

	// Thumbnails stay unless stripped, where the output can hold them.
	hasThumbnail := fx.probed.thumbnails() > 0 || c.ThumbnailFile != ""
	switch {
	case c.StripThumbnail && probed.thumbnails() > 0:
		issues = append(issues, sweepIssue{class: "thumbnail-not-stripped", detail: c.Ext + " to " + wantExt})
	case !c.StripThumbnail && hasThumbnail && parsing.HoldsThumbnail(wantExt) && gotCodec != "" && probed.thumbnails() == 0:
		issues = append(issues, sweepIssue{class: "thumbnail-lost", detail: c.Ext + " to " + wantExt})
	}
	return issues
}

// sweepCheckVideoCodec checks the output's video codec is the one asked.
func sweepCheckVideoCodec(c sweepCase, ext, source, got string) []sweepIssue {
	var entries []string
	if c.VideoCodecs != "" {
		entries = []string{c.VideoCodecs}
	}
	all, pairs, err := sharedparsing.ParseCodecMappings(entries, sharedvalidation.ValidateVideoCodec)
	if err != nil {
		return nil
	}
	want, settled := sweepWant(all, pairs, source)
	switch {
	case !settled:
		return nil
	case want == "" && !sharedvalidation.VideoCodecFits(ext, source):
		return nil // Metarr picks one that fits.
	case want == "" || want == sharedconsts.VCodecCopy:
		want = source
	}
	if got != want {
		return []sweepIssue{{class: "wrong-video-codec", detail: sweepCodecMismatch(c, sweepVideoOverrides, source, want, got)}}
	}
	return nil
}

// Extra FFmpeg args that replace the codec Metarr chose.
var (
	sweepVideoOverrides = []string{"-c:v", "-c"}
	sweepAudioOverrides = []string{"-c:a", "-c"}
)

// sweepCodecMismatch words a codec coming out other than asked, grouping those with one likely cause.
func sweepCodecMismatch(c sweepCase, overrides []string, source, want, got string) string {
	switch {
	case slices.ContainsFunc(overrides, func(o string) bool { return sweepHasOption(c.ExtraArgs, o) }):
		return "extra FFmpeg args replaced the codec"
	case got == source && c.Video == "":
		return "codec asked not applied to a file with no video"
	case got == source:
		return "codec asked not applied to a " + source + " source"
	}
	return fmt.Sprintf("asked %s, got %s", want, got)
}

// sweepCheckAudioCodec checks the output's audio codec is the one asked.
func sweepCheckAudioCodec(c sweepCase, ext, source, got string) []sweepIssue {
	var entries []string
	if c.AudioCodecs != "" {
		entries = []string{c.AudioCodecs}
	}
	all, pairs, err := sharedparsing.ParseCodecMappings(entries, sharedvalidation.ValidateAudioCodec)
	if err != nil {
		return nil
	}
	want, settled := sweepWant(all, pairs, source)
	switch {
	case !settled:
		return nil
	case want == "" && !sharedvalidation.AudioCodecFits(ext, source):
		return nil // Metarr picks one that fits.
	case want == "" || want == sharedconsts.ACodecCopy:
		want = source
	}
	want = sweepNormalAudio(want)

	// MP4-family files label all MPEG-1 audio mp3, so either name can read back as the other going in or out of one.
	mpegAudio := (want == sharedconsts.ACodecMP2 || want == sharedconsts.ACodecMP3) && (got == sharedconsts.ACodecMP2 || got == sharedconsts.ACodecMP3)
	if mpegAudio && (sweepMP4Family(ext) || sweepMP4Family(c.Ext)) {
		return nil
	}
	if got != want {
		return []sweepIssue{{class: "wrong-audio-codec", detail: sweepCodecMismatch(c, sweepAudioOverrides, source, want, got)}}
	}
	return nil
}

// sweepMP4Family reports whether ext is one of the containers based on MP4.
func sweepMP4Family(ext string) bool {
	switch ext {
	case sharedconsts.ExtMP4, sharedconsts.ExtM4V, sharedconsts.ExtMOV, sharedconsts.Ext3GP, sharedconsts.Ext3G2:
		return true
	}
	return false
}

// sweepCheckPixFmt checks an encoded video came out in the pixel format asked.
func sweepCheckPixFmt(c sweepCase, source, got, gotPixFmt string) []sweepIssue {
	// Only an encoder sets a pixel format, and Metarr only encodes a video in its own codec if nothing names another for it.
	if got != c.ArgsCodec || (got == source && !sweepEncodesOwnCodec(c, source)) {
		return nil
	}
	args := strings.Fields(strings.ReplaceAll(c.VideoArgs, sweepArgSeparator, " "))
	at := slices.Index(args, "-pix_fmt")
	if at < 0 || at+1 >= len(args) {
		return nil
	}
	asked := args[at+1]
	if asked == "yuv420p10" {
		asked += "le" // Metarr adds a missing "le".
	}

	// Decoders report these as another format: H.264's gray as 4:2:0, and VP8 and VP9's alpha is stored apart from the picture.
	if (got == sharedconsts.VCodecH264 && strings.HasPrefix(asked, "gray")) || asked == "yuva420p" {
		return nil
	}

	// Full range formats have the same bit depth and chroma.
	asked = strings.Replace(asked, "yuvj", "yuv", 1)
	gotPixFmt = strings.Replace(gotPixFmt, "yuvj", "yuv", 1)
	if gotPixFmt != asked && !slices.Contains(consts.PixFmtEquivalents[asked], gotPixFmt) {
		return []sweepIssue{{class: "wrong-pixel-format", detail: fmt.Sprintf("%s: asked %s, got %s", got, asked, gotPixFmt)}}
	}
	return nil
}

// sweepEncodesOwnCodec reports whether a video with a -pix_fmt set for its own codec would be encoded in it: "copy" or
// another codec named for it, or extra args replacing the codec, leave it as it is.
func sweepEncodesOwnCodec(c sweepCase, source string) bool {
	var entries []string
	if c.VideoCodecs != "" {
		entries = []string{c.VideoCodecs}
	}
	all, pairs, err := sharedparsing.ParseCodecMappings(entries, sharedvalidation.ValidateVideoCodec)
	if err != nil {
		return false
	}
	want, settled := sweepWant(all, pairs, source)
	if !settled || (want != "" && want != source) {
		return false
	}
	return !slices.ContainsFunc(sweepVideoOverrides, func(o string) bool { return sweepHasOption(c.ExtraArgs, o) })
}

// sweepDecodeComplaint returns the first error from decoding a file that says something's wrong with it. Timestamps
// repeated by a container's rounding are left out: only the check's own null output minds, and every frame is there.
func sweepDecodeComplaint(errors string) string {
	for line := range strings.Lines(errors) {
		if !strings.Contains(line, "non monotonically increasing dts") && !strings.Contains(line, "Last message repeated") {
			return strings.TrimSpace(line)
		}
	}
	return ""
}

// firstLine returns the first line of s.
func firstLine(s string) string {
	line, _, _ := strings.Cut(s, "\n")
	return line
}

// sweepAssets writes the files the cases share: subtitles and thumbnails.
func sweepAssets(t *testing.T, dir string) {
	t.Helper()
	subtitles := "1\n00:00:00,100 --> 00:00:00,800\nA subtitle\n"
	if err := os.WriteFile(filepath.Join(dir, "subtitles.srt"), []byte(subtitles), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, ext := range sweepThumbnailExts {
		out, err := exec.Command(sharedconsts.ProgramFFmpeg, "-v", "error", "-y", "-f", "lavfi", "-i", "color=black:s=64x64", "-frames:v", "1", "-update", "1",
			filepath.Join(dir, "thumb."+ext)).CombinedOutput()
		if err != nil {
			t.Fatalf("making the %s thumbnail: %v\n%s", ext, err, out)
		}
	}
}

// sweepGroup is the cases found with one issue.
type sweepGroup struct {
	key      string
	class    string
	count    int
	note     string
	command  string
	examples []sweepCase
	routes   map[string]int // How many went from each input to output extension.
	pairs    map[string]int // How many asked for each output extension, video and audio codec.
	extras   map[string]int // How many had each extra FFmpeg args.
	smallest sweepCase
}

// sweepTally counts how cases ended.
type sweepTally struct {
	outcomes map[string]int
	refusals map[string]int
	sources  map[string]int // Why sources couldn't be made.
	routes   map[string]int // The input to output extensions that came out fine.
	groups   map[string]*sweepGroup
}

// add counts a case's ending.
func (t *sweepTally) add(c sweepCase, r sweepResult) {
	t.outcomes[r.outcome]++
	switch r.outcome {
	case sweepUnmakeable:
		t.sources[r.reason]++
	case sweepRefusedFlags, sweepRefusedRun:
		t.refusals[r.outcome+": "+r.reason]++
	case sweepFine:
		t.routes[c.Ext+" to "+cmpOr(c.OutputExt, "same")]++
	}
	for _, issue := range r.issues {
		g := t.groups[issue.key()]
		if g == nil {
			g = &sweepGroup{key: issue.key(), class: issue.class, note: issue.note, command: r.command, routes: map[string]int{}, pairs: map[string]int{}, extras: map[string]int{}, smallest: c}
			t.groups[g.key] = g
		}
		g.count++
		g.routes[c.Ext+" to "+cmpOr(c.OutputExt, "same")]++
		g.pairs[cmpOr(c.OutputExt, "same")+" "+cmpOr(c.VideoCodecs, "-")+" "+cmpOr(c.AudioCodecs, "-")]++
		g.extras[cmpOr(c.ExtraArgs, "-")]++
		if len(g.examples) < sweepExamples {
			g.examples = append(g.examples, c)
		}
	}
}

// cmpOr returns s, or the fallback if it's blank.
func cmpOr(s, fallback string) string {
	if s == "" {
		return fallback
	}
	return s
}

// shrink returns the smallest case still showing an issue, and that case's result.
func (sw *sweep) shrink(c sweepCase, key string) (sweepCase, sweepResult) {
	best := sw.run(c)
	runs := 1
	for runs < sweepMaxShrink {
		smaller := false
		for _, candidate := range c.simpler() {
			if runs >= sweepMaxShrink {
				break
			}
			runs++
			result := sw.run(candidate)
			if result.has(key) {
				c = candidate
				best = result
				smaller = true
				break
			}
		}
		if !smaller {
			break
		}
	}
	return c, best
}

// sweepTop returns a count map as lines, largest first, at most n of them.
func sweepTop(counts map[string]int, n int) []string {
	names := slices.SortedFunc(maps.Keys(counts), func(a, b string) int {
		if counts[a] != counts[b] {
			return counts[b] - counts[a]
		}
		return strings.Compare(a, b)
	})
	var lines []string
	for i, name := range names {
		if i == n {
			lines = append(lines, fmt.Sprintf("... and %d more", len(names)-n))
			break
		}
		lines = append(lines, fmt.Sprintf("%6d  %s", counts[name], name))
	}
	return lines
}

// report writes up what the sweep found.
func (sw *sweep) report(version string, seed uint64, runs int, tally *sweepTally, groups []*sweepGroup) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s\nSeed %d, %d cases.\n\n", version, seed, runs)
	for _, line := range sweepTop(tally.outcomes, 10) {
		fmt.Fprintln(&b, line)
	}

	// What it found, worst first.
	fmt.Fprintf(&b, "\n%d kinds of problem:\n", len(groups))
	for i, g := range groups {
		fmt.Fprintf(&b, "\n%d. %s  (%d cases)\n", i+1, g.key, g.count)
		if g.note != "" {
			fmt.Fprintf(&b, "    note   %s\n", g.note)
		}
		fmt.Fprintf(&b, "    %s\n", g.smallest.describe(sw.assets))
		if g.command != "" {
			fmt.Fprintf(&b, "    ffmpeg %s\n", g.command)
		}
		fmt.Fprintf(&b, "    make   ffmpeg %s\n", strings.Join(g.smallest.makeArgs("<assets>", "<source>"), " "))
		fmt.Fprintf(&b, "    routes %s\n", strings.Join(sweepTop(g.routes, 40), ", "))
		fmt.Fprintf(&b, "    asked  %s\n", strings.Join(sweepTop(g.pairs, 40), ", "))
		fmt.Fprintf(&b, "    extras %s\n", strings.Join(sweepTop(g.extras, 40), ", "))
		for _, example := range g.examples {
			fmt.Fprintf(&b, "    case   %s\n", strings.ReplaceAll(example.describe(sw.assets), "\n    flags ", " |"))
		}
	}

	// What it didn't reach, to judge how much it covered.
	fmt.Fprintf(&b, "\nRefusals:\n")
	for _, line := range sweepTop(tally.refusals, 40) {
		fmt.Fprintln(&b, line)
	}
	fmt.Fprintf(&b, "\nSources that couldn't be made:\n")
	for _, line := range sweepTop(tally.sources, 25) {
		fmt.Fprintln(&b, line)
	}
	fmt.Fprintf(&b, "\nRoutes that came out fine: %d\n", len(tally.routes))
	for _, line := range sweepTop(tally.routes, 15) {
		fmt.Fprintln(&b, line)
	}
	return b.String()
}

// sweepSeverity ranks a class of problem, lowest worst: unusable or lost media, then damaged media, then settings not
// honoured, then odd but playable output, and last failures, which at least leave the original and say so.
func sweepSeverity(class string) int {
	switch class {
	case "hung", "panicked", "original-lost", "unreadable", "undecodable", "video-lost", "no-video-frames", "changed-video-after-failure":
		return 0
	case "audio-streams-lost", "no-audio-frames", "audio-silent", "picture-not-black", "audio-wrong-length", "video-wrong-length", "audio-out-of-sync":
		return 1
	case "subtitles-lost", "thumbnail-lost", "thumbnail-not-stripped", "wrong-video-codec", "wrong-audio-codec", "wrong-pixel-format", "wrong-size", "wrong-extension":
		return 2
	case "ffmpeg-failed":
		return 4
	}
	return 3
}

// sweepEnvInt reads a whole number setting, or the fallback if it isn't set.
func sweepEnvInt(t *testing.T, name string, fallback int) int {
	t.Helper()
	value := os.Getenv(name)
	if value == "" {
		return fallback
	}
	n, err := strconv.Atoi(value)
	if err != nil {
		t.Fatalf("%s=%q: %v", name, value, err)
	}
	return n
}

// TestTranscodeSweep runs Metarr on tiny black videos with random inputs, codecs and arguments, and checks a run it
// finishes leaves a video that decodes in full, with its streams, in the codecs asked, and a run it can't finish was
// refused rather than failing inside FFmpeg. Run it where the real FFmpeg is, as FFmpeg's versions differ:
//
//	CGO_ENABLED=0 go test -c -o /tmp/sweep/metarr.test ./cmd/metarr
//	docker run --rm --name metarr-sweep --memory 12g --network none --tmpfs /tmp:rw,size=4g,mode=1777 \
//	  -v /tmp/sweep:/sweep --user "$(id -u):$(id -g)" -e METARR_SWEEP_RUNS=20000 -e METARR_SWEEP_REPORT=/sweep/report.txt \
//	  --entrypoint /sweep/metarr.test tubarr-ffmpeg -test.run '^TestTranscodeSweep$' -test.timeout 0
func TestTranscodeSweep(t *testing.T) {
	runs := sweepEnvInt(t, sweepRunsEnv, 0)
	if runs == 0 {
		t.Skipf("set %s to run", sweepRunsEnv)
	}
	seed := uint64(sweepEnvInt(t, sweepSeedEnv, 1))
	workers := sweepEnvInt(t, sweepWorkersEnv, runtime.NumCPU())
	if clip := os.Getenv(sweepClipEnv); clip != "" {
		sweepClipSeconds = clip
	}

	versionOut, err := exec.Command(sharedconsts.ProgramFFmpeg, "-hide_banner", "-version").Output()
	if err != nil {
		t.Skipf("FFmpeg isn't installed: %v", err)
	}
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}

	// Metarr posts its logs to Tubarr, so they go here instead of to a Tubarr running on this machine.
	logs := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
	}))
	t.Cleanup(logs.Close)

	sw := &sweep{t: t, root: t.TempDir(), assets: t.TempDir(), exe: exe, logURL: logs.URL}
	sweepAssets(t, sw.assets)

	// Run the cases.
	type finished struct {
		c sweepCase
		r sweepResult
	}
	jobs := make(chan sweepCase)
	results := make(chan finished)
	var wg sync.WaitGroup
	for range workers {
		wg.Go(func() {
			for c := range jobs {
				results <- finished{c, sw.run(c)}
			}
		})
	}
	tableCases := append(sweepTableCases(), sweepCopyCases()...)
	runs += len(tableCases)
	go func() {
		for _, c := range tableCases {
			jobs <- c
		}
		for i := range runs - len(tableCases) {
			jobs <- randomSweepCase(rand.New(rand.NewPCG(seed, uint64(i))))
		}
		close(jobs)
		wg.Wait()
		close(results)
	}()
	tally := &sweepTally{outcomes: map[string]int{}, refusals: map[string]int{}, sources: map[string]int{}, routes: map[string]int{}, groups: map[string]*sweepGroup{}}
	done := 0
	started := time.Now()
	for f := range results {
		tally.add(f.c, f.r)
		done++
		if done%500 == 0 {
			fmt.Fprintf(os.Stderr, "%d of %d cases in %v: %v, %d kinds of problem\n", done, runs, time.Since(started).Round(time.Second), tally.outcomes, len(tally.groups))
		}
	}

	// Shrink each kind of problem to the smallest case showing it.
	groups := slices.Collect(maps.Values(tally.groups))
	shrinkJobs := make(chan *sweepGroup)
	var shrinkers sync.WaitGroup
	for range workers {
		shrinkers.Go(func() {
			for g := range shrinkJobs {
				// Each try at a hung case takes the whole timeout, so its flags are reported as they were.
				if g.class == "hung" {
					continue
				}
				smallest, result := sw.shrink(g.smallest, g.key)
				g.smallest = smallest
				if result.command != "" {
					g.command = result.command
				}
				for _, issue := range result.issues {
					if issue.key() == g.key && issue.note != "" {
						g.note = issue.note
					}
				}
			}
		})
	}
	for _, g := range groups {
		shrinkJobs <- g
	}
	close(shrinkJobs)
	shrinkers.Wait()

	// Worst first, then the most common.
	slices.SortFunc(groups, func(a, b *sweepGroup) int {
		switch {
		case sweepSeverity(a.class) != sweepSeverity(b.class):
			return sweepSeverity(a.class) - sweepSeverity(b.class)
		case a.count != b.count:
			return b.count - a.count
		}
		return strings.Compare(a.key, b.key)
	})

	// Report.
	report := sw.report(firstLine(string(versionOut)), seed, runs, tally, groups)
	t.Log(report)
	if path := os.Getenv(sweepReportEnv); path != "" {
		if err := os.WriteFile(path, []byte(report), 0o644); err != nil {
			t.Error(err)
		}
	}
	if len(groups) > 0 {
		t.Errorf("found %d kinds of problem", len(groups))
	}
}

// ******************************** Video only ****************************************************************************************

// TestVideoWithoutMetafile runs Metarr on a video given with no metafile, which keeps its own tags and still goes through
// the video settings.
func TestVideoWithoutMetafile(t *testing.T) {
	if _, err := exec.LookPath(sharedconsts.ProgramFFmpeg); err != nil {
		t.Skip("FFmpeg isn't installed")
	}
	const title = "Kept Title"

	for _, c := range []struct {
		name        string
		ext         string
		thumbnail   bool
		flags       []string
		wantFFmpeg  bool
		wantName    string // The video's name afterwards, in the output directory if one is set.
		wantOutDir  bool
		wantNoThumb bool
	}{
		{name: "nothing to change", ext: sharedconsts.ExtMKV, flags: []string{"--" + keys.WriteNFO}, wantName: "video.mkv"},
		{name: "new container keeps its tags", ext: sharedconsts.ExtMKV, flags: []string{"--" + keys.OutputFiletype + "=mp4"}, wantFFmpeg: true, wantName: "video.mp4"},
		{name: "thumbnail stripped", ext: sharedconsts.ExtMP4, thumbnail: true, flags: []string{"--" + keys.StripThumbnails}, wantFFmpeg: true, wantName: "video.mp4", wantNoThumb: true},
		{name: "thumbnail kept", ext: sharedconsts.ExtMP4, thumbnail: true, wantName: "video.mp4"},
		{
			name: "renamed, moved and purge passed over", ext: sharedconsts.ExtMKV,
			flags:    []string{"--" + keys.FilenameOpsInput + "=prefix:new ", "--" + keys.MetaPurge + "=all"},
			wantName: "new video.mkv", wantOutDir: true,
		},
		{name: "template keeps the name", ext: sharedconsts.ExtMKV, flags: []string{"--" + keys.FilenameOpsInput + "=prefix:{{title}} "}, wantName: "video.mkv", wantOutDir: true},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			video := makeTaggedVideo(t, dir, c.ext, title, c.thumbnail)

			flags := append([]string{"--" + keys.VideoFiles + "=" + video}, c.flags...)
			wantPath := filepath.Join(dir, c.wantName)
			if c.wantOutDir {
				out := filepath.Join(dir, "out")
				flags = append(flags, "--"+keys.OutputDirectory+"="+out)
				wantPath = filepath.Join(out, c.wantName)
			}
			run := runReencodeMetarr(t, flags)

			// FFmpeg run or not.
			if got := run.command != ""; got != c.wantFFmpeg {
				t.Errorf("FFmpeg ran = %t, want %t\ncommand: %s", got, c.wantFFmpeg, run.command)
			}

			// Where the video is, as reported, with no metafile reported.
			if !strings.Contains(run.output, "final video path: "+wantPath+"\n") {
				t.Errorf("final video path not %q in:\n%s", wantPath, run.output)
			}
			if !strings.Contains(run.output, "final json path: \n") {
				t.Errorf("a final metafile path was reported:\n%s", run.output)
			}

			// Its own tags and thumbnail.
			if got := probeTitle(t, wantPath); got != title {
				t.Errorf("title = %q, want %q", got, title)
			}
			if c.thumbnail {
				if got := hasThumbnail(t, wantPath); got == c.wantNoThumb {
					t.Errorf("has thumbnail = %t, want %t", got, !c.wantNoThumb)
				}
			}

			// No NFO is made with no metadata to fill it.
			if nfos, _ := filepath.Glob(filepath.Join(filepath.Dir(wantPath), "*"+sharedconsts.MExtNFO)); len(nfos) > 0 {
				t.Errorf("NFO written: %v", nfos)
			}
		})
	}
}

// makeTaggedVideo writes a short video with a title tag, and a thumbnail if asked.
func makeTaggedVideo(t *testing.T, dir, ext, title string, thumbnail bool) string {
	t.Helper()
	video := filepath.Join(dir, "video"+ext)
	args := []string{"-v", "error", "-y", "-f", "lavfi", "-i", "testsrc2=s=64x64:d=0.2:r=10"}
	if thumbnail {
		thumb := filepath.Join(dir, "thumb.jpg")
		if out, err := exec.Command(sharedconsts.ProgramFFmpeg, "-v", "error", "-y", "-f", "lavfi", "-i", "color=red:s=32x32", "-frames:v", "1", thumb).CombinedOutput(); err != nil {
			t.Fatalf("making thumbnail: %v\n%s", err, out)
		}
		args = append(args, "-i", thumb, "-map", "0", "-map", "1", "-c:v:1", "copy", "-disposition:v:1", "attached_pic")
	}
	args = append(args, "-c:v:0", "mpeg4", "-metadata", "title="+title, video)
	if out, err := exec.Command(sharedconsts.ProgramFFmpeg, args...).CombinedOutput(); err != nil {
		t.Fatalf("making video: %v\n%s", err, out)
	}
	return video
}

// probeTitle returns a video's title tag.
func probeTitle(t *testing.T, path string) string {
	t.Helper()
	out, err := exec.Command(sharedconsts.ProgramFFprobe, "-v", "error", "-show_entries", "format_tags=title", "-of", "default=nw=1:nk=1", path).Output()
	if err != nil {
		t.Fatalf("probing %s: %v", path, err)
	}
	return strings.TrimSpace(string(out))
}

// hasThumbnail reports whether a video has an attached picture.
func hasThumbnail(t *testing.T, path string) bool {
	t.Helper()
	out, err := exec.Command(sharedconsts.ProgramFFprobe, "-v", "error", "-show_entries", "stream_disposition=attached_pic", "-of", "csv=p=0", path).Output()
	if err != nil {
		t.Fatalf("probing %s: %v", path, err)
	}
	for _, line := range strings.Fields(string(out)) {
		if line == "1" {
			return true
		}
	}
	return false
}
