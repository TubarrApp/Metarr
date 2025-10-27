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
	if abstractions.IsSet(keys.FilenameOpsModels) {
		if fOps, ok := abstractions.Get(keys.FilenameOpsModels).(*FilenameOps); ok {
			fd.FilenameOps = fOps
		} else {
			fmt.Printf("Failed to retrieve FilenameOps from abstractions, got type %T", abstractions.Get(keys.FilenameOpsModels))
		}
	}
	if abstractions.IsSet(keys.MetaOpsModels) {
		if mOps, ok := abstractions.Get(keys.MetaOpsModels).(*MetaOps); ok {
			fd.MetaOps = mOps
		} else {
			fmt.Printf("Failed to retrieve MetaOps from abstractions, got type %T", abstractions.Get(keys.MetaOpsModels))
		}
	}
	fd.EnsureFilenameOps()
	fd.EnsureMetaOps()
	return fd
}

// EnsureFilenameOps initializes nil filename operation structure values.
func (fd *FileData) EnsureFilenameOps() {
	if fd.FilenameOps == nil {
		fd.FilenameOps = &FilenameOps{}
	}
	if fd.FilenameOps.Appends == nil {
		fd.FilenameOps.Appends = []FOpAppend{}
	}
	if fd.FilenameOps.Prefixes == nil {
		fd.FilenameOps.Prefixes = []FOpPrefix{}
	}
	if fd.FilenameOps.Replaces == nil {
		fd.FilenameOps.Replaces = []FOpReplace{}
	}
	if fd.FilenameOps.ReplacePrefixes == nil {
		fd.FilenameOps.ReplacePrefixes = []FOpReplacePrefix{}
	}
	if fd.FilenameOps.ReplaceSuffixes == nil {
		fd.FilenameOps.ReplaceSuffixes = []FOpReplaceSuffix{}
	}
}

// EnsureMetaOps initializes nil metadata operation structure values.
func (fd *FileData) EnsureMetaOps() {
	if fd.MetaOps == nil {
		fd.MetaOps = &MetaOps{}
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
		fd.MetaOps.DeleteDateTags = make(map[string]MetaDateTag, 0)
	}
	if fd.MetaOps.NewFields == nil {
		fd.MetaOps.NewFields = []MetaNewField{}
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
	if fd.MetaOps.TrimSuffixes == nil {
		fd.MetaOps.TrimSuffixes = []MetaTrimSuffix{}
	}
	if fd.MetaOps.TrimPrefixes == nil {
		fd.MetaOps.TrimPrefixes = []MetaTrimPrefix{}
	}
	if fd.MetaOps.CopyToFields == nil {
		fd.MetaOps.CopyToFields = []CopyToField{}
	}
	if fd.MetaOps.PasteFromFields == nil {
		fd.MetaOps.PasteFromFields = []PasteFromField{}
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
