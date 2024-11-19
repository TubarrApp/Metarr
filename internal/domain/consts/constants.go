package consts

// File prefix and suffix
const (
	BackupTag = "_metarrbackup"
	TempTag   = "tmp_"
)

const TimeSfx = "T00:00:00Z"

var (
	AllVidExtensions = map[string]bool{
		Ext3G2: false, Ext3GP: false, ExtAVI: false, ExtF4V: false, ExtFLV: false,
		ExtM4V: false, ExtMKV: false, ExtMOV: false, ExtMP4: false, ExtMPEG: false,
		ExtMPG: false, ExtMTS: false, ExtOGM: false, ExtOGV: false, ExtRM: false,
		ExtRMVB: false, ExtTS: false, ExtVOB: false, ExtWEBM: false, ExtWMV: false,
		ExtASF: false,
	}
)

var (
	AllMetaExtensions = map[string]bool{
		MExtJSON: false, MExtNFO: false,
	}
)

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
