package types

import (
	enums "Metarr/internal/domain/enums"
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
	VideoDirectory        string   `json:"-" xml:"-"`
	OriginalVideoPath     string   `json:"-" xml:"-"`
	OriginalVideoBaseName string   `json:"-" xml:"-"`
	TempOutputFilePath    string   `json:"-" xml:"-"`
	FinalVideoPath        string   `json:"-" xml:"-"`
	FinalVideoBaseName    string   `json:"-" xml:"-"`
	FilenameMetaPrefix    string   `json:"-" xml:"-"`
	FilenameDateTag       string   `json:"-" xml:"-"`
	RenamedVideo          string   `json:"-"`
	RenamedMeta           string   `json:"-"`
	VideoFile             *os.File `json:"-" xml:"-"`
	// JSON paths
	JSONDirectory string `json:"-" xml:"-"`
	JSONFilePath  string `json:"-" xml:"-"`
	JSONBaseName  string `json:"-" xml:"-"`
	// NFO paths
	NFOBaseName  string `json:"-" xml:"-"`
	NFODirectory string `json:"-" xml:"-"`
	NFOFilePath  string `json:"-" xml:"-"`
	// Meta type
	MetaFileType enums.MetaFileTypeEnum `json:"-" xml:"-"`
	// Metadata
	MCredits   *MetadataCredits     `json:"meta_credits" xml:"credits"`
	MTitleDesc *MetadataTitlesDescs `json:"meta_title_description" xml:"titles"`
	MDates     *MetadataDates       `json:"meta_dates" xml:"dates"`
	MShowData  *MetadataShowData    `json:"meta_show_data" xml:"show"`
	MWebData   *MetadataWebData     `json:"meta_web_data" xml:"web"`
	MOther     *MetadataOtherData   `json:"meta_other_data" xml:"other"`

	JSONFileRW JSONFileRW
	NFOFileRW  NFOFileRW
	NFOData    *NFOData
}
