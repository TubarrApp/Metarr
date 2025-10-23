package models

import (
	"metarr/internal/domain/enums"
	"net/http"
)

// NewMetaOps creates a new MetaOps with initialized maps.
//
// This ensures all map fields are non-nil and ready to use.
func NewMetaOps() *MetaOps {
	return &MetaOps{
		SetOverrides:     make(map[enums.OverrideMetaType]string),
		AppendOverrides:  make(map[enums.OverrideMetaType]string),
		ReplaceOverrides: make(map[enums.OverrideMetaType]MOverrideReplacePair),
		DateTags:         make(map[string]MetaDateTag),
		DeleteDateTags:   make(map[string]MetaDateTag),
		NewFields:        make([]MetaNewField, 0),
		Appends:          make([]MetaAppend, 0),
		Prefixes:         make([]MetaPrefix, 0),
		Replaces:         make([]MetaReplace, 0),
		TrimSuffixes:     make([]MetaTrimSuffix, 0),
		TrimPrefixes:     make([]MetaTrimPrefix, 0),
		CopyToFields:     make([]CopyToField, 0),
		PasteFromFields:  make([]PasteFromField, 0),
	}
}

// EnsureMetaOps returns the provided MetaOps or creates a new one if nil.
//
// This is useful for defensive programming to avoid nil pointer dereferences.
func EnsureMetaOps(maps *MetaOps) *MetaOps {
	if maps == nil {
		return NewMetaOps()
	}
	return maps
}

// BatchConfig holds data for the current batch's configuration options.
type BatchConfig struct {
	ID         int64
	Video      string
	JSON       string
	IsDirs     bool
	MetaOps    *MetaOps
	SkipVideos bool
}

type MOverrideReplacePair struct {
	Value       string
	Replacement string
}

type CopyToField struct {
	Field string
	Dest  string
}

type PasteFromField struct {
	Field  string
	Origin string
}

type MetaAppend struct {
	Field  string
	Suffix string
}

type MetaPrefix struct {
	Field  string
	Prefix string
}

type MetaTrimPrefix struct {
	Field  string
	Prefix string
}

type MetaTrimSuffix struct {
	Field  string
	Suffix string
}

type MetaNewField struct {
	Field string
	Value string
}

type MetaDateTag struct {
	Loc    enums.MetaDateTagLocation
	Format enums.DateFormat
}

type MetaReplace struct {
	Field       string
	Value       string
	Replacement string
}

type FilenameDatePrefix struct {
	YearLength  int
	MonthLength int
	DayLength   int
	Order       enums.DateFormat
}

type FilenameReplaceSuffix struct {
	Suffix      string
	Replacement string
}

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

type MetadataWebData struct {
	WebpageURL string         `json:"webpage_url" xml:"webpage_url"`
	VideoURL   string         `json:"url" xml:"url"`
	Domain     string         `json:"webpage_url_domain" xml:"domain"`
	Referer    string         `json:"referer" xml:"referer"`
	Cookies    []*http.Cookie `json:"-" xml:"-"`
	TryURLs    []string       `json:"-"`
}

type MetadataShowData struct {
	Show         string `json:"show" xml:"show"`
	EpisodeID    string `json:"episode_id" xml:"episode_id"`
	EpisodeSort  string `json:"episode_sort" xml:"episode_sort"`
	SeasonNumber string `json:"season_number" xml:"season_number"`
	SeasonTitle  string `json:"season_title" xml:"seasontitle"`
}

type MetadataOtherData struct {
	Language string `json:"language" xml:"language"`
	Genre    string `json:"genre" xml:"genre"`
	HDVideo  string `json:"hd_video" xml:"hd_video"`
}
