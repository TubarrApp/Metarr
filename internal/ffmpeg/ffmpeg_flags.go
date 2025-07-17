package ffmpeg

import (
	"metarr/internal/domain/consts"
	"metarr/internal/utils/logging"
)

// formatPreset holds a pre-calculated set of ffmpeg flags
type formatPreset struct {
	flags []string
}

var (
	// Direct copy preset
	copyPreset = formatPreset{
		flags: consts.AVCodecCopy[:],
	}

	// Standard h264 conversion
	h264Preset = formatPreset{
		flags: concat(
			consts.VideoToH264Balanced[:],
			consts.AudioToAAC[:],
			consts.AudioBitrate[:],
		),
	}

	// Video copy with AAC audio
	videoCopyAACPreset = formatPreset{
		flags: concat(
			consts.VideoCodecCopy[:],
			consts.AudioToAAC[:],
			consts.AudioBitrate[:],
		),
	}

	// Full webm conversion preset
	webmPreset = formatPreset{
		flags: concat(
			consts.VideoToH264Balanced[:],
			consts.KeyframeBalanced[:],
			consts.AudioToAAC[:],
			consts.AudioBitrate[:],
		),
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
		"*":            h264Preset, // default preset
	},
	consts.ExtMP4: {
		consts.ExtMP4:  copyPreset,
		consts.ExtMKV:  videoCopyAACPreset,
		consts.ExtWEBM: webmPreset,
		"*":            h264Preset,
	},
	consts.ExtMKV: {
		consts.ExtMKV: copyPreset,
		consts.ExtMP4: videoCopyAACPreset,
		consts.ExtM4V: videoCopyAACPreset,
		"*":           h264Preset,
	},
	consts.ExtWEBM: {
		consts.ExtWEBM: copyPreset,
		consts.ExtMP4:  copyPreset,
		"*":            webmPreset,
	},
}

// concat combines multiple string slices into one
func concat(slices ...[]string) []string {
	var totalLen int
	for _, s := range slices {
		totalLen += len(s)
	}

	result := make([]string, 0, totalLen)
	for _, s := range slices {
		result = append(result, s...)
	}

	logging.D(2, "Made format flag array %v", result)
	return result
}
