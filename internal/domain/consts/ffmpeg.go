package consts

import "github.com/TubarrApp/gocommon/sharedconsts"

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
	FFmpegCRF                 = "-crf"
	FFmpegBA                  = "-b:a"
	FFmpegAR                  = "-ar"
	FFmpegStrict              = "-strict"
	FFmpegStrictExperimental  = "experimental"
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

// compatibility for HW acceleration types.
var (
	VAAPICompatibility = []string{"format=nv12,hwupload"}
	CudaCompatibility  = []string{"hwdownload,format=nv12,hwupload_cuda"}
)
