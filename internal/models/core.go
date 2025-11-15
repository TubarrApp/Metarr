// Package models holds structs used throughout the Metarr program.
package models

import (
	"context"
	"fmt"
	"metarr/internal/abstractions"
	"metarr/internal/domain/keys"
	"os"
	"path/filepath"
	"strings"
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
			fmt.Fprintf(os.Stderr, "Failed to retrieve FilenameOps from abstractions, got type %T", abstractions.Get(keys.FilenameOpsModels))
		}
	}
	fd.EnsureFilenameOps()

	// Meta Ops
	if abstractions.IsSet(keys.MetaOpsModels) {
		if mOps, ok := abstractions.Get(keys.MetaOpsModels).(*MetaOps); ok {
			fd.MetaOps = mOps
		} else {
			fmt.Fprintf(os.Stderr, "Failed to retrieve MetaOps from abstractions, got type %T", abstractions.Get(keys.MetaOpsModels))
		}
	}
	fd.EnsureMetaOps()

	return fd
}

// FileData contains information about the file and how it should be handled.
type FileData struct {
	// Files & dirs
	VideoDirectory      string `json:"-" xml:"-"`
	OriginalVideoPath   string `json:"-" xml:"-"`
	PostFFmpegVideoPath string `json:"-" xml:"-"` // Video path after FFmpeg processing but before renaming

	// Transformations
	FilenameDateTag  string `json:"-" xml:"-"`
	RenamedVideoPath string `json:"-" xml:"-"`
	RenamedMetaPath  string `json:"-" xml:"-"`

	// Final paths (set only at the final boundary after all operations complete)
	FinalVideoPath string `json:"-" xml:"-"` // True final video path after all transformations
	FinalMetaPath  string `json:"-" xml:"-"` // True final metadata path after all transformations

	// Metafile paths
	MetaDirectory string `json:"-" xml:"-"`
	MetaFilePath  string `json:"-" xml:"-"`
	MetaFileType  string `json:"-" xml:"-"`

	// Metadata
	MCredits   *MetadataCredits     `json:"meta_credits" xml:"credits"`
	MTitleDesc *MetadataTitlesDescs `json:"meta_title_description" xml:"titles"`
	MDates     *MetadataDates       `json:"meta_dates" xml:"dates"`
	MShowData  *MetadataShowData    `json:"meta_show_data" xml:"show"`
	MWebData   *MetadataWebData     `json:"meta_web_data" xml:"web"`
	MOther     *MetadataOtherData   `json:"meta_other_data" xml:"other"`
	NFOData    *NFOData

	// Meta transformations
	MetaOps *MetaOps

	// File transformations
	FilenameOps *FilenameOps

	// Misc
	MetaAlreadyExists    bool `json:"-" xml:"-"`
	ModelMOverwrite      bool
	HasEmbeddedThumbnail bool
}

// SetFinalPaths sets the final video and metadata paths after all transformations are complete.
func (fd *FileData) SetFinalPaths(videoPath, metaPath string) {
	fd.FinalVideoPath = videoPath
	fd.FinalMetaPath = metaPath

	fmt.Fprintf(os.Stdout, "final video path: %s\n", videoPath)
	fmt.Fprintf(os.Stdout, "final json path: %s", metaPath)
	fmt.Fprintf(os.Stderr, "\n")
}

// GetBaseNameWithoutExt returns the base name (without extension) of any file path.
func (fd *FileData) GetBaseNameWithoutExt(path string) string {
	if path == "" {
		return ""
	}
	base := filepath.Base(path)
	return strings.TrimSuffix(base, filepath.Ext(base))
}

// Core contains variables important to the program core.
type Core struct {
	Ctx context.Context
	Wg  *sync.WaitGroup
}

// BatchPairs contains directories and files from a batch entry.
type BatchPairs struct {
	VideoDirs,
	VideoFiles,
	MetaDirs,
	MetaFiles []string
}
