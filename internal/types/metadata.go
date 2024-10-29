package types

import (
	enums "Metarr/internal/domain/enums"
	"net/http"
)

type MetaReplaceSuffix struct {
	Field       string
	Suffix      string
	Replacement string
}

type MetaReplacePrefix struct {
	Field       string
	Prefix      string
	Replacement string
}

type MetaNewField struct {
	Field string
	Value string
}

type FilenameDatePrefix struct {
	YearLength  int
	MonthLength int
	DayLength   int
	Order       enums.FilenameDateFormat
}

type FilenameReplaceSuffix struct {
	Suffix      string
	Replacement string
}

type MetadataCredits struct {
	Actor      string `json:"actor" xml:"actor"`
	Actors     []string
	Author     string `json:"author" xml:"author"`
	Artist     string `json:"artist" xml:"artist"`
	Artists    []string
	Creator    string `json:"creator" xml:"creator"`
	Studio     string `json:"studio" xml:"studio"`
	Studios    []string
	Publisher  string `json:"publisher" xml:"publisher"`
	Publishers []string
	Producer   string `json:"producer" xml:"producer"`
	Producers  []string
	Performer  string `json:"performer" xml:"performer"`
	Performers []string
	Uploader   string `json:"uploader" xml:"uploader"`
	Composer   string `json:"composer" xml:"composer"`
	Composers  []string
	Director   string `json:"director" xml:"director"`
	Directors  []string
	Writer     string `json:"writer" xml:"writer"`
	Writers    []string
}

type MetadataTitlesDescs struct {
	Title            string `json:"fulltitle" xml:"title"`
	FallbackTitle    string `json:"title" xml:"originaltitle"`
	Subtitle         string `json:"subtitle" xml:"subtitle"`
	Description      string `json:"description" xml:"description"`
	LongDescription  string `json:"longdescription" xml:"plot"`
	Long_Description string `json:"long_description" xml:"long_description"`
	Synopsis         string `json:"synopsis" xml:"synopsis"`
	Summary          string `json:"summary" xml:"summary"`
	Comment          string `json:"comment" xml:"comment"`
}

type MetadataDates struct {
	FormattedDate           string `json:"-" xml:"-"`
	UploadDate              string `json:"upload_date" xml:"upload_date"`
	ReleaseDate             string `json:"release_date" xml:"release_date"`
	Date                    string `json:"date" xml:"date"`
	Year                    string `json:"year" xml:"year"`
	Originally_Available_At string `json:"originally_available_at" xml:"originally_available_at"`
	Creation_Time           string `json:"creation_time" xml:"creation_time"`
	StringDate              string `json:"-"`
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
	Show          string `json:"show" xml:"show"`
	Episode_ID    string `json:"episode_id" xml:"episode_id"`
	Episode_Sort  string `json:"episode_sort" xml:"episode_sort"`
	Season_Number string `json:"season_number" xml:"season_number"`
	Season_Title  string `json:"season_title" xml:"seasontitle"`
}

type MetadataOtherData struct {
	Language string `json:"language" xml:"language"`
	Genre    string `json:"genre" xml:"genre"`
	HD_Video string `json:"hd_video" xml:"hd_video"`
}
