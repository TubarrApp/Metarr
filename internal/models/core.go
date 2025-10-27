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

	// Filename Ops
	if abstractions.IsSet(keys.FilenameOpsModels) {
		if fOps, ok := abstractions.Get(keys.FilenameOpsModels).(*FilenameOps); ok {
			fd.FilenameOps = fOps
		} else {
			fmt.Printf("Failed to retrieve FilenameOps from abstractions, got type %T", abstractions.Get(keys.FilenameOpsModels))
		}
	}
	fd.EnsureFilenameOps()

	// Meta Ops
	if abstractions.IsSet(keys.MetaOpsModels) {
		if mOps, ok := abstractions.Get(keys.MetaOpsModels).(*MetaOps); ok {
			fd.MetaOps = mOps
		} else {
			fmt.Printf("Failed to retrieve MetaOps from abstractions, got type %T", abstractions.Get(keys.MetaOpsModels))
		}
	}
	fd.EnsureMetaOps()

	return fd
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
	FilenameOps *FilenameOps

	// Misc
	MetaFileType      enums.MetaFiletype `json:"-" xml:"-"`
	MetaAlreadyExists bool               `json:"-" xml:"-"`
	ModelMOverwrite   bool
}

// Core contains variables important to the program core.
type Core struct {
	Ctx context.Context
	Wg  *sync.WaitGroup
}
