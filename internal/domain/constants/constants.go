package domain

// File prefix and suffix
const (
	OldTag  = "_metarrbackup"
	TempTag = "tmp_"
)

var (
	AllVidExtensions = [...]string{".3gp", ".avi", ".f4v", ".flv", ".m4v",
		".mkv", ".mov", ".mp4", ".mpeg", ".mpg",
		".ogm", ".ogv", ".ts", ".vob", ".webm",
		".wmv"}
)

var (
	AllMetaExtensions = [...]string{".json", ".nfo"}
)

// Webpage tags
var (
	// Ensure lengths match
	WebDateTags        = [...]string{"release-date", "upload-date", "date", "date-text", "text-date"}
	WebDescriptionTags = [...]string{"description", "longdescription", "long-description", "summary", "synopsis",
		"check-for-urls"}

	// Credits tags, and nested elements
	WebCreditsTags      = [...]string{"creator", "uploader", "uploaded-by", "uploaded_by", "claim-preview__title"}
	WebCreditsSelectors = map[string]string{
		"claim-preview__title":               "truncated-text",
		`script[type="application/ld+json"]`: "author.name",
	}

	WebTitleTags = [...]string{"video-title", "video-name"}
)
