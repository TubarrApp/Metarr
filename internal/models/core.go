// Package models holds structs used throughout the Metarr program.
package models

import (
	"context"
	"fmt"
	"metarr/internal/abstractions"
	"metarr/internal/domain/enums"
	"metarr/internal/domain/keys"
	"sync"
)

// NewFileData generates a new FileData model.
func NewFileData() *FileData {
	fd := &FileData{
		MTitleDesc: &MetadataTitlesDescs{},
		MCredits:   &MetadataCredits{},
		MDates:     &MetadataDates{},
		MShowData:  &MetadataShowData{},
		MWebData:   &MetadataWebData{},
		MOther:     &MetadataOtherData{},
	}
	fd.LoadFilenameReplacements()
	return fd
}

// LoadFilenameReplacements loads filename replacement config into FileData.
func (fd *FileData) LoadFilenameReplacements() {
	if abstractions.IsSet(keys.FilenameReplaceSfx) {
		if result, ok := abstractions.Get(keys.FilenameReplaceSfx).([]FilenameReplaceSuffix); ok {
			fd.FilenameReplaceSuffix = result
		} else {
			fmt.Printf("Invalid type for filename replace suffixes: got %T, expected []FilenameReplaceSuffix", abstractions.Get(keys.FilenameReplaceSfx))
		}
	}

	if abstractions.IsSet(keys.FilenameReplacePfx) {
		if result, ok := abstractions.Get(keys.FilenameReplacePfx).([]FilenameReplacePrefix); ok {
			fd.FilenameReplacePrefix = result
		} else {
			fmt.Printf("Invalid type for filename replace prefixes: got %T, expected []FilenameReplacePrefix", abstractions.Get(keys.FilenameReplacePfx))
		}
	}

	if abstractions.IsSet(keys.FilenameReplaceStr) {
		if result, ok := abstractions.Get(keys.FilenameReplaceStr).([]FilenameReplaceStrings); ok {
			fd.FilenameReplaceStrings = result
		} else {
			fmt.Printf("Invalid type for filename replace strings: got %T, expected []FilenameReplaceStrings", abstractions.Get(keys.FilenameReplaceStr))
		}
	}
}

// FileData contains information about the file and how it should be handled.
type FileData struct {
	// Files & dirs
	VideoDirectory        string `json:"-" xml:"-"`
	OriginalVideoPath     string `json:"-" xml:"-"`
	OriginalVideoBaseName string `json:"-" xml:"-"`
	TempOutputFilePath    string `json:"-" xml:"-"`
	FinalVideoPath        string `json:"-" xml:"-"`
	FinalVideoBaseName    string `json:"-" xml:"-"`

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

	// Meta transformations
	MetaOps *MetaOps

	// File transformations
	FilenameReplaceSuffix  []FilenameReplaceSuffix
	FilenameReplacePrefix  []FilenameReplacePrefix
	FilenameReplaceStrings []FilenameReplaceStrings

	// Misc
	MetaFileType      enums.MetaFiletype `json:"-" xml:"-"`
	MetaAlreadyExists bool               `json:"-" xml:"-"`
	ModelMOverwrite   bool
}

// Core contains variables important to the program core.
type Core struct {
	Cancel context.CancelFunc
	Ctx    context.Context
	Wg     *sync.WaitGroup
}
