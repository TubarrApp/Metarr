package utils

import (
	"Metarr/internal/config"
	enums "Metarr/internal/domain/enums"
	keys "Metarr/internal/domain/keys"
	logging "Metarr/internal/utils/logging"
	"bufio"
	"bytes"
	"fmt"
	"io"
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
	same := 0
	if destVideo != srcVideo {
		same++
	}
	if destMeta != srcMeta {
		same++
	}

	logging.PrintD(2, "Made FSFileWriter with parameters:\n\nSkip videos? %v\n\nOriginal Video: %s\nRenamed Video:  %s\n\nOriginal Metafile: %s\nRenamed Metafile:  %s\n\n%d file names will be changed...\n\n",
		skipVids, srcVideo, destVideo, srcMeta, destMeta, same)
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

	if !fs.SkipVids {
		if fs.SrcVideo != fs.DestVideo && fs.SrcVideo != "" && fs.DestVideo != "" {
			if err := os.Rename(fs.SrcVideo, fs.DestVideo); err != nil {
				return fmt.Errorf("failed to rename %s to %s. error: %v", fs.SrcVideo, fs.DestVideo, err)
			}
		}
	}
	if fs.SrcMeta != fs.DestMeta && fs.SrcMeta != "" && fs.DestMeta != "" {
		if err := os.Rename(fs.SrcMeta, fs.DestMeta); err != nil {
			return fmt.Errorf("failed to rename %s to %s. error: %v", fs.SrcMeta, fs.DestMeta, err)
		}
	}

	logging.PrintD(1, "Rename function final commands:\n\nVideo: Replacing '%v' with '%v'\nMetafile: Replacing '%v' with '%v'\n\n", fs.SrcVideo, fs.DestVideo,
		fs.SrcMeta, fs.DestMeta)
	return nil
}

// MoveFile moves files to specified location
func (fs *FSFileWriter) MoveFile(noMeta bool) error {
	fs.muFs.Lock()
	defer fs.muFs.Unlock()

	if !config.IsSet(keys.MoveOnComplete) {
		return nil
	}

	if fs.DestVideo == "" && fs.DestMeta == "" {
		return fmt.Errorf("video and metafile source strings both empty")
	}

	dst := config.GetString(keys.MoveOnComplete)
	dst = filepath.Clean(dst)

	// Check destination directory
	check, err := os.Stat(dst)
	if err != nil {
		return fmt.Errorf("unable to stat destination folder '%s': %w", dst, err)
	}
	if !check.IsDir() {
		return fmt.Errorf("destination path must be a folder: '%s'", dst)
	}

	// Move/copy video and metadata file
	if fs.DestVideo != "" {
		destVBase := filepath.Base(fs.DestVideo)
		destVTarget := filepath.Join(dst, destVBase)
		if err := fs.moveOrCopyFile(fs.DestVideo, destVTarget); err != nil {
			return fmt.Errorf("failed to move video file: %w", err)
		}
	}

	if !noMeta {
		if fs.DestMeta != "" {
			destMBase := filepath.Base(fs.DestMeta)
			destMTarget := filepath.Join(dst, destMBase)
			if err := fs.moveOrCopyFile(fs.DestMeta, destMTarget); err != nil {
				return fmt.Errorf("failed to move metadata file: %w", err)
			}
		}
	}
	return nil
}

// copyFile copies a file to a target destination
func (fs *FSFileWriter) copyFile(src, dst string) error {
	src = filepath.Clean(src)
	dst = filepath.Clean(dst)

	if src == dst {
		return fmt.Errorf("entered source file '%s' and destination '%s' file as the same name and same path", src, dst)
	}

	logging.PrintI("Copying:\n'%s'\nto\n'%s'...", src, dst)

	// Validate source file
	sourceInfo, err := os.Stat(src)
	if err != nil {
		return fmt.Errorf("failed to stat source file: %w", err)
	}
	if !sourceInfo.Mode().IsRegular() {
		return fmt.Errorf("source is not a regular file: %s", src)
	}
	if sourceInfo.Size() == 0 {
		return fmt.Errorf("source file is empty: %s", src)
	}

	// Check destination
	if destInfo, err := os.Stat(dst); err == nil {
		if os.SameFile(sourceInfo, destInfo) {
			return nil // Same file
		}
		return fmt.Errorf("aborting move, destination file '%s' is equal to source file '%s'", dst, src)
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("error checking destination file: %w", err)
	}

	// Ensure destination directory exists
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return fmt.Errorf("failed to create destination directory: %w", err)
	}

	// Open source file
	sourceFile, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("failed to open source file: %w", err)
	}
	defer sourceFile.Close()

	// Create destination file
	destFile, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("failed to create destination file, do you have adequate permissions on the destination folder?: %w", err)
	}
	defer func() {
		destFile.Close()
		if err != nil {
			os.Remove(dst) // Clean up on error
		}
	}()

	// Copy contents with buffer
	bufferedSource := bufio.NewReaderSize(sourceFile, 4*1024*1024) // 4MB: 1024 * 1024 is 1 MB
	bufferedDest := bufio.NewWriterSize(destFile, 4*1024*1024)
	defer bufferedDest.Flush()

	buf := make([]byte, 4*1024*1024)

	if _, err = io.CopyBuffer(bufferedDest, bufferedSource, buf); err != nil {
		return fmt.Errorf("failed to copy file contents: %w", err)
	}

	// Sync to ensure write is complete
	if err = destFile.Sync(); err != nil {
		return fmt.Errorf("failed to sync destination file: %w", err)
	}

	// Set same permissions as source
	if err = os.Chmod(dst, sourceInfo.Mode()); err != nil {
		logging.PrintI("Failed to set file permissions, is destination folder remote? (%v)", err)
	}

	// Verify destination file
	check, err := destFile.Stat()
	if err != nil {
		return fmt.Errorf("error statting destination file: %w", err)
	}
	if check.Size() != sourceInfo.Size() {
		return fmt.Errorf("destination file size (%d) does not match source size (%d)",
			check.Size(), sourceInfo.Size())
	}
	return nil
}

// moveOrCopyFile attempts rename first, falls back to copy+delete for cross-device moves
func (fs *FSFileWriter) moveOrCopyFile(src, dst string) error {
	src = filepath.Clean(src)
	dst = filepath.Clean(dst)

	if src == dst {
		return nil // Same file, nothing to do
	}

	srcHash, err := fs.calculateFileHash(src)
	if err != nil {
		return fmt.Errorf("failed to calculate initial source hash: %w", err)
	}

	// Try rename (pure move) first
	err = os.Rename(src, dst)
	if err == nil {
		dstHash, verifyErr := fs.calculateFileHash(dst)
		if verifyErr != nil {
			return fmt.Errorf("move verification failed: %w", verifyErr)
		}
		if !bytes.Equal(srcHash, dstHash) {
			return fmt.Errorf("hash mismatch after move")
		}
		return nil
	}

	// If cross-device error, fall back to copy+delete
	if strings.Contains(err.Error(), "invalid cross-device link") {
		logging.PrintD(1, "Falling back to copy for moving %s to %s", src, dst)

		// Copy the file
		if err := fs.copyFile(src, dst); err != nil {
			os.Remove(dst)
			return fmt.Errorf("failed to copy file: %w", err)
		}

		// Verify copy with hash comparison
		dstHash, verifyErr := fs.calculateFileHash(dst)
		if verifyErr != nil {
			os.Remove(dst)
			return fmt.Errorf("copy verification failed: %w", verifyErr)
		}
		if !bytes.Equal(srcHash, dstHash) {
			os.Remove(dst)
			return fmt.Errorf("hash mismatch after copy")
		}

		// Remove source after successful copy and verification
		if err := os.Remove(src); err != nil {
			logging.PrintE(0, "Failed to remove source file after verified copy: %v", err)
			// Operation successful, do not return error, just log the error
		}
		return nil
	}
	return fmt.Errorf("failed to move file: %w", err)
}

// DeleteJSON safely removes JSON metadata files once file operations are complete
func (fs *FSFileWriter) DeleteMetafile(file string) (error, bool) {

	if !config.IsSet(keys.MetaPurgeEnum) {
		return fmt.Errorf("meta purge enum not set"), false
	}

	e, ok := config.Get(keys.MetaPurgeEnum).(enums.PurgeMetafiles)
	if !ok {
		return fmt.Errorf("wrong type for purge metafile enum. Got %T", e), false
	}

	ext := filepath.Ext(file)
	ext = strings.ToLower(ext)

	switch e {
	case enums.PURGEMETA_ALL:
		// Continue
	case enums.PURGEMETA_JSON:
		if ext != ".json" {
			logging.PrintD(3, "Skipping deletion of metafile '%s' as extension does not match user selection")
			return nil, false
		}
	case enums.PURGEMETA_NFO:
		if ext != ".nfo" {
			logging.PrintD(3, "Skipping deletion of metafile '%s' as extension does not match user selection")
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
		return fmt.Errorf("metafile '%s' is a directory, not a file", file), false
	}

	if !fileInfo.Mode().IsRegular() {
		return fmt.Errorf("metafile '%s' is not a regular file", file), false
	}

	if err := os.Remove(file); err != nil {
		return fmt.Errorf("unable to delete JSON file: %w", err), false
	}

	logging.PrintS(0, "Successfully deleted metafile. Bye bye '%s'!", file)

	return nil, true
}
