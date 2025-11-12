package consts

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
	ExtMP4: {VCodecVP8, VCodecVP9, VCodecMPEG2}, // AV1, H.264, HEVC OK
	ExtM4V: {VCodecVP8, VCodecVP9, VCodecMPEG2}, // same as MP4
	ExtMOV: {VCodecVP8, VCodecVP9, VCodecAV1},   // H.264, HEVC widely OK; MPEG-2 often OK

	// WebM / Matroska
	ExtWEBM: {VCodecH264, VCodecHEVC, VCodecMPEG2}, // WebM only supports VP8, VP9, AV1
	ExtMKV:  {},                                    // Matroska supports all listed codecs

	// Legacy/desktop
	ExtAVI: {VCodecAV1, VCodecHEVC, VCodecVP8, VCodecVP9, VCodecH264},  // nonstandard for modern codecs
	ExtFLV: {VCodecAV1, VCodecHEVC, VCodecMPEG2, VCodecVP8, VCodecVP9}, // FLV supports only H.263/VP6/H.264
	ExtF4V: {VCodecAV1, VCodecHEVC, VCodecMPEG2, VCodecVP8, VCodecVP9}, // Flash MP4 variant; H.264 only

	// Mobile-era
	Ext3GP: {VCodecAV1, VCodecHEVC, VCodecMPEG2, VCodecVP8, VCodecVP9}, // supports H.263, MPEG-4 Part 2, H.264
	Ext3G2: {VCodecAV1, VCodecHEVC, VCodecMPEG2, VCodecVP8, VCodecVP9},

	// Ogg family
	ExtOGV: {VCodecAV1, VCodecH264, VCodecHEVC, VCodecMPEG2, VCodecVP8, VCodecVP9}, // OGV → Theora/Dirac
	ExtOGM: {VCodecAV1, VCodecH264, VCodecHEVC, VCodecMPEG2, VCodecVP8, VCodecVP9}, // obsolete; treat as incompatible

	// MPEG program/transport & derivatives
	ExtMPG:  {VCodecAV1, VCodecH264, VCodecHEVC, VCodecVP8, VCodecVP9}, // only MPEG-1/2 video is valid
	ExtMPEG: {VCodecAV1, VCodecH264, VCodecHEVC, VCodecVP8, VCodecVP9},
	ExtVOB:  {VCodecAV1, VCodecH264, VCodecHEVC, VCodecVP8, VCodecVP9},  // DVD-Video → MPEG-2 only
	ExtTS:   {VCodecAV1, VCodecVP8, VCodecVP9},                          // TS supports MPEG-2, H.264, HEVC
	ExtMTS:  {VCodecAV1, VCodecVP8, VCodecVP9, VCodecMPEG2, VCodecHEVC}, // AVCHD = H.264 only

	// Real/Windows/QuickTime ecosystems
	ExtASF:  {VCodecAV1, VCodecH264, VCodecHEVC, VCodecMPEG2, VCodecVP8, VCodecVP9}, // ASF expects WMV/VC-1
	ExtWMV:  {VCodecAV1, VCodecH264, VCodecHEVC, VCodecMPEG2, VCodecVP8, VCodecVP9}, // same as ASF
	ExtRM:   {VCodecAV1, VCodecH264, VCodecHEVC, VCodecMPEG2, VCodecVP8, VCodecVP9}, // RealMedia → RealVideo only
	ExtRMVB: {VCodecAV1, VCodecH264, VCodecHEVC, VCodecMPEG2, VCodecVP8, VCodecVP9},
}
