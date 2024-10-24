package commandvars

var AVCodecCopy = []string{"-codec", "copy"}
var VideoCodecCopy = []string{"-c:v", "copy"}
var AudioCodecCopy = []string{"-c:a", "copy"}

var AudioToAAC = []string{"-c:a", "aac"}
var VideoToH264Balanced = []string{"-c:v", "libx264", "-crf", "23", "-profile:v", "main"}
var AudioBitrate = []string{"-b:a", "256k"}

var PixelFmtYuv420p = []string{"-pix_fmt", "yuv420p"}
var KeyframeBalanced = []string{"-g", "50", "-keyint_min", "30"}

var OutputExt = []string{"-f", "mp4"}

var NvidiaAccel = []string{"-hwaccel", "nvdec"}
var AMDAccel = []string{"-hwaccel", "vulkan"}
var IntelAccel = []string{"-hwaccel", "qsv"}
