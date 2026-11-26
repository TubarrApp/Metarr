package ffmpeg

import (
	"context"
	"metarr/internal/domain/logger"
	"metarr/internal/ffprobe"
	"strconv"

	"github.com/TubarrApp/gocommon/sharedmodels"
)

// videoBitrate returns the bitrate to encode a video at, lowered to the source's own if that's less.
func videoBitrate(ctx context.Context, inputFile, asked string) int64 {
	bitrate, err := strconv.ParseInt(asked, 10, 64)
	if err != nil {
		logger.Pl.DevE("Bitrate %q passed validation but can't be read: %v", asked, err)
	}

	// No target has nothing to lower.
	if bitrate <= 0 {
		return bitrate
	}

	// A higher bitrate than the source's makes a bigger file, not a better one.
	if sourceBitrate := ffprobe.ProbeVideoBitrate(ctx, inputFile); sourceBitrate > 0 && bitrate > sourceBitrate {
		logger.Pl.I("Lowering the target bitrate for %q from %d to its own %d bits per second.", inputFile, bitrate, sourceBitrate)
		bitrate = sourceBitrate
	}
	return bitrate
}

// argsForResolution returns the args of the line covering a video's resolution tier, else the plain "codec:" line's.
func argsForResolution(byResolution map[int]sharedmodels.EncoderArgs, resolution int) sharedmodels.EncoderArgs {
	if args, ok := byResolution[resolution]; ok {
		return args
	}
	return byResolution[0]
}
