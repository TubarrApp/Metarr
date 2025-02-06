package consts

// AV codec copy
var (
	AVCodecCopy = [...]string{"-c:v", "copy", "-c:a", "copy", "-c:s", "copy", "-c:d", "copy"}
)

// Audio flags
var (
	AudioCodecCopy = [...]string{"-c:a", "copy"}
	AudioToAAC     = [...]string{"-c:a", "aac"}
	AudioBitrate   = [...]string{"-b:a", "256k"}
)

// Video flags
var (
	VideoCodecCopy      = [...]string{"-c:v", "copy"}
	VideoToH264Balanced = [...]string{"-c:v", "libx264", "-profile:v", "main"}
	CRFQuality          = [...]string{"-crf", "23"}
	PixelFmtYuv420p     = [...]string{"-pix_fmt", "yuv420p"}
	KeyframeBalanced    = [...]string{"-g", "50", "-keyint_min", "30"}
)

// GPU hardware flags
var (
	NvidiaAccel = [...]string{"-hwaccel", "cuda", "-hwaccel_output_format", "nvenc"}
	AMDAccel    = [...]string{"-hwaccel", "vaapi", "-hwaccel_output_format", "vaapi"}
	IntelAccel  = [...]string{"-hwaccel", "qsv", "-hwaccel_output_format", "qsv"}
	AutoHWAccel = []string{"-hwaccel", "auto"}
)

// HW Accel Flags
var (
	VaapiCompatibility = []string{"-vf", "hwupload"}
)
