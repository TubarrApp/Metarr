package utils

import (
	"fmt"
	"metarr/internal/cfg"
	"metarr/internal/domain/consts"
	"metarr/internal/domain/enums"
	"metarr/internal/domain/keys"
	"metarr/internal/utils/logging"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

type FSFileWriter struct {
	SkipVids  bool
	DestVideo string
	SrcVideo  string
	DestMeta  string
	SrcMeta   string
	muFs      sync.RWMutex
}

func NewFSFileWriter(skipVids bool, destVideo, srcVideo, destMeta, srcMeta string) *FSFileWriter {

	if logging.Level > 1 {
		differ := 0
		if !strings.EqualFold(destVideo, srcVideo) {
			differ++
		}
		if !strings.EqualFold(destMeta, srcMeta) {
			differ++
		}

		logging.D(2, "Made FSFileWriter with parameters:\n\nSkip videos? %v\n\nOriginal Video: %s\nRenamed Video:  %s\n\nOriginal Metafile: %s\nRenamed Metafile:  %s\n\n%d file names will be changed...\n\n",
			skipVids, srcVideo, destVideo, srcMeta, destMeta, differ)
	}

	return &FSFileWriter{
		SkipVids:  skipVids,
		DestVideo: destVideo,
		SrcVideo:  srcVideo,
		DestMeta:  destMeta,
		SrcMeta:   srcMeta,
	}
}

// WriteResults executes the final commands to write the transformed files
func (fs *FSFileWriter) WriteResults() error {
	fs.muFs.Lock()
	defer fs.muFs.Unlock()

	// Rename video file
	if shouldProcess(fs.SrcVideo, fs.DestVideo, true, fs.SkipVids) {
		logging.D(1, "Video rename function:\n\nVideo: Replacing '%v' with '%v'", fs.SrcVideo, fs.DestVideo)
		if err := os.Rename(fs.SrcVideo, fs.DestVideo); err != nil {
			return fmt.Errorf("failed to rename %s to %s. error: %v", fs.SrcVideo, fs.DestVideo, err)
		}
	}

	// Rename meta file
	if shouldProcess(fs.SrcMeta, fs.DestMeta, false, fs.SkipVids) {
		logging.D(1, "Rename function final commands:\n\nMetafile: Replacing '%v' with '%v'", fs.SrcMeta, fs.DestMeta)
		if err := os.Rename(fs.SrcMeta, fs.DestMeta); err != nil {
			return fmt.Errorf("failed to rename %s to %s. error: %v", fs.SrcMeta, fs.DestMeta, err)
		}
	}

	return nil
}

// MoveFile moves files to specified location
func (fs *FSFileWriter) MoveFile(noMeta bool) error {
	fs.muFs.Lock()
	defer fs.muFs.Unlock()

	if !cfg.IsSet(keys.MoveOnComplete) {
		return nil
	}

	if fs.DestVideo == "" && fs.DestMeta == "" {
		return fmt.Errorf("video and metafile source strings both empty")
	}

	dst := cfg.GetString(keys.MoveOnComplete)
	dst = filepath.Clean(dst)

	if _, err := os.Stat(dst); os.IsNotExist(err) {
		if err := os.MkdirAll(dst, 0o755); err != nil {
			return fmt.Errorf("failed to create or find destination directory: %w", err)
		}
	}

	// Move/copy video and metadata file
	if !fs.SkipVids {
		if fs.DestVideo != "" {
			destVBase := filepath.Base(fs.DestVideo)
			destVTarget := filepath.Join(dst, destVBase)
			if err := moveOrCopyFile(fs.DestVideo, destVTarget); err != nil {
				return fmt.Errorf("failed to move video file: %w", err)
			}
		}
	}

	if !noMeta {
		if fs.DestMeta != "" {
			destMBase := filepath.Base(fs.DestMeta)
			destMTarget := filepath.Join(dst, destMBase)
			if err := moveOrCopyFile(fs.DestMeta, destMTarget); err != nil {
				return fmt.Errorf("failed to move metadata file: %w", err)
			}
		}
	}
	return nil
}

// DeleteJSON safely removes JSON metadata files once file operations are complete
func (fs *FSFileWriter) DeleteMetafile(file string) (error, bool) {

	if !cfg.IsSet(keys.MetaPurgeEnum) {
		return fmt.Errorf("meta purge enum not set"), false
	}

	e, ok := cfg.Get(keys.MetaPurgeEnum).(enums.PurgeMetafiles)
	if !ok {
		return fmt.Errorf("wrong type for purge metafile enum. Got %T", e), false
	}

	ext := filepath.Ext(file)
	ext = strings.ToLower(ext)

	switch e {
	case enums.PURGEMETA_ALL:
		break

	case enums.PURGEMETA_JSON:
		if ext != consts.MExtJSON {
			logging.D(3, "Skipping deletion of metafile %q as extension does not match user selection", file)
			return nil, false
		}

	case enums.PURGEMETA_NFO:
		if ext != consts.MExtNFO {
			logging.D(3, "Skipping deletion of metafile %q as extension does not match user selection", file)
			return nil, false
		}

	case enums.PURGEMETA_NONE:
		return fmt.Errorf("user selected to skip purging metadata, this should be inaccessible. Exiting function"), false

	default:
		return fmt.Errorf("support not added for this metafile purge enum yet, exiting function"), false
	}

	fileInfo, err := os.Stat(file)
	if err != nil {
		return err, false
	}

	if fileInfo.IsDir() {
		return fmt.Errorf("metafile %q is a directory, not a file", file), false
	}

	if !fileInfo.Mode().IsRegular() {
		return fmt.Errorf("metafile %q is not a regular file", file), false
	}

	if err := os.Remove(file); err != nil {
		return fmt.Errorf("unable to delete meta file: %w", err), false
	}

	logging.S(0, "Successfully deleted metafile. Bye bye %q!", file)

	return nil, true
}
