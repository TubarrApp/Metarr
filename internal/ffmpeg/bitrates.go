package ffmpeg

import (
	"context"
	"encoding/json"
	"maps"
	"metarr/internal/domain/logger"
	"os/exec"
	"slices"
	"strconv"
)

// videoBitrate returns the bitrate to encode a video at, from bitrates by resolution, lowered to the source's own if that's less.
func videoBitrate(ctx context.Context, inputFile string, bitrates map[int]int64) int64 {
	shorterSide, sourceBitrate := probeVideoSize(ctx, inputFile)
	bitrate := bitrateFor(bitrates, shorterSide)

	// A higher bitrate than the source's makes a bigger file, not a better one.
	if bitrate > 0 && sourceBitrate > 0 && bitrate > sourceBitrate {
		logger.Pl.I("Lowering the target bitrate for %q from %d to its own %d bits per second.", inputFile, bitrate, sourceBitrate)
		bitrate = sourceBitrate
	}
	return bitrate
}

// bitrateFor returns the bitrate for the largest resolution at or below a video's shorter side, else the smallest resolution's.
func bitrateFor(bitrates map[int]int64, shorterSide int) int64 {
	resolutions := slices.Sorted(maps.Keys(bitrates))
	if len(resolutions) == 0 {
		return 0
	}
	bitrate := bitrates[resolutions[0]]
	for _, resolution := range resolutions {
		if resolution <= shorterSide {
			bitrate = bitrates[resolution]
		}
	}
	return bitrate
}

// probeVideoSize returns a video's shorter side in pixels and its bitrate, the whole file's when the stream has none (as in MKV and WebM).
func probeVideoSize(ctx context.Context, inputFile string) (shorterSide int, bitrate int64) {
	out, err := exec.CommandContext(ctx, "ffprobe",
		"-v", "error",
		"-select_streams", "v:0",
		"-show_entries", "stream=width,height,bit_rate:format=bit_rate",
		"-of", "json",
		inputFile,
	).Output()
	if err != nil {
		logger.Pl.W("Could not read the size of %q, so the smallest -b:v resolution is used: %v", inputFile, err)
		return 0, 0
	}

	var probe struct {
		Streams []struct {
			Width   int    `json:"width"`
			Height  int    `json:"height"`
			BitRate string `json:"bit_rate"`
		} `json:"streams"`
		Format struct {
			BitRate string `json:"bit_rate"`
		} `json:"format"`
	}
	if err := json.Unmarshal(out, &probe); err != nil {
		logger.Pl.W("Could not read the size of %q, so the smallest -b:v resolution is used: %v", inputFile, err)
		return 0, 0
	}

	// The first video stream's size and bitrate, else the file's bitrate.
	if len(probe.Streams) > 0 {
		shorterSide = min(probe.Streams[0].Width, probe.Streams[0].Height)
		bitrate, _ = strconv.ParseInt(probe.Streams[0].BitRate, 10, 64)
	}
	if bitrate <= 0 {
		bitrate, _ = strconv.ParseInt(probe.Format.BitRate, 10, 64)
	}
	return shorterSide, bitrate
}
