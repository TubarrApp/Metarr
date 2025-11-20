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
	"metarr/internal/parsing"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/TubarrApp/gocommon/logging"
)

// FSFileWriter is a model granting access to file writer functions.
type FSFileWriter struct {
	Fd           *models.FileData
	SkipVids     bool
	RenamedVideo string
	InputVideo   string
	RenamedMeta  string
	InputMeta    string
	muFs         sync.RWMutex
}

// NewFSFileWriter returns a file writer, used for writing changes to filenames etc.
func NewFSFileWriter(fd *models.FileData, skipVids bool) (*FSFileWriter, error) {
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

	if logging.Level > 1 {
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
	}, nil
}

// RenameFiles calls os.Rename on the video/meta files.
func (fs *FSFileWriter) RenameFiles() error {
	fs.muFs.Lock()
	defer fs.muFs.Unlock()

	// Rename video file.
	if shouldProcess(fs.InputVideo, fs.RenamedVideo, true, fs.SkipVids) {
		if err := os.Rename(fs.InputVideo, fs.RenamedVideo); err != nil {
			return fmt.Errorf("failed to rename %s → %s. error: %w", fs.InputVideo, fs.RenamedVideo, err)
		}
		logger.Pl.S("Renamed: %q → %q", fs.InputVideo, fs.RenamedVideo)
		fs.Fd.RenamedVideoPath = fs.RenamedVideo
	}

	// Rename meta file.
	if shouldProcess(fs.InputMeta, fs.RenamedMeta, false, fs.SkipVids) {
		if err := os.Rename(fs.InputMeta, fs.RenamedMeta); err != nil {
			return fmt.Errorf("failed to rename %s → %s. error: %w", fs.InputMeta, fs.RenamedMeta, err)
		}
		logger.Pl.S("Renamed: %q → %q", fs.InputMeta, fs.RenamedMeta)
		fs.Fd.RenamedMetaPath = fs.RenamedMeta
	}

	return nil
}

// MoveFile moves files to specified location.
func (fs *FSFileWriter) MoveFile(noMeta bool) error {
	fs.muFs.Lock()
	defer fs.muFs.Unlock()

	if !abstractions.IsSet(keys.OutputDirectory) {
		return nil
	}

	if fs.RenamedVideo == "" && fs.RenamedMeta == "" {
		logger.Pl.D(1, "Skipping video or metadata renaming, as the renamed strings are empty")
		return nil
	}

	dstIn := abstractions.GetString(keys.OutputDirectory)

	prs := parsing.NewDirectoryParser(fs.Fd)
	dst, err := prs.ParseDirectory(dstIn)
	if err != nil {
		return err
	}

	if _, err := os.Stat(dst); os.IsNotExist(err) {
		if err := os.MkdirAll(dst, 0o755); err != nil {
			return fmt.Errorf("failed to create or find destination directory: %w", err)
		}
	}

	// Move or copy video.
	if !fs.SkipVids {
		if fs.RenamedVideo != "" {
			videoDestPath := filepath.Join(dst, filepath.Base(fs.RenamedVideo))
			if err := moveOrCopyFile(fs.RenamedVideo, videoDestPath); err != nil {
				return fmt.Errorf("failed to move video file from %q → %q: %w", fs.RenamedVideo, videoDestPath, err)
			}
		}
	}

	// Move or copy metadata file.
	if !noMeta {
		if fs.RenamedMeta != "" {
			metaDestPath := filepath.Join(dst, filepath.Base(fs.RenamedMeta))
			if err := moveOrCopyFile(fs.RenamedMeta, metaDestPath); err != nil {
				return fmt.Errorf("failed to move metadata file from %q → %q: %w", fs.RenamedMeta, metaDestPath, err)
			}
		}
	}
	return nil
}

// DeleteMetafile safely removes metadata files once file operations are complete.
func (fs *FSFileWriter) DeleteMetafile(file string) (deleted bool, err error) {
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
		if ext != consts.MExtJSON {
			logger.Pl.D(3, "Skipping deletion of metafile %q as extension does not match user selection", file)
			return false, nil
		}

	case enums.PurgeMetaNFO:
		if ext != consts.MExtNFO {
			logger.Pl.D(3, "Skipping deletion of metafile %q as extension does not match user selection", file)
			return false, nil
		}

	case enums.PurgeMetaNone:
		return false, errors.New("user selected to skip purging metadata, this should be inaccessible. Exiting function")

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
