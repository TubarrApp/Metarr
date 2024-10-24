package models

import (
	"net/http"
	"os"
)

func NewFileData() *FileData {
	return &FileData{
		MTitleDesc: &MetadataTitlesDescs{},
		MCredits:   &MetadataCredits{},
		MDates:     &MetadataDates{},
		MShowData:  &MetadataShowData{},
		MWebData:   &MetadataWebData{},
		MOther:     &MetadataOtherData{},
	}
}

type FileData struct {
	VideoDirectory        string   `json:"-"`
	OriginalVideoPath     string   `json:"-"`
	OriginalVideoBaseName string   `json:"-"`
	TempOutputFilePath    string   `json:"-"`
	FinalVideoPath        string   `json:"-"`
	FinalVideoBaseName    string   `json:"-"`
	FilenameMetaPrefix    string   `json:"-"`
	FilenameDateTag       string   `json:"-"`
	VideoFile             *os.File `json:"-"`

	// JSON paths

	JSONDirectory string `json:"-"`
	JSONFilePath  string `json:"-"`
	JSONBaseName  string `json:"-"`

	// NFO paths

	NFODirectory string `json:"-"`
	NFOFilePath  string `json:"-"`

	// Metadata

	MCredits   *MetadataCredits     `json:"meta_credits"`
	MTitleDesc *MetadataTitlesDescs `json:"meta_title_description"`
	MDates     *MetadataDates       `json:"meta_dates"`
	MShowData  *MetadataShowData    `json:"meta_show_data"`
	MWebData   *MetadataWebData     `json:"meta_web_data"`
	MOther     *MetadataOtherData   `json:"meta_other_data"`
}

type MetadataCredits struct {
	Actor     string `json:"actor"`
	Author    string `json:"author"`
	Artist    string `json:"artist"`
	Creator   string `json:"creator"`
	Studio    string `json:"studio"`
	Publisher string `json:"publisher"`
	Producer  string `json:"producer"`
	Performer string `json:"performer"`
	Uploader  string `json:"uploader"`
	Composer  string `json:"composer"` // Writer(s)
	Director  string `json:"director"`
}

type MetadataTitlesDescs struct {
	Title            string `json:"fulltitle"`
	FallbackTitle    string `json:"title"`
	Subtitle         string `json:"subtitle"`
	Description      string `json:"description"`
	LongDescription  string `json:"longdescription"`
	Long_Description string `json:"long_description"`
	Synopsis         string `json:"synopsis"`
	Summary          string `json:"summary"`
	Comment          string `json:"comment"`
}

type MetadataDates struct {
	FormattedDate           string `json:"-"`
	UploadDate              string `json:"upload_date"`
	ReleaseDate             string `json:"release_date"`
	Date                    string `json:"date"`
	Year                    string `json:"year"`
	Originally_Available_At string `json:"originally_available_at"`
	Creation_Time           string `json:"creation_time"` // YYYY-MM-DDT00:00:00Z
}

// Video page URL
type MetadataWebData struct {
	WebpageURL string         `json:"webpage_url"`
	VideoURL   string         `json:"url"`
	Domain     string         `json:"webpage_url_domain"`
	Referer    string         `json:"referer"`
	Cookies    []*http.Cookie `json:"-"`
}

type MetadataShowData struct {
	// Series
	Show          string `json:"show"`
	Episode_ID    string `json:"episode_id"`   // TV episode ID
	Episode_Sort  string `json:"episode_sort"` // Episode number
	Season_Number string `json:"season_number"`
}

type MetadataOtherData struct {
	Language string `json:"language"`
	Genre    string `json:"genre"`
	HD_Video string `json:"hd_video"` // HD flag (0 = SD, 1 = 720p, 2 = 1080p/i Full HD, 3 = 2160p UHD)
}
