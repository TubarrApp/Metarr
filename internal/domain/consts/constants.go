// Package consts holds global unchanging constants and variables.
package consts

// Tags
const (
	LogTagError    string = ColorRed + "[ERROR] " + ColorReset
	LogTagSuccess  string = ColorGreen + "[Success] " + ColorReset
	LogTagDebug    string = ColorYellow + "[Debug] " + ColorReset
	LogTagWarning  string = ColorYellow + "[Warning] " + ColorReset
	LogTagInfo     string = ColorCyan + "[Info] " + ColorReset
	LogTagDevError string = ColorRed + "[ DEV ERROR! ]" + ColorReset
)

// Browser name constants
const (
	BrowserChrome  = "chrome"
	BrowserEdge    = "edge"
	BrowserFirefox = "firefox"
	BrowserSafari  = "safari"
)

// File prefix and suffix
const (
	BackupTag = "_metarrbackup"
	TempTag   = "tmp_"
)

// Buffers
const (
	KB        = 1 * 1024
	GB        = 1024 * 1024 * 1024
	MB        = 1024 * 1024
	Buffer4MB = 4 * 1024 * 1024
)

// Video codecs.
const (
	VCodecAV1   = "av1"
	VCodecH264  = "h264"
	VCodecHEVC  = "hevc"
	VCodecMPEG2 = "mpeg2"
	VCodecVP8   = "vp8"
	VCodecVP9   = "vp9"
)

// Audio codecs.
const (
	ACodecAAC    = "aac"
	ACodecAC3    = "ac3"
	ACodecALAC   = "alac"
	ACodecDTS    = "dts"
	ACodecEAC3   = "eac3"
	ACodecFLAC   = "flac"
	ACodecMP2    = "mp2"
	ACodecMP3    = "mp3"
	ACodecOpus   = "opus"
	ACodecPCM    = "pcm"
	ACodecTrueHD = "truehd"
	ACodecVorbis = "vorbis"
	ACodecWAV    = "wav"
)

// Accel types.
const (
	AccelTypeAMF   = "amf"
	AccelTypeAuto  = "auto"
	AccelTypeNVENC = "nvenc"
	AccelTypeVAAPI = "vaapi"
	AccelTypeQSV   = "qsv"
)

// TimeSfx is used as a time suffix format in metadata.
const TimeSfx = "T00:00:00Z"

// ValidVideoCodecs contains all valid video codecs.
var ValidVideoCodecs = []string{
	VCodecAV1,
	VCodecH264,
	VCodecHEVC,
	VCodecMPEG2,
	VCodecVP8,
	VCodecVP9,
}

// ValidAudioCodecs contains all valid audio codecs.
var ValidAudioCodecs = []string{
	ACodecAAC,
	ACodecALAC,
	ACodecDTS,
	ACodecEAC3,
	ACodecFLAC,
	ACodecMP2,
	ACodecMP3,
	ACodecOpus,
	ACodecPCM,
	ACodecTrueHD,
	ACodecVorbis,
	ACodecWAV,
}

// Webpage tags
var (
	// Ensure lengths match
	WebDateTags        = [...]string{"release-date", "upload-date", "date", "date-text", "text-date"}
	WebDescriptionTags = [...]string{"description", "longdescription", "long-description", "summary", "synopsis",
		"check-for-urls"}

	// Credits tags, and nested elements
	WebCreditsTags      = [...]string{"creator", "uploader", "uploaded-by", "uploaded_by", "channel-name", "claim-preview__title"}
	WebCreditsSelectors = map[string]string{
		"claim-preview__title":               "truncated-text",
		`script[type="application/ld+json"]`: "author.name",
	}

	WebTitleTags = [...]string{"video-title", "video-name"}
)
