package ffmpeg

import (
	"metarr/internal/domain/consts"

	"github.com/TubarrApp/gocommon/sharedconsts"
)

var unsafeHardwareEncode = map[string]map[string]bool{
	sharedconsts.AccelTypeNvidia: {"mjpeg": true}, // hypothetical crashes.
	sharedconsts.AccelTypeVAAPI:  {"vp8": true, "vp9": true, "av1": true},
	sharedconsts.AccelTypeIntel:  {"vp8": true, "vp9": true, "av1": true},
}

// Presets for transcoding.
var (
	// Direct copy preset.
	copyPreset = map[string]string{
		consts.FFmpegCV0: "copy",
		consts.FFmpegCA:  "copy",
		consts.FFmpegCS:  "copy",
		consts.FFmpegCD:  "copy",
		consts.FFmpegCT:  "copy",
	}
)
