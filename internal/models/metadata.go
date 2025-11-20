package models

import (
	"metarr/internal/domain/enums"
	"net/http"
)

// MetaOps contains maps related to the operations to be carried out during the program's run.
type MetaOps struct {
	SetOverrides     map[enums.OverrideMetaType]string
	AppendOverrides  map[enums.OverrideMetaType]string
	ReplaceOverrides map[enums.OverrideMetaType]MOverrideReplacePair
	DateTags         map[string]MetaDateTag
	DeleteDateTags   map[string]MetaDeleteDateTag
	SetFields        []MetaSetField
	Appends          []MetaAppend
	Prefixes         []MetaPrefix
	Replaces         []MetaReplace
	ReplaceSuffixes  []MetaReplaceSuffix
	ReplacePrefixes  []MetaReplacePrefix
	CopyToFields     []CopyToField
	PasteFromFields  []PasteFromField
}

// NewMetaOps creates a new MetaOps with initialized maps.
//
// This ensures all map fields are non-nil and ready to use.
func NewMetaOps() *MetaOps {
	return &MetaOps{
		// Initialize all maps to prevent nil map panics.
		SetOverrides:     make(map[enums.OverrideMetaType]string),
		ReplaceOverrides: make(map[enums.OverrideMetaType]MOverrideReplacePair),
		AppendOverrides:  make(map[enums.OverrideMetaType]string),
		DateTags:         make(map[string]MetaDateTag),
		DeleteDateTags:   make(map[string]MetaDeleteDateTag),

		// Initialize slices
		SetFields:       make([]MetaSetField, 0),
		Appends:         make([]MetaAppend, 0),
		Prefixes:        make([]MetaPrefix, 0),
		ReplaceSuffixes: make([]MetaReplaceSuffix, 0),
		ReplacePrefixes: make([]MetaReplacePrefix, 0),
		Replaces:        make([]MetaReplace, 0),
		CopyToFields:    make([]CopyToField, 0),
		PasteFromFields: make([]PasteFromField, 0),
	}
}

// EnsureMetaOps initializes nil metadata operation structure values.
func (fd *FileData) EnsureMetaOps() {
	if fd.MetaOps == nil {
		fd.MetaOps = NewMetaOps()
		return
	}
	if fd.MetaOps.SetOverrides == nil {
		fd.MetaOps.SetOverrides = make(map[enums.OverrideMetaType]string, 0)
	}
	if fd.MetaOps.AppendOverrides == nil {
		fd.MetaOps.AppendOverrides = make(map[enums.OverrideMetaType]string, 0)
	}
	if fd.MetaOps.ReplaceOverrides == nil {
		fd.MetaOps.ReplaceOverrides = make(map[enums.OverrideMetaType]MOverrideReplacePair, 0)
	}
	if fd.MetaOps.DateTags == nil {
		fd.MetaOps.DateTags = make(map[string]MetaDateTag, 0)
	}
	if fd.MetaOps.DeleteDateTags == nil {
		fd.MetaOps.DeleteDateTags = make(map[string]MetaDeleteDateTag, 0)
	}
	if fd.MetaOps.SetFields == nil {
		fd.MetaOps.SetFields = []MetaSetField{}
	}
	if fd.MetaOps.Appends == nil {
		fd.MetaOps.Appends = []MetaAppend{}
	}
	if fd.MetaOps.Prefixes == nil {
		fd.MetaOps.Prefixes = []MetaPrefix{}
	}
	if fd.MetaOps.Replaces == nil {
		fd.MetaOps.Replaces = []MetaReplace{}
	}
	if fd.MetaOps.ReplaceSuffixes == nil {
		fd.MetaOps.ReplaceSuffixes = []MetaReplaceSuffix{}
	}
	if fd.MetaOps.ReplacePrefixes == nil {
		fd.MetaOps.ReplacePrefixes = []MetaReplacePrefix{}
	}
	if fd.MetaOps.CopyToFields == nil {
		fd.MetaOps.CopyToFields = []CopyToField{}
	}
	if fd.MetaOps.PasteFromFields == nil {
		fd.MetaOps.PasteFromFields = []PasteFromField{}
	}
}

// BatchConfig holds data for the current batch's configuration options.
type BatchConfig struct {
	ID         int64
	Video      string
	JSON       string
	IsDirs     bool
	SkipVideos bool
}

// MOverrideReplacePair holds a value and replacement for overriding metadata.
type MOverrideReplacePair struct {
	Value       string
	Replacement string
}

// CopyToField contains a field to copy from, and destination field to copy to.
type CopyToField struct {
	Field string
	Dest  string
}

// PasteFromField contains a field to paste to, and field to paste from.
type PasteFromField struct {
	Field  string
	Origin string
}

// MetaAppend appends text onto a metafield's value.
type MetaAppend struct {
	Field  string
	Append string
}

// MetaPrefix prefixes text onto a metafield's value.
type MetaPrefix struct {
	Field  string
	Prefix string
}

// MetaReplacePrefix trims a given prefix from a metafield's value.
type MetaReplacePrefix struct {
	Field       string
	Prefix      string
	Replacement string
}

// MetaReplaceSuffix trims a given suffix from a metafield's value.
type MetaReplaceSuffix struct {
	Field       string
	Suffix      string
	Replacement string
}

// MetaSetField contains a new field and value to add to metadata.
type MetaSetField struct {
	Field string
	Value string
}

// MetaDateTag contains the location for a date tag placement, and format (e.g. ymd).
type MetaDateTag struct {
	// Don't need field, using map
	Loc    enums.DateTagLocation
	Format enums.DateFormat
}

// MetaDeleteDateTag contains the location for a date tag placement, and format (e.g. ymd).
type MetaDeleteDateTag struct {
	// Don't need field, using map
	Loc    enums.DateTagLocation
	Format enums.DateFormat
}

// MetaReplace contains a field with a given value, and its desired replacement.
type MetaReplace struct {
	Field       string
	Value       string
	Replacement string
}

// FilenameDatePrefix contains the year, month, day lengths, and ordering, desired by the user.
type FilenameDatePrefix struct {
	YearLength  int
	MonthLength int
	DayLength   int
	Order       enums.DateFormat
}

// FilenameReplaceSuffix replaces a suffix from a given filename.
type FilenameReplaceSuffix struct {
	Suffix      string
	Replacement string
}

// FilenameReplacePrefix replaces a suffix from a given filename.
type FilenameReplacePrefix struct {
	Prefix      string
	Replacement string
}

// FilenameReplaceStrings replaces strings in a filename with user input.
type FilenameReplaceStrings struct {
	FindString  string
	ReplaceWith string
}

// MetadataCredits contains credits metadata.
type MetadataCredits struct {
	Override  string `json:"-"`
	Actor     string `json:"actor" xml:"actor"`
	Author    string `json:"author" xml:"author"`
	Artist    string `json:"artist" xml:"artist"`
	Channel   string `json:"channel" xml:"channel"`
	Creator   string `json:"creator" xml:"creator"`
	Studio    string `json:"studio" xml:"studio"`
	Publisher string `json:"publisher" xml:"publisher"`
	Producer  string `json:"producer" xml:"producer"`
	Performer string `json:"performer" xml:"performer"`
	Uploader  string `json:"uploader" xml:"uploader"`
	Composer  string `json:"composer" xml:"composer"`
	Director  string `json:"director" xml:"director"`
	Writer    string `json:"writer" xml:"writer"`

	Actors     []string
	Artists    []string
	Studios    []string
	Publishers []string
	Producers  []string
	Performers []string
	Composers  []string
	Directors  []string
	Writers    []string
}

// MetadataTitlesDescs contains title and description metadata.
type MetadataTitlesDescs struct {
	Fulltitle                 string `json:"fulltitle" xml:"title"`
	Title                     string `json:"title" xml:"originaltitle"`
	Subtitle                  string `json:"subtitle" xml:"subtitle"`
	Description               string `json:"description" xml:"description"`
	LongDescription           string `json:"longdescription" xml:"plot"`
	LongUnderscoreDescription string `json:"long_description" xml:"long_description"`
	Synopsis                  string `json:"synopsis" xml:"synopsis"`
	Summary                   string `json:"summary" xml:"summary"`
	Comment                   string `json:"comment" xml:"comment"`
}

// MetadataDates contains time and date metadata.
type MetadataDates struct {
	FormattedDate         string `json:"-" xml:"-"`
	UploadDate            string `json:"upload_date" xml:"upload_date"`
	ReleaseDate           string `json:"release_date" xml:"release_date"`
	Date                  string `json:"date" xml:"date"`
	Year                  string `json:"year" xml:"year"`
	OriginallyAvailableAt string `json:"originally_available_at" xml:"originally_available_at"`
	CreationTime          string `json:"creation_time" xml:"creation_time"`
	StringDate            string `json:"-"`
}

// MetadataWebData contains web related metadata.
type MetadataWebData struct {
	WebpageURL string         `json:"webpage_url" xml:"webpage_url"`
	VideoURL   string         `json:"url" xml:"url"`
	Domain     string         `json:"webpage_url_domain" xml:"domain"`
	Referer    string         `json:"referer" xml:"referer"`
	Thumbnail  string         `json:"thumbnail" xml:"thumbnail"`
	Cookies    []*http.Cookie `json:"-" xml:"-"`
	TryURLs    []string       `json:"-"`
}

// MetadataShowData contains main show info metadata.
type MetadataShowData struct {
	Show         string `json:"show" xml:"show"`
	EpisodeID    string `json:"episode_id" xml:"episode_id"`
	EpisodeSort  string `json:"episode_sort" xml:"episode_sort"`
	SeasonNumber string `json:"season_number" xml:"season_number"`
	SeasonTitle  string `json:"season_title" xml:"seasontitle"`
}

// MetadataOtherData contains other misc metadata.
type MetadataOtherData struct {
	Language string `json:"language" xml:"language"`
	Genre    string `json:"genre" xml:"genre"`
	HDVideo  string `json:"hd_video" xml:"hd_video"`
}
