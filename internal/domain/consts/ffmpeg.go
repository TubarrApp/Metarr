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
	AccelTypeQSV    = "qsv"
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
)

// Compatability for HW acceleration types.
var (
	VAAPICompatability = []string{"format=nv12|vaapi,hwupload"}
	CudaCompatability  = []string{"hwdownload,format=nv12,hwupload_cuda"}
)
