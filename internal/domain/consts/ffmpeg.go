package consts

import (
	"github.com/TubarrApp/gocommon/sharedconsts"
)

// Video codecs.
const (
	FFVCodecKeyCopy  = "copy"
	FFVCodecKeyAV1   = "libsvtav1"
	FFVCodecKeyH264  = "libx264"
	FFVCodecKeyH265  = "libx265"
	FFVCodecKeyMPEG2 = "mpeg2video"
	FFVCodecKeyVP8   = "libvpx"
	FFVCodecKeyVP9   = "libvpx-vp9"
)

// VCodecToFFVCodec maps desired codecs to the recommended FFmpeg codec.
var VCodecToFFVCodec = map[string]string{
	sharedconsts.VCodecCopy:  FFVCodecKeyCopy,
	sharedconsts.VCodecAV1:   FFVCodecKeyAV1,
	sharedconsts.VCodecH264:  FFVCodecKeyH264,
	sharedconsts.VCodecHEVC:  FFVCodecKeyH265,
	sharedconsts.VCodecMPEG2: FFVCodecKeyMPEG2,
	sharedconsts.VCodecVP8:   FFVCodecKeyVP8,
	sharedconsts.VCodecVP9:   FFVCodecKeyVP9,
}

// Audio codecs.
const (
	FFACodecKeyCopy   = "copy"
	FFACodecKeyAAC    = "aac"
	FFACodecKeyAC3    = "ac3"
	FFACodecKeyALAC   = "alac"
	FFACodecKeyDTS    = "dca"
	FFACodecKeyEAC3   = "eac3"
	FFACodecKeyFLAC   = "flac"
	FFACodecKeyMP2    = "mp2"
	FFACodecKeyMP3    = "libmp3lame"
	FFACodecKeyOpus   = "libopus" // FFmpeg's own Opus and Vorbis encoders are experimental, so use the libraries'.
	FFACodecKeyPCM    = "pcm_s16le"
	FFACodecKeyTrueHD = "truehd"
	FFACodecKeyVorbis = "libvorbis"
)

// ACodecToFFACodec maps desired codecs to the recommended FFmpeg codec.
var ACodecToFFACodec = map[string]string{
	sharedconsts.ACodecCopy:   FFACodecKeyCopy,
	sharedconsts.ACodecAAC:    FFACodecKeyAAC,
	sharedconsts.ACodecAC3:    FFACodecKeyAC3,
	sharedconsts.ACodecALAC:   FFACodecKeyALAC,
	sharedconsts.ACodecDTS:    FFACodecKeyDTS,
	sharedconsts.ACodecEAC3:   FFACodecKeyEAC3,
	sharedconsts.ACodecFLAC:   FFACodecKeyFLAC,
	sharedconsts.ACodecMP2:    FFACodecKeyMP2,
	sharedconsts.ACodecMP3:    FFACodecKeyMP3,
	sharedconsts.ACodecOpus:   FFACodecKeyOpus,
	sharedconsts.ACodecPCM:    FFACodecKeyPCM,
	sharedconsts.ACodecTrueHD: FFACodecKeyTrueHD,
	sharedconsts.ACodecVorbis: FFACodecKeyVorbis,
	sharedconsts.ACodecWAV:    FFACodecKeyPCM,
}

// Accel flags.
const (
	AccelFlagNvenc = "nvenc"
)

// Command constants.
const (
	FFmpegCA                  = "-c:a"
	FFmpegCD                  = "-c:d"
	FFmpegCS                  = "-c:s"
	FFmpegCT                  = "-c:t"
	FFmpegCV0                 = "-c:v:0"
	FFmpegHWAccel             = "-hwaccel"
	FFmpegHWAccelDevice       = "-hwaccel_device"
	FFmpegHWAccelOutputFormat = "-hwaccel_output_format"
	FFmpegDeviceQSV           = "-qsv_device"
	FFmpegDeviceVAAPI         = "-vaapi_device"
	FFmpegVF                  = "-vf"
	FFmpegFilter              = "-filter:v:0"
	FFmpegPixFmt              = "-pix_fmt:v:0"
	FFmpegBitrate             = "-b:v:0"
	FFmpegPass                = "-pass:v:0"
	FFmpegPassLogFile         = "-passlogfile:v:0"
	FFmpegCRF                 = "-crf"
	FFmpegBA                  = "-b:a"
	FFmpegAR                  = "-ar"
	FFmpegAF                  = "-af"
	FFmpegMetadata            = "-metadata"
	FFmpegFormat              = "-f"
)

// Muxers named for extensions FFmpeg can't pick one for, and for passes writing no file.
const (
	FFmpegMuxerOGV  = "ogv"
	FFmpegMuxerRM   = "rm"
	FFmpegMuxerNull = "null"
)

// FFmpegStrictExperimental lets FFmpeg use features it counts as experimental.
var FFmpegStrictExperimental = []string{"-strict", "experimental"}

// FFmpegGeneratePTS goes before an input, filling in timestamps some containers leave out. It changes nothing in files that have them.
var FFmpegGeneratePTS = []string{"-fflags", "+genpts"}

// OpusLayoutFilter turns other channel layouts into the nearest the Opus encoder takes (e.g. "5.1(side)", which AC-3 uses, into "5.1").
const OpusLayoutFilter = "aformat=channel_layouts=7.1|6.1|5.1|5.0|quad|3.0|stereo|mono"

// EvenSizeFilter cuts the last row and column off a video where its size is odd, keeping every other pixel as it was.
// MinSideToCut is the shortest side that's done for, as a row is too much of a smaller picture to lose.
const (
	EvenSizeFilter = "crop=trunc(iw/2)*2:trunc(ih/2)*2"
	MinSideToCut   = 32
)

// Audio rate values.
const (
	AudioRate48khz = "48000"
)

// Audio rate for audio type.
var (
	AudioRateForType = map[string]string{
		sharedconsts.ACodecAC3:  AudioRate48khz,
		sharedconsts.ACodecDTS:  AudioRate48khz,
		sharedconsts.ACodecEAC3: AudioRate48khz,
	}
)

// PixFmtEquivalents lists pixel formats with the same bit depth and chroma, for encoders that don't take the one asked.
var PixFmtEquivalents = map[string][]string{
	"yuv420p":     {"nv12"},
	"yuv420p10le": {"p010le"},
	"yuv420p12le": {"p012le"},
	"yuv422p":     {"nv16", "yuyv422"},
	"yuv422p10le": {"p210le", "y210le"},
	"yuv444p":     {"vuyx"},
	"yuv444p10le": {"yuv444p16le", "xv30le"},
	"nv12":        {"yuv420p"},
	"p010le":      {"yuv420p10le"},
	"p012le":      {"yuv420p12le"},
	"nv16":        {"yuv422p"},
	"p210le":      {"yuv422p10le"},
	"y210le":      {"yuv422p10le"},
}

// VAAPIUploadFormats are the pixel formats VAAPI uploads frames to the GPU in, for each format asked.
var VAAPIUploadFormats = map[string]string{
	"yuv420p":     "nv12",
	"nv12":        "nv12",
	"yuv420p10le": "p010",
	"p010le":      "p010",
}

// VAAPI filter uploading frames to the GPU in a pixel format, 8-bit 4:2:0 unless asked otherwise.
const (
	VAAPIUploadFilter = "format=%s,hwupload"
	VAAPIUploadPixFmt = "nv12"
)

// compatibility for HW acceleration types.
var (
	CudaCompatibility = []string{"hwdownload,format=nv12,hwupload_cuda"}
)
