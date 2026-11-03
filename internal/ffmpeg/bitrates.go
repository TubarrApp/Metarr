package ffmpeg

import (
	"context"
	"maps"
	"metarr/internal/domain/logger"
	"metarr/internal/ffprobe"
	"slices"
)

// videoBitrate returns the bitrate to encode a video at, from bitrates by resolution, lowered to the source's own if that's less.
func videoBitrate(ctx context.Context, inputFile string, bitrates map[int]int64) int64 {
	shorterSide, sourceBitrate := ffprobe.ProbeVideoSize(ctx, inputFile)
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
