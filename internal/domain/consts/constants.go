// Package consts holds global unchanging constants and variables.
package consts

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

// TimeSfx is used as a time suffix format in metadata.
const TimeSfx = "T00:00:00Z"

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
