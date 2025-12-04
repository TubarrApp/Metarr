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
	FFmpegCRF                 = "-crf"
	FFmpegBA                  = "-b:a"
	FFmpegAR                  = "-ar"
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

// IncompatibleCodecsForContainer by container/extension.
var IncompatibleCodecsForContainer = map[string][]string{
	// ISO BMFF family.
	sharedconsts.ExtMP4: {sharedconsts.VCodecVP8, sharedconsts.VCodecVP9, sharedconsts.VCodecMPEG2}, // AV1, H.264, HEVC OK.
	sharedconsts.ExtM4V: {sharedconsts.VCodecVP8, sharedconsts.VCodecVP9, sharedconsts.VCodecMPEG2}, // same as MP4.
	sharedconsts.ExtMOV: {sharedconsts.VCodecVP8, sharedconsts.VCodecVP9, sharedconsts.VCodecAV1},   // H.264, HEVC widely OK, MPEG-2 often OK.

	// WebM / Matroska.
	sharedconsts.ExtWEBM: {sharedconsts.VCodecH264, sharedconsts.VCodecHEVC, sharedconsts.VCodecMPEG2}, // WebM only supports VP8, VP9, AV1.
	sharedconsts.ExtMKV:  {},                                                                           // Matroska supports all listed codecs.

	// Legacy/desktop.
	sharedconsts.ExtAVI: {sharedconsts.VCodecAV1, sharedconsts.VCodecHEVC, sharedconsts.VCodecVP8, sharedconsts.VCodecVP9, sharedconsts.VCodecH264},  // nonstandard for modern codecs.
	sharedconsts.ExtFLV: {sharedconsts.VCodecAV1, sharedconsts.VCodecHEVC, sharedconsts.VCodecMPEG2, sharedconsts.VCodecVP8, sharedconsts.VCodecVP9}, // FLV supports only H.263/VP6/H.264.
	sharedconsts.ExtF4V: {sharedconsts.VCodecAV1, sharedconsts.VCodecHEVC, sharedconsts.VCodecMPEG2, sharedconsts.VCodecVP8, sharedconsts.VCodecVP9}, // Flash MP4 variant; H.264 only.

	// Mobile-era.
	sharedconsts.Ext3GP: {sharedconsts.VCodecAV1, sharedconsts.VCodecHEVC, sharedconsts.VCodecMPEG2, sharedconsts.VCodecVP8, sharedconsts.VCodecVP9}, // supports H.264, MPEG-4, H.264.
	sharedconsts.Ext3G2: {sharedconsts.VCodecAV1, sharedconsts.VCodecHEVC, sharedconsts.VCodecMPEG2, sharedconsts.VCodecVP8, sharedconsts.VCodecVP9},

	// Ogg family.
	sharedconsts.ExtOGV: {sharedconsts.VCodecAV1, sharedconsts.VCodecH264, sharedconsts.VCodecHEVC, sharedconsts.VCodecMPEG2, sharedconsts.VCodecVP8, sharedconsts.VCodecVP9}, // OGV → Theora/Dirac.
	sharedconsts.ExtOGM: {sharedconsts.VCodecAV1, sharedconsts.VCodecH264, sharedconsts.VCodecHEVC, sharedconsts.VCodecMPEG2, sharedconsts.VCodecVP8, sharedconsts.VCodecVP9}, // obsolete; treat as incompatible.

	// MPEG program/transport & derivatives.
	sharedconsts.ExtMPG:  {sharedconsts.VCodecAV1, sharedconsts.VCodecH264, sharedconsts.VCodecHEVC, sharedconsts.VCodecVP8, sharedconsts.VCodecVP9}, // only MPEG-1/2 video is valid.
	sharedconsts.ExtMPEG: {sharedconsts.VCodecAV1, sharedconsts.VCodecH264, sharedconsts.VCodecHEVC, sharedconsts.VCodecVP8, sharedconsts.VCodecVP9},
	sharedconsts.ExtVOB:  {sharedconsts.VCodecAV1, sharedconsts.VCodecH264, sharedconsts.VCodecHEVC, sharedconsts.VCodecVP8, sharedconsts.VCodecVP9},  // DVD-Video → MPEG-2 only.
	sharedconsts.ExtTS:   {sharedconsts.VCodecAV1, sharedconsts.VCodecVP8, sharedconsts.VCodecVP9},                                                    // TS supports MPEG-2, H.264, HEVC.
	sharedconsts.ExtMTS:  {sharedconsts.VCodecAV1, sharedconsts.VCodecVP8, sharedconsts.VCodecVP9, sharedconsts.VCodecMPEG2, sharedconsts.VCodecHEVC}, // AVCHD = H.264 only.

	// Real/Windows/QuickTime.
	sharedconsts.ExtASF:  {sharedconsts.VCodecAV1, sharedconsts.VCodecH264, sharedconsts.VCodecHEVC, sharedconsts.VCodecMPEG2, sharedconsts.VCodecVP8, sharedconsts.VCodecVP9}, // ASF expects WMV/VC-1.
	sharedconsts.ExtWMV:  {sharedconsts.VCodecAV1, sharedconsts.VCodecH264, sharedconsts.VCodecHEVC, sharedconsts.VCodecMPEG2, sharedconsts.VCodecVP8, sharedconsts.VCodecVP9}, // same as ASF.
	sharedconsts.ExtRM:   {sharedconsts.VCodecAV1, sharedconsts.VCodecH264, sharedconsts.VCodecHEVC, sharedconsts.VCodecMPEG2, sharedconsts.VCodecVP8, sharedconsts.VCodecVP9}, // RealMedia → RealVideo only.
	sharedconsts.ExtRMVB: {sharedconsts.VCodecAV1, sharedconsts.VCodecH264, sharedconsts.VCodecHEVC, sharedconsts.VCodecMPEG2, sharedconsts.VCodecVP8, sharedconsts.VCodecVP9},
}
