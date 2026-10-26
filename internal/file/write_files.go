package file

import (
	"errors"
	"fmt"
	"metarr/internal/abstractions"
	"metarr/internal/domain/consts"
	"metarr/internal/domain/enums"
	"metarr/internal/domain/keys"
	"metarr/internal/domain/logger"
	"metarr/internal/models"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/TubarrApp/gocommon/logging"
	"github.com/TubarrApp/gocommon/sharedconsts"
)

// FSFileWriter is a model granting access to file writer functions.
type FSFileWriter struct {
	Fd           *models.FileData
	SkipVids     bool
	RenamedVideo string
	InputVideo   string
	RenamedMeta  string
	InputMeta    string
	OutputDir    string
	muFs         sync.RWMutex
}

// NewFSFileWriter returns a file writer, used for writing changes to filenames etc.
func NewFSFileWriter(fd *models.FileData, skipVids bool, outputDir string) (*FSFileWriter, error) {
	inputVid := fd.PostFFmpegVideoPath
	renamedVid := fd.RenamedVideoPath
	inputMeta := fd.MetaFilePath
	renamedMeta := fd.RenamedMetaPath

	if !skipVids {
		if inputVid == "" && renamedVid == "" {
			return nil, fmt.Errorf("some required video paths are empty:\n\nVid src: %q\nVid dest: %q", inputVid, renamedVid)
		}
	}

	if inputMeta == "" && renamedMeta == "" {
		return nil, fmt.Errorf("some required meta paths are empty:\n\nMeta src: %q\nMeta dest: %q", inputMeta, renamedMeta)
	}

	if logging.Level() > 1 {
		differ := 0
		if !strings.EqualFold(renamedVid, inputVid) {
			differ++
		}
		if !strings.EqualFold(renamedMeta, inputMeta) {
			differ++
		}

		logger.Pl.D(2, "Made FSFileWriter with parameters:\n\nSkip videos? %v\n\nOriginal Video: %s\nRenamed Video:  %s\n\nOriginal Metafile: %s\nRenamed Metafile:  %s\n\n%d file names will be changed...\n\n",
			skipVids, inputVid, renamedVid, inputMeta, renamedMeta, differ)
	}

	return &FSFileWriter{
		Fd:           fd,
		SkipVids:     skipVids,
		RenamedVideo: renamedVid,
		InputVideo:   inputVid,
		RenamedMeta:  renamedMeta,
		InputMeta:    inputMeta,
		OutputDir:    outputDir,
	}, nil
}

// RenameAndMoveFiles moves the video/meta files to their new paths, in place or in the output directory, leaving out the meta
// file if noMeta (i.e. it was purged).
func (fs *FSFileWriter) RenameAndMoveFiles(noMeta bool) error {
	fs.muFs.Lock()
	defer fs.muFs.Unlock()

	if fs.RenamedVideo == "" && fs.RenamedMeta == "" {
		logger.Pl.D(1, "Skipping video or metadata renaming, as the renamed strings are empty")
		return nil
	}

	// Create the output directory if one is set.
	if fs.OutputDir != "" {
		if err := os.MkdirAll(fs.OutputDir, consts.PermsVideoDir); err != nil {
			return fmt.Errorf("failed to create or find destination directory: %w", err)
		}
	}

	// Move video file.
	if shouldProcess(fs.InputVideo, fs.RenamedVideo, true, fs.SkipVids) {
		if err := moveOrCopyFile(fs.InputVideo, fs.RenamedVideo); err != nil {
			return fmt.Errorf("failed to move video file from %q → %q: %w", fs.InputVideo, fs.RenamedVideo, err)
		}
	}

	// Move meta file.
	if !noMeta && shouldProcess(fs.InputMeta, fs.RenamedMeta, false, fs.SkipVids) {
		if err := moveOrCopyFile(fs.InputMeta, fs.RenamedMeta); err != nil {
			return fmt.Errorf("failed to move metadata file from %q → %q: %w", fs.InputMeta, fs.RenamedMeta, err)
		}
	}
	return nil
}

// DeleteMetafile safely removes metadata files once file operations are complete.
func (fs *FSFileWriter) DeleteMetafile(file string) (deleted bool, err error) {
	logger.Pl.I("Attempting to delete metafile %q...", file)

	if !abstractions.IsSet(keys.MetaPurgeEnum) {
		return false, errors.New("meta purge enum not set")
	}

	e, ok := abstractions.Get(keys.MetaPurgeEnum).(enums.PurgeMetafiles)
	if !ok {
		return false, fmt.Errorf("wrong type for purge metafile enum. Got %T", e)
	}

	ext := filepath.Ext(file)
	ext = strings.ToLower(ext)

	switch e {
	case enums.PurgeMetaAll:
		break

	case enums.PurgeMetaJSON:
		if ext != sharedconsts.MExtJSON {
			logger.Pl.D(3, "Skipping deletion of metafile %q as extension does not match user selection", file)
			return false, nil
		}

	case enums.PurgeMetaNFO:
		if ext != sharedconsts.MExtNFO {
			logger.Pl.D(3, "Skipping deletion of metafile %q as extension does not match user selection", file)
			return false, nil
		}

	case enums.PurgeMetaNone: // A blank purge setting.
		return false, nil

	default:
		return false, errors.New("support not added for this metafile purge enum yet, exiting function")
	}

	fileInfo, err := os.Stat(file)
	if err != nil {
		return false, err
	}

	if fileInfo.IsDir() {
		return false, fmt.Errorf("metafile %q is a directory, not a file", file)
	}

	if !fileInfo.Mode().IsRegular() {
		return false, fmt.Errorf("metafile %q is not a regular file", file)
	}

	if err := os.Remove(file); err != nil {
		return false, fmt.Errorf("unable to delete meta file: %w", err)
	}
	logger.Pl.S("Successfully deleted metafile %q", file)

	return true, nil
}
