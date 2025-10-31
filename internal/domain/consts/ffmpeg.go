package consts

// AV codec copy.
var (
	AVCodecCopy = [...]string{"-c:v", "copy", "-c:a", "copy", "-c:s", "copy", "-c:d", "copy"}
)

// Audio flags.
var (
	AudioCodecCopy = [...]string{"-c:a", "copy"}
	AudioToAAC     = [...]string{"-c:a", "aac"}
	AudioToAC3     = [...]string{"-c:a", "ac3"}
	AudioToALAC    = [...]string{"-c:a", "alac"}
	AudioToDTS     = [...]string{"-c:a", "dts"}
	AudioToEAC3    = [...]string{"-c:a", "eac3"}
	AudioToFLAC    = [...]string{"-c:a", "flac"}
	AudioToMP2     = [...]string{"-c:a", "mp2"}
	AudioToMP3     = [...]string{"-c:a", "libmp3lame"}
	AudioToOpus    = [...]string{"-c:a", "libopus"}
	AudioToPCM     = [...]string{"-c:a", "pcm_s16le"}
	AudioToTrueHD  = [...]string{"-c:a", "truehd"}
	AudioToVorbis  = [...]string{"-c:a", "libvorbis"}
	AudioToWAV     = [...]string{"-c:a", "pcm_s16le"}
)

// Video codecs.
var (
	VideoCodecCopy = [...]string{"-c:v", "copy"}
	VideoToAV1     = [...]string{"-c:v", "libsvtav1"}
	VideoToH264    = [...]string{"-c:v", "libx264"}
	VideoToH265    = [...]string{"-c:v", "libx265"}
	VideoToMPEG2   = [...]string{"-c:v", "mpeg2video"}
	VideoToVP8     = [...]string{"-c:v", "libvpx"}
	VideoToVP9     = [...]string{"-c:v", "libvpx-vp9"}
)

// Video preset strings.
var (
	VideoToH264Balanced = [...]string{"-c:v", "libx264", "-profile:v", "main"}
	CRFQuality          = [...]string{"-crf", "20", "-preset", "slow"}
	PixelFmtYuv420p     = [...]string{"-pix_fmt", "yuv420p"}
)

// GPU hardware flags.
var (
	AccelFlagAuto   = []string{"-hwaccel", "auto"}
	AccelFlagAMF    = []string{"-hwaccel", "amf"}
	AccelFlagNvidia = [...]string{"-hwaccel", "cuda"}
	AccelFlagVAAPI  = [...]string{"-hwaccel", "vaapi"}
	AccelFlagIntel  = [...]string{"-hwaccel", "qsv"}
)

// HW Accel Flags.
var (
	VaapiCompatibility = []string{"-vf", "format=nv12,hwupload"}
)
