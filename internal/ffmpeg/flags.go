package ffmpeg

import (
	"metarr/internal/domain/consts"

	"github.com/TubarrApp/gocommon/sharedconsts"
)

// unsafeHardwareEncode contains codecs unsafe for transcoding on this GPU acceleration type.
var unsafeHardwareEncode = map[string]map[string]struct{}{
	sharedconsts.AccelTypeCuda:  {"mjpeg": {}},
	sharedconsts.AccelTypeVAAPI: {sharedconsts.VCodecVP8: {}, sharedconsts.VCodecVP9: {}, sharedconsts.VCodecAV1: {}},
	sharedconsts.AccelTypeQSV:   {sharedconsts.VCodecVP8: {}, sharedconsts.VCodecVP9: {}},
}

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
