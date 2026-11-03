//go:build unix

package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"metarr/internal/domain/consts"
	"metarr/internal/domain/keys"

	"github.com/TubarrApp/gocommon/sharedconsts"
)

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
