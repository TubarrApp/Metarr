package ffmpeg

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"metarr/internal/domain/consts"
	"metarr/internal/domain/logger"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/TubarrApp/gocommon/sharedconsts"
)

// ffmpegLogAddress matches the memory address FFmpeg logs beside a component's name.
var ffmpegLogAddress = regexp.MustCompile(` @ 0x[0-9a-f]+`)

// hardwareProbeLines mark errors from -hwaccel auto trying hardware that isn't there, which FFmpeg moves on from.
var hardwareProbeLines = []string{"AVHWDeviceContext", "CUDA", "Device creation failed"}

// runPasses runs an FFmpeg command once per encoding pass, returning the failing pass's output with its error.
func runPasses(ctx context.Context, args []string, passes int, durationUS int64, inputFile string) (stderr string, err error) {
	if passes <= 1 {
		return runFFmpeg(ctx, args, durationUS, 1, 1, inputFile)
	}

	// Each video's pass stats go in a folder of their own, so videos encoded at once don't share them.
	statsDir, err := os.MkdirTemp("", "metarr-passes-")
	if err != nil {
		return "", fmt.Errorf("failed to make a folder for pass stats: %w", err)
	}
	defer func() {
		if err := os.RemoveAll(statsDir); err != nil {
			logger.Pl.W("Could not remove pass stats folder %q: %v", statsDir, err)
		}
	}()
	stats := filepath.Join(statsDir, "stats")

	// Earlier passes only write stats, the final pass writes the output.
	output := args[len(args)-1]
	for i := 1; i <= passes; i++ {
		passArgs := append(slices.Clone(args[:len(args)-1]), consts.FFmpegPass, strconv.Itoa(ffmpegPass(i, passes)), consts.FFmpegPassLogFile, stats)
		if i < passes {
			passArgs = append(passArgs, consts.FFmpegFormat, consts.FFmpegMuxerNull, os.DevNull)
		} else {
			passArgs = append(passArgs, output)
		}
		if stderr, err := runFFmpeg(ctx, passArgs, durationUS, i, passes, inputFile); err != nil {
			return stderr, fmt.Errorf("pass %d of %d: %w", i, passes, err)
		}
	}
	return "", nil
}

// ffmpegPass returns FFmpeg's -pass number for pass i of passes: 1 writes the stats, 3 refines them and 2 makes the output.
func ffmpegPass(i, passes int) int {
	switch i {
	case 1:
		return 1
	case passes:
		return 2
	}
	return 3
}

// runFFmpeg runs FFmpeg, printing its progress as the given pass, and returns its output if it fails.
func runFFmpeg(ctx context.Context, args []string, durationUS int64, pass, passes int, inputFile string) (string, error) {
	args = append([]string{"-progress", "pipe:1", "-nostats", "-loglevel", "level+info"}, args...) // level+ tags each line, so ffmpegFailure can find the errors.
	command := exec.CommandContext(ctx, sharedconsts.ProgramFFmpeg, args...)
	logger.Pl.I("Constructed FFmpeg command for %q:\n\n%v\n", inputFile, command.String())

	// Progress goes to stdout as progress lines, the rest of FFmpeg's output to stderr.
	var stderr bytes.Buffer
	command.Stdout = newProgressWriter(os.Stdout, durationUS, pass, passes)
	command.Stderr = io.MultiWriter(os.Stderr, &stderr)
	if err := command.Run(); err != nil {
		return stderr.String(), err
	}
	return "", nil
}

// ffmpegFailure returns the first error in FFmpeg's output, which is its cause, as later ones only follow from it ("" if there are none).
func ffmpegFailure(stderr string) string {
	for line := range strings.Lines(stderr) {
		if !strings.Contains(line, "[error]") && !strings.Contains(line, "[fatal]") {
			continue
		}
		if slices.ContainsFunc(hardwareProbeLines, func(probe string) bool { return strings.Contains(line, probe) }) {
			continue
		}
		line = ffmpegLogAddress.ReplaceAllString(line, "")
		line = strings.NewReplacer("[error] ", "", "[fatal] ", "").Replace(line)
		return strings.TrimSpace(line)
	}
	return ""
}
