package domain

// AV codec copy
var (
	AVCodecCopy = [...]string{"-codec", "copy"}
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
	VideoToH264Balanced = [...]string{"-c:v", "libx264", "-crf", "23", "-profile:v", "main"}
	PixelFmtYuv420p     = [...]string{"-pix_fmt", "yuv420p"}
	KeyframeBalanced    = [...]string{"-g", "50", "-keyint_min", "30"}
)

// GPU hardware flags
var (
	NvidiaAccel = [...]string{"-hwaccel", "nvdec"}
	AMDAccel    = [...]string{"-hwaccel", "vulkan"}
	IntelAccel  = [...]string{"-hwaccel", "qsv"}
)
