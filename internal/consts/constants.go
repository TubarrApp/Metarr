package consts

import "fmt"

// Colors
const (
	ColorReset  = "\033[0m"
	ColorRed    = "\033[91m"
	ColorGreen  = "\033[92m"
	ColorYellow = "\033[93m"
	ColorBlue   = "\033[34m"
	ColorPurple = "\033[35m"
	ColorCyan   = "\033[96m"
	ColorWhite  = "\033[37m"
)

var RedError string = fmt.Sprintf("%v[ERROR] %v", ColorRed, ColorReset)
var YellowDebug string = fmt.Sprintf("%v[DEBUG] %v", ColorYellow, ColorReset)
var GreenSuccess string = fmt.Sprintf("%v[SUCCESS] %v", ColorGreen, ColorReset)
var BlueInfo string = fmt.Sprintf("%v[Info] %v", ColorCyan, ColorReset)

// File prefix and suffix
const (
	OldTag  = "_metarrbackup"
	TempTag = "tmp_"
)

// Log file keys
const (
	LogFinished = "FINISHED: "
	LogError    = "ERROR: "
	LogFailure  = "FAILED: "
	LogSuccess  = "Success: "
	LogInfo     = "Info: "
	LogWarning  = "Warning: "
	LogBasic    = ""
)

// Webpage tags
var WebDateTags = []string{"release-date", "upload-date", "date", "date-text"}
var WebDescriptionTags = []string{"description", "longdescription", "long-description", "summary", "synopsis", "check-for-urls"}
var WebCreditsTags = []string{"creator", "uploader", "uploaded-by", "uploaded_by"}

const (
	JActor     = "actor"
	JAuthor    = "author"
	JArtist    = "artist"
	JComposer  = "composer"
	JCreator   = "creator"
	JDirector  = "director"
	JPerformer = "performer"
	JProducer  = "producer"
	JPublisher = "publisher"
	JStudio    = "studio"
	JUploader  = "uploader"
)

const (
	JComment          = "comment"
	JDescription      = "description"
	JFallbackTitle    = "title"
	JLongDescription  = "longdescription"
	JLong_Description = "long_description"
	JSubtitle         = "subtitle"
	JSummary          = "summary"
	JSynopsis         = "synopsis"
	JTitle            = "fulltitle"
)

const (
	JCreationTime        = "creation_time"
	JDate                = "date"
	JFormattedDate       = "formatted_date"
	JOriginallyAvailable = "originally_available_at"
	JReleaseDate         = "release_date"
	JUploadDate          = "upload_date"
	JYear                = "year"
	JReleaseYear         = "release_year"
)

const (
	JDomain        = "domain"
	JReferer       = "referer"
	JURL           = "url"
	JWebpageDomain = "webpage_url_domain"
	JWebpageURL    = "webpage_url"
)
