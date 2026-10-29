// Package consts holds global unchanging constants and variables.
package consts

// Browser name constants.
const (
	BrowserChrome  = "chrome"
	BrowserEdge    = "edge"
	BrowserFirefox = "firefox"
	BrowserSafari  = "safari"
)

// Bytes.
const (
	KB = 1024
	MB = 1024 * 1024
	GB = 1024 * 1024 * 1024
)

// Program.
const (
	TimeSfx = "T00:00:00Z"
)

// Webpage tags.
var (
	// Ensure lengths match.
	WebDateTags        = [...]string{"release-date", "upload-date", "date", "date-text", "text-date"}
	WebDescriptionTags = [...]string{"description", "longdescription", "long-description", "summary", "synopsis",
		"check-for-urls"}

	// Credits tags, and nested elements.
	WebCreditsTags      = [...]string{"creator", "uploader", "uploaded-by", "uploaded_by", "channel-name", "claim-preview__title"}
	WebCreditsSelectors = map[string]string{
		"claim-preview__title":               "truncated-text",
		`script[type="application/ld+json"]`: "author.name",
	}

	WebTitleTags = [...]string{"video-title", "video-name"}
)
