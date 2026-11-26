package ffmpeg

import (
	"maps"
	"metarr/internal/domain/logger"
	"slices"
	"strconv"

	"github.com/TubarrApp/gocommon/sharedmodels"
)

// videoBitrate returns the bitrate to encode a video at, lowered to the source's own if that's less.
func videoBitrate(inputFile, asked string, sourceBitrate int64) int64 {
	bitrate, err := strconv.ParseInt(asked, 10, 64)
	if err != nil {
		logger.Pl.DevE("Bitrate %q passed validation but can't be read: %v", asked, err)
	}

	// A higher bitrate than the source's makes a bigger file, not a better one.
	if bitrate > 0 && sourceBitrate > 0 && bitrate > sourceBitrate {
		logger.Pl.I("Lowering the target bitrate for %q from %d to its own %d bits per second.", inputFile, bitrate, sourceBitrate)
		bitrate = sourceBitrate
	}
	return bitrate
}

// argsForSize returns the args for the largest size at or below a video's shorter side, else the plain "codec:" line's.
func argsForSize(sizes map[int]sharedmodels.EncoderArgs, shorterSide int) sharedmodels.EncoderArgs {
	var args sharedmodels.EncoderArgs
	for _, size := range slices.Sorted(maps.Keys(sizes)) {
		if size <= shorterSide {
			args = sizes[size]
		}
	}
	return args
}
