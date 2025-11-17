package consts

import "github.com/TubarrApp/gocommon/sharedconsts"

// Audio flags.
const (
	AudioCodecCopy = "copy"
	AudioToAAC     = "aac"
	AudioToAC3     = "ac3"
	AudioToALAC    = "alac"
	AudioToDTS     = "dts"
	AudioToEAC3    = "eac3"
	AudioToFLAC    = "flac"
	AudioToMP2     = "mp2"
	AudioToMP3     = "libmp3lame"
	AudioToOpus    = "libopus"
	AudioToPCM     = "pcm_s16le"
	AudioToTrueHD  = "truehd"
	AudioToVorbis  = "libvorbis"
	AudioToWAV     = "pcm_s16le"
)

// Video codecs.
const (
	VideoCodecCopy = "copy"
	VideoToAV1     = "libsvtav1"
	VideoToH264    = "libx264"
	VideoToH265    = "libx265"
	VideoToMPEG2   = "mpeg2video"
	VideoToVP8     = "libvpx"
	VideoToVP9     = "libvpx-vp9"
)

// Accel types.
const (
	AccelTypeAMF    = "amf"
	AccelTypeAuto   = "auto"
	AccelTypeNvidia = "cuda"
	AccelTypeVAAPI  = "vaapi"
	AccelTypeIntel  = "qsv"
)

// Accel flags.
const (
	AccelFlagNvenc = "nvenc"
)

// Command constants
const (
	FFmpegCA                  = "-c:a"
	FFmpegCD                  = "-c:d"
	FFmpegCS                  = "-c:s"
	FFmpegCT                  = "-c:t"
	FFmpegCV0                 = "-c:v:0"
	FFmpegHWAccel             = "-hwaccel"
	FFmpegHWAccelOutputFormat = "-hwaccel_output_format"
	FFmpegDeviceHW            = "-hwaccel_device"
	FFmpegDeviceQSV           = "-qsv_device"
	FFmpegDeviceVAAPI         = "-vaapi_device"
	FFmpegVF                  = "-vf"
	FFmpegCRF                 = "-crf"
	FFmpegBA                  = "-b:a"
	FFmpegAR                  = "-ar"
)

// Audio rate values
const (
	AudioRate48khz = "48000"
)

// Compatability for HW acceleration types.
var (
	VAAPICompatability = []string{"format=nv12|vaapi,hwupload"}
	CudaCompatability  = []string{"hwdownload,format=nv12,hwupload_cuda"}
)

// Incompatible codecs by container/extension.
//
// WARNING: This was grabbed by AI, needs human intervention!
var IncompatibleCodecsForContainer = map[string][]string{
	// ISO BMFF family
	ExtMP4: {sharedconsts.VCodecVP8, sharedconsts.VCodecVP9, sharedconsts.VCodecMPEG2}, // AV1, H.264, HEVC OK
	ExtM4V: {sharedconsts.VCodecVP8, sharedconsts.VCodecVP9, sharedconsts.VCodecMPEG2}, // same as MP4
	ExtMOV: {sharedconsts.VCodecVP8, sharedconsts.VCodecVP9, sharedconsts.VCodecAV1},   // H.264, HEVC widely OK; MPEG-2 often OK

	// WebM / Matroska
	ExtWEBM: {sharedconsts.VCodecH264, sharedconsts.VCodecHEVC, sharedconsts.VCodecMPEG2}, // WebM only supports VP8, VP9, AV1
	ExtMKV:  {},                                                                           // Matroska supports all listed codecs

	// Legacy/desktop
	ExtAVI: {sharedconsts.VCodecAV1, sharedconsts.VCodecHEVC, sharedconsts.VCodecVP8, sharedconsts.VCodecVP9, sharedconsts.VCodecH264},  // nonstandard for modern codecs
	ExtFLV: {sharedconsts.VCodecAV1, sharedconsts.VCodecHEVC, sharedconsts.VCodecMPEG2, sharedconsts.VCodecVP8, sharedconsts.VCodecVP9}, // FLV supports only H.263/VP6/H.264
	ExtF4V: {sharedconsts.VCodecAV1, sharedconsts.VCodecHEVC, sharedconsts.VCodecMPEG2, sharedconsts.VCodecVP8, sharedconsts.VCodecVP9}, // Flash MP4 variant; H.264 only

	// Mobile-era
	Ext3GP: {sharedconsts.VCodecAV1, sharedconsts.VCodecHEVC, sharedconsts.VCodecMPEG2, sharedconsts.VCodecVP8, sharedconsts.VCodecVP9}, // supports H.263, MPEG-4 Part 2, H.264
	Ext3G2: {sharedconsts.VCodecAV1, sharedconsts.VCodecHEVC, sharedconsts.VCodecMPEG2, sharedconsts.VCodecVP8, sharedconsts.VCodecVP9},

	// Ogg family
	ExtOGV: {sharedconsts.VCodecAV1, sharedconsts.VCodecH264, sharedconsts.VCodecHEVC, sharedconsts.VCodecMPEG2, sharedconsts.VCodecVP8, sharedconsts.VCodecVP9}, // OGV → Theora/Dirac
	ExtOGM: {sharedconsts.VCodecAV1, sharedconsts.VCodecH264, sharedconsts.VCodecHEVC, sharedconsts.VCodecMPEG2, sharedconsts.VCodecVP8, sharedconsts.VCodecVP9}, // obsolete; treat as incompatible

	// MPEG program/transport & derivatives
	ExtMPG:  {sharedconsts.VCodecAV1, sharedconsts.VCodecH264, sharedconsts.VCodecHEVC, sharedconsts.VCodecVP8, sharedconsts.VCodecVP9}, // only MPEG-1/2 video is valid
	ExtMPEG: {sharedconsts.VCodecAV1, sharedconsts.VCodecH264, sharedconsts.VCodecHEVC, sharedconsts.VCodecVP8, sharedconsts.VCodecVP9},
	ExtVOB:  {sharedconsts.VCodecAV1, sharedconsts.VCodecH264, sharedconsts.VCodecHEVC, sharedconsts.VCodecVP8, sharedconsts.VCodecVP9},  // DVD-Video → MPEG-2 only
	ExtTS:   {sharedconsts.VCodecAV1, sharedconsts.VCodecVP8, sharedconsts.VCodecVP9},                                                    // TS supports MPEG-2, H.264, HEVC
	ExtMTS:  {sharedconsts.VCodecAV1, sharedconsts.VCodecVP8, sharedconsts.VCodecVP9, sharedconsts.VCodecMPEG2, sharedconsts.VCodecHEVC}, // AVCHD = H.264 only

	// Real/Windows/QuickTime ecosystems
	ExtASF:  {sharedconsts.VCodecAV1, sharedconsts.VCodecH264, sharedconsts.VCodecHEVC, sharedconsts.VCodecMPEG2, sharedconsts.VCodecVP8, sharedconsts.VCodecVP9}, // ASF expects WMV/VC-1
	ExtWMV:  {sharedconsts.VCodecAV1, sharedconsts.VCodecH264, sharedconsts.VCodecHEVC, sharedconsts.VCodecMPEG2, sharedconsts.VCodecVP8, sharedconsts.VCodecVP9}, // same as ASF
	ExtRM:   {sharedconsts.VCodecAV1, sharedconsts.VCodecH264, sharedconsts.VCodecHEVC, sharedconsts.VCodecMPEG2, sharedconsts.VCodecVP8, sharedconsts.VCodecVP9}, // RealMedia → RealVideo only
	ExtRMVB: {sharedconsts.VCodecAV1, sharedconsts.VCodecH264, sharedconsts.VCodecHEVC, sharedconsts.VCodecMPEG2, sharedconsts.VCodecVP8, sharedconsts.VCodecVP9},
}
