// Package models holds structs used throughout the Metarr program.
package models

import (
	"context"
	"metarr/internal/domain/enums"
	"os"
	"sync"
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
	// Files & dirs
	VideoDirectory        string   `json:"-" xml:"-"`
	OriginalVideoPath     string   `json:"-" xml:"-"`
	OriginalVideoBaseName string   `json:"-" xml:"-"`
	TempOutputFilePath    string   `json:"-" xml:"-"`
	FinalVideoPath        string   `json:"-" xml:"-"`
	FinalVideoBaseName    string   `json:"-" xml:"-"`
	VideoFile             *os.File `json:"-" xml:"-"`

	// Transformations
	FilenameMetaPrefix string `json:"-" xml:"-"`
	FilenameDateTag    string `json:"-" xml:"-"`
	RenamedVideoPath   string `json:"-" xml:"-"`
	RenamedMetaPath    string `json:"-" xml:"-"`

	// JSON paths
	JSONDirectory string `json:"-" xml:"-"`
	JSONFilePath  string `json:"-" xml:"-"`
	JSONBaseName  string `json:"-" xml:"-"`

	// NFO paths
	NFOBaseName  string `json:"-" xml:"-"`
	NFODirectory string `json:"-" xml:"-"`
	NFOFilePath  string `json:"-" xml:"-"`

	// Metadata
	MCredits   *MetadataCredits     `json:"meta_credits" xml:"credits"`
	MTitleDesc *MetadataTitlesDescs `json:"meta_title_description" xml:"titles"`
	MDates     *MetadataDates       `json:"meta_dates" xml:"dates"`
	MShowData  *MetadataShowData    `json:"meta_show_data" xml:"show"`
	MWebData   *MetadataWebData     `json:"meta_web_data" xml:"web"`
	MOther     *MetadataOtherData   `json:"meta_other_data" xml:"other"`
	NFOData    *NFOData

	// File writers
	JSONFileRW JSONFileRW
	NFOFileRW  NFOFileRW

	// Own transformations
	ModelMAppend     []MetaAppend
	ModelMNewField   []MetaNewField
	ModelMPrefix     []MetaPrefix
	ModelMReplace    []MetaReplace
	ModelMTrimPrefix []MetaTrimPrefix
	ModelMTrimSuffix []MetaTrimSuffix

	ModelFileSfxReplace []FilenameReplaceSuffix

	// Misc
	MetaFileType      enums.MetaFiletype `json:"-" xml:"-"`
	MetaAlreadyExists bool               `json:"-" xml:"-"`
	ModelMOverwrite   bool
}

type Core struct {
	Cleanup chan os.Signal
	Cancel  context.CancelFunc
	Ctx     context.Context
	Wg      *sync.WaitGroup
}
