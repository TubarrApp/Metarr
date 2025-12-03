package ffmpeg

import (
	"metarr/internal/domain/consts"

	"github.com/TubarrApp/gocommon/sharedconsts"
)

// Presets for transcoding.
var (
	// Direct copy preset.
	copyPreset = map[string]string{
		consts.FFmpegCV0: sharedconsts.VCodecCopy,
		consts.FFmpegCA:  sharedconsts.VCodecCopy,
		consts.FFmpegCS:  sharedconsts.VCodecCopy,
		consts.FFmpegCD:  sharedconsts.VCodecCopy,
		consts.FFmpegCT:  sharedconsts.VCodecCopy,
	}
)
