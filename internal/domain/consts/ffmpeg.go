package consts

// AV codec copy
var (
	AVCodecCopy = [...]string{"-c:v", "copy", "-c:a", "copy", "-c:s", "copy", "-c:d", "copy"}
)

// Audio flags
var (
	AudioCodecCopy = [...]string{"-c:a", "copy"}
	AudioToAAC     = [...]string{"-c:a", "aac"}
)

// Video flags
var (
	VideoCodecCopy      = [...]string{"-c:v", "copy"}
	VideoToH264         = [...]string{"-c:v", "libx264"}
	VideoToH265         = [...]string{"-c:v", "libx265"}
	VideoToH264Balanced = [...]string{"-c:v", "libx264", "-profile:v", "main"}
	CRFQuality          = [...]string{"-crf", "20", "-preset", "slow"}
	PixelFmtYuv420p     = [...]string{"-pix_fmt", "yuv420p"}
)

// GPU hardware flags
var (
	NvidiaAccel = [...]string{"-hwaccel", "cuda"}
	VaapiAccel  = [...]string{}
	IntelAccel  = [...]string{}
	AutoHWAccel = []string{"-hwaccel", "auto"}
)

// HW Accel Flags
var (
	VaapiCompatibility = []string{"-vf", "format=nv12,hwupload"}
)
