package file

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"fmt"
	"io"
	"metarr/internal/domain/consts"
	"metarr/internal/domain/logger"
	"os"
	"path/filepath"
	"strings"
)

// moveOrCopyFile attempts rename first, falls back to copy+delete for cross-device moves.
func moveOrCopyFile(src, dst string) error {
	src = filepath.Clean(src)
	dst = filepath.Clean(dst)

	if src == dst {
		return nil // Same file, nothing to do.
	}

	var srcHashErr error

	// Store original file hash.
	srcHash, err := calculateFileHash(src)
	if err != nil {
		srcHashErr = err
	}

	// Try rename (pure move) first.
	if err = os.Rename(src, dst); err == nil { // If err IS nil (move ("rename") succeeded).

		// Return with warning if no source hash calc.
		if srcHashErr != nil {
			logger.Pl.W("Unable to calculate initial file hash due to error (%v)... Skipping move hash checks.\n\nAttempted move: %q → %q", srcHashErr, src, dst)
			return nil
		}

		// Calculate destination file hash.
		dstHash, dstHashErr := calculateFileHash(dst)

		// Failed to get destination file hash.
		if dstHashErr != nil {
			logger.Pl.W("Failed to calculate destination file hash, move may or may not have failed: %v\n\nAttempted move: %q → %q", dstHashErr, src, dst)
			return nil
		}

		// Hash comparison.
		if !bytes.Equal(srcHash, dstHash) { // Hash mismatch (FAIL).

			err = fmt.Errorf("hash mismatch (source: %x, destination: %x)\n\nAttempted move %q → %q", srcHash, dstHash, src, dst)

			if delErr := os.Remove(dst); delErr != nil && !os.IsNotExist(delErr) {
				logger.Pl.E("Unable to remove failed moved file %q due to error: %v", dst, delErr)
			}
			// Do not return here, program will continue and attempt a copy.

		} else { // Hash match (SUCCESS).
			logger.Pl.S("Moved file: %q → %q", src, dst)
			return nil
		}
	}

	// removed wrapper: "if strings.Contains(err.Error(), "invalid cross-device link")"
	// around the following block... Successful move should return nil above.

	logger.Pl.E("Move error: %v\n\nAttempting to copy %q to %q instead...", err, src, dst)

	// Copy the file
	if err := copyFile(src, dst); err == nil { // If err IS nil (copy succeeded).

		// Return with warning if no source hash calc.
		if srcHashErr != nil {
			logger.Pl.W("Unable to calculate initial file hash due to error (%v)... Skipping copy hash checks.\n\nAttempted copy: %q → %q", srcHashErr, src, dst)
			return nil
		}

		// Verify copy with hash comparison.
		dstHash, copyDstHashErr := calculateFileHash(dst)

		// Failed to verify destination file hash.
		if copyDstHashErr != nil {
			logger.Pl.W("Failed to calculate destination file hash, copy may or may not have failed: %v\n\nAttempted copy: %q → %q", copyDstHashErr, src, dst)
			return nil
		}

		// Hash comparison.
		if !bytes.Equal(srcHash, dstHash) { // Hash mismatch (FAIL).
			if delErr := os.Remove(dst); delErr != nil && !os.IsNotExist(delErr) {
				logger.Pl.E("Unable to remove failed moved file %q due to error: %v", dst, delErr)
			}
			return fmt.Errorf("hash mismatch after copy (source: %x, destination: %x)\n\nAttempted copy %q → %q", srcHash, dstHash, src, dst)
		}

		// Else hash match (SUCCESS).

		// Remove source after successful copy and verification.
		if err := os.Remove(src); err != nil {
			logger.Pl.W("Failed to remove source file after verified copy due to error: %v", err)
			// Do not return error, user will simply need to manually delete the original.
		}

		logger.Pl.S("Copied file and removed original: %q → %q", src, dst)
		return nil
	}

	if err := os.Remove(dst); err != nil {
		logger.Pl.E("Failed to remove failed copied file %q due to error: %v", dst, err)
	}
	return fmt.Errorf("failed to copy file %q → %q: %w", src, dst, err)
}

// copyFile copies a file to a target destination.
func copyFile(src, dst string) error {
	src = filepath.Clean(src)
	dst = filepath.Clean(dst)

	if src == dst {
		return fmt.Errorf("entered source file %q and destination %q file as the same name and same path", src, dst)
	}

	logger.Pl.I("Copying:\n%q\nto\n%q...", src, dst)

	// Validate source file.
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

	// Check destination.
	if destInfo, err := os.Stat(dst); err == nil {
		if os.SameFile(sourceInfo, destInfo) {
			return nil // Same file.
		}
		return fmt.Errorf("aborting move, destination file %q is equal to source file %q", dst, src)
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("error checking destination file: %w", err)
	}

	// Ensure destination directory exists.
	if _, err := os.Stat(dst); os.IsNotExist(err) {
		if err := os.MkdirAll(dst, 0o755); err != nil {
			return fmt.Errorf("failed to create or find destination directory: %w", err)
		}
	}

	// Open source file.
	sourceFile, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("failed to open source file: %w", err)
	}
	defer func() {
		if err := sourceFile.Close(); err != nil {
			logger.Pl.E("Failed to close %q: %v", sourceFile.Name(), err)
		}
	}()

	// Create destination file.
	destFile, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("failed to create destination file, do you have adequate permissions on the destination folder?: %w", err)
	}
	// Cleanup on function exit.
	defer func() {
		if err := destFile.Close(); err != nil {
			logger.Pl.E("Failed to close %q: %v", sourceFile.Name(), err)
		}
		if err != nil {
			if err := os.Remove(dst); err != nil {
				logger.Pl.E("Failed to remove %q: %v", dst, err)
			}
		}
	}()

	// Copy contents with buffer.
	bufferedSource := bufio.NewReaderSize(sourceFile, (4 * consts.MB))
	bufferedDest := bufio.NewWriterSize(destFile, (4 * consts.MB))
	defer func() {
		if err := bufferedDest.Flush(); err != nil {
			logger.Pl.E("failed to flush buffer for %q: %v", destFile.Name(), err)
		}
	}()
	buf := make([]byte, (4 * consts.MB))

	if _, err = io.CopyBuffer(bufferedDest, bufferedSource, buf); err != nil {
		return fmt.Errorf("failed to copy file contents: %w", err)
	}

	// Sync to ensure write is complete.
	if err = destFile.Sync(); err != nil {
		return fmt.Errorf("failed to sync destination file: %w", err)
	}

	// Set same permissions as source.
	if err = os.Chmod(dst, sourceInfo.Mode()); err != nil {
		logger.Pl.I("Failed to set file permissions, is destination folder remote? (%v)", err)
	}

	// Verify destination file.
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

// shouldProcess determines if the file move/rename should be processed.
func shouldProcess(src, dst string, isVid, skipVids bool) bool {
	switch {
	case skipVids && isVid:
		logger.Pl.I("Not moving or renaming video files. Skip vids is %v", skipVids)
		return false

	case strings.EqualFold(src, dst):
		logger.Pl.I("Not moving or renaming files. Source and destination match: Src: %q, Dest: %q", src, dst)
		return false

	case src == "", dst == "":
		logger.Pl.I("Not moving or renaming files. Source or destination path empty: Src: %q, Dest: %q", src, dst)
		return false

	default:
		logger.Pl.I("Conducting move/rename file operations for %q", src)
		return true
	}
}

// calculateFileHash computes SHA-256 hash of a file.
func calculateFileHash(fpath string) ([]byte, error) {
	file, err := os.Open(fpath)
	if err != nil {
		return nil, fmt.Errorf("failed to open file for hashing: %w", err)
	}
	defer func() {
		if closeErr := file.Close(); closeErr != nil {
			logger.Pl.E("Failed to close %q: %v", file.Name(), closeErr)
		}
	}()

	hash := sha256.New()
	buf := make([]byte, (4 * consts.MB))
	reader := bufio.NewReaderSize(file, (4 * consts.MB))

	for {
		n, err := reader.Read(buf)
		if n > 0 {
			if _, err := hash.Write(buf[:n]); err != nil {
				return nil, fmt.Errorf("error writing to hash: %w", err)
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("error reading file for hash: %w", err)
		}
	}
	return hash.Sum(nil), nil
}
