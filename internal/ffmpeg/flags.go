package ffmpeg

import (
	"metarr/internal/domain/consts"

	"github.com/TubarrApp/gocommon/sharedconsts"
)

// formatPreset holds a pre-calculated set of ffmpeg flags.
type formatPreset struct {
	flags map[string]string
}

var unsafeHardwareEncode = map[string]map[string]bool{
	sharedconsts.AccelTypeNvidia: {"mjpeg": true}, // hypothetical crashes.
	sharedconsts.AccelTypeVAAPI:  {"vp8": true, "vp9": true, "av1": true},
	sharedconsts.AccelTypeIntel:  {"vp8": true, "vp9": true, "av1": true},
}

// Presets for transcoding.
var (
	// Direct copy preset.
	copyPreset = formatPreset{
		flags: map[string]string{
			consts.FFmpegCV0: "copy",
			consts.FFmpegCA:  "copy",
			consts.FFmpegCS:  "copy",
			consts.FFmpegCD:  "copy",
			consts.FFmpegCT:  "copy",
		},
	}

	// Standard h264 conversion.
	h264Preset = formatPreset{
		flags: map[string]string{
			consts.FFmpegCV0: "libx264",
			consts.FFmpegCA:  "copy",
			consts.FFmpegCS:  "copy",
			consts.FFmpegCD:  "copy",
			consts.FFmpegCT:  "copy",
		},
	}

	// Video copy with AAC audio.
	videoCopyAACPreset = formatPreset{
		flags: map[string]string{
			consts.FFmpegCV0: "copy",
			consts.FFmpegCA:  "aac",
			consts.FFmpegCS:  "copy",
			consts.FFmpegCD:  "copy",
			consts.FFmpegCT:  "copy",
		},
	}

	// Full webm conversion preset.
	webmPreset = formatPreset{
		flags: map[string]string{
			consts.FFmpegCV0: "libx264",
			consts.FFmpegCA:  "copy",
			consts.FFmpegCS:  "copy",
			consts.FFmpegCD:  "copy",
			consts.FFmpegCT:  "copy",
		},
	}
)

var formatMap = map[string]map[string]formatPreset{
	consts.ExtAVI: {
		consts.ExtAVI:  copyPreset,
		consts.ExtMP4:  videoCopyAACPreset,
		consts.ExtM4V:  videoCopyAACPreset,
		consts.ExtMOV:  videoCopyAACPreset,
		consts.ExtRM:   webmPreset,
		consts.ExtRMVB: webmPreset,
		"*":            h264Preset, // default preset.
	},
	consts.ExtMP4: {
		consts.ExtMP4:  copyPreset,
		consts.ExtMKV:  videoCopyAACPreset,
		consts.ExtWEBM: webmPreset,
		"*":            h264Preset, // default preset.
	},
	consts.ExtMKV: {
		consts.ExtMKV: copyPreset,
		consts.ExtMP4: videoCopyAACPreset,
		consts.ExtM4V: videoCopyAACPreset,
		"*":           h264Preset, // default preset.
	},
	consts.ExtWEBM: {
		consts.ExtWEBM: copyPreset,
		consts.ExtMP4:  copyPreset,
		"*":            webmPreset, // default preset.
	},
}
