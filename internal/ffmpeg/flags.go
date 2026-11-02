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

	// Subtitle codecs holding images, which FFmpeg can't convert to the text ones mp4 and webm hold.
	bitmapSubtitleCodecs = map[string]struct{}{
		"dvb_subtitle":      {},
		"dvb_teletext":      {},
		"dvd_subtitle":      {},
		"hdmv_pgs_subtitle": {},
		"xsub":              {},
	}

	// FFmpeg's highest quality for a JPG.
	jpgQualityArgs = []string{"-qmin", "1", "-q:v", "1"}
)

// How a JPG thumbnail is named and described.
const (
	jpgCodec          = "mjpeg"
	thumbnailFilename = "cover.jpg" // The name Matroska players look for.
	thumbnailMimetype = "image/jpeg"
)

// Subtitle encoders, for containers that can't take another's subtitles as they are.
const (
	subtitleEncoderMovText = "mov_text"
	subtitleEncoderSRT     = "srt"
	subtitleEncoderWebVTT  = "webvtt"
)

// subtitleEncoderFor returns the encoder subtitles need going between two containers, or "" if they're copied or can't go.
func subtitleEncoderFor(inExt, outExt string) string {
	if inExt == outExt {
		return ""
	}
	switch outExt {
	case sharedconsts.ExtMP4, sharedconsts.ExtM4V, sharedconsts.ExtMOV:
		return subtitleEncoderMovText
	case sharedconsts.ExtWEBM:
		return subtitleEncoderWebVTT
	case sharedconsts.ExtMKV:
		// Matroska takes any but mov_text, which the MP4 family use.
		switch inExt {
		case sharedconsts.ExtMP4, sharedconsts.ExtM4V, sharedconsts.ExtMOV, sharedconsts.Ext3GP, sharedconsts.Ext3G2:
			return subtitleEncoderSRT
		}
	}
	return ""
}
