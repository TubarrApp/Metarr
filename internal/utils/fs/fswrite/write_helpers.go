package fswrite

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"fmt"
	"io"
	"metarr/internal/domain/consts"
	"metarr/internal/utils/logging"
	"os"
	"path/filepath"
	"strings"
)

// moveOrCopyFile attempts rename first, falls back to copy+delete for cross-device moves.
func moveOrCopyFile(src, dst string) error {
	src = filepath.Clean(src)
	dst = filepath.Clean(dst)

	if src == dst {
		return nil // Same file, nothing to do
	}

	srcHash, err := calculateFileHash(src)
	if err != nil {
		return fmt.Errorf("failed to calculate initial source hash: %w", err)
	}

	// Try rename (pure move) first
	if err = os.Rename(src, dst); err == nil { // If err IS nil
		dstHash, verifyErr := calculateFileHash(dst)
		if verifyErr != nil {
			return fmt.Errorf("move verification failed: %w", verifyErr)
		}
		if !bytes.Equal(srcHash, dstHash) {
			return fmt.Errorf("hash mismatch after move (src: %d, dest: %d)", len(srcHash), len(dstHash))
		}
		return nil
	}

	logging.S(0, "Moved file: %q → %q", src, dst)

	// If cross-device error, fall back to copy+delete
	if strings.Contains(err.Error(), "invalid cross-device link") {
		logging.D(1, "Falling back to copy for moving %q to %q", src, dst)

		// Copy the file
		if err := copyFile(src, dst); err != nil {
			if err := os.Remove(dst); err != nil {
				logging.E(0, "Failed to remove %q: %v", dst, err)
			}
			return fmt.Errorf("failed to copy file: %w", err)
		}

		// Verify copy with hash comparison
		dstHash, verifyErr := calculateFileHash(dst)
		if verifyErr != nil {
			if err := os.Remove(dst); err != nil {
				logging.E(0, "Failed to remove %q: %v", dst, err)
			}
			return fmt.Errorf("copy verification failed: %w", verifyErr)
		}
		if !bytes.Equal(srcHash, dstHash) {
			if err := os.Remove(dst); err != nil {
				logging.E(0, "Failed to remove %q: %v", dst, err)
			}
			return fmt.Errorf("hash mismatch after copy")
		}

		// Remove source after successful copy and verification
		if err := os.Remove(src); err != nil {
			logging.E(0, "Failed to remove source file after verified copy: %v", err)
		}
		return nil
	}
	return fmt.Errorf("failed to move file: %w", err)
}

// copyFile copies a file to a target destination.
func copyFile(src, dst string) error {
	src = filepath.Clean(src)
	dst = filepath.Clean(dst)

	if src == dst {
		return fmt.Errorf("entered source file %q and destination %q file as the same name and same path", src, dst)
	}

	logging.I("Copying:\n%q\nto\n%q...", src, dst)

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
		return fmt.Errorf("aborting move, destination file %q is equal to source file %q", dst, src)
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("error checking destination file: %w", err)
	}

	// Ensure destination directory exists
	if _, err := os.Stat(dst); os.IsNotExist(err) {
		if err := os.MkdirAll(dst, 0o755); err != nil {
			return fmt.Errorf("failed to create or find destination directory: %w", err)
		}
	}

	// Open source file
	sourceFile, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("failed to open source file: %w", err)
	}
	defer func() {
		if err := sourceFile.Close(); err != nil {
			logging.E(0, "Failed to close %q: %v", sourceFile.Name(), err)
		}
	}()

	// Create destination file
	destFile, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("failed to create destination file, do you have adequate permissions on the destination folder?: %w", err)
	}
	// Cleanup on function exit
	defer func() {
		if err := destFile.Close(); err != nil {
			logging.E(0, "Failed to close %q: %v", sourceFile.Name(), err)
		}
		if err != nil {
			if err := os.Remove(dst); err != nil {
				logging.E(0, "Failed to remove %q: %v", dst, err)
			}
		}
	}()

	// Copy contents with buffer
	bufferedSource := bufio.NewReaderSize(sourceFile, consts.Buffer4MB)
	bufferedDest := bufio.NewWriterSize(destFile, consts.Buffer4MB)
	defer func() {
		if err := bufferedDest.Flush(); err != nil {
			logging.E(0, "failed to flush buffer for %q: %v", destFile.Name(), err)
		}
	}()

	buf := make([]byte, consts.Buffer4MB)

	if _, err = io.CopyBuffer(bufferedDest, bufferedSource, buf); err != nil {
		return fmt.Errorf("failed to copy file contents: %w", err)
	}

	// Sync to ensure write is complete
	if err = destFile.Sync(); err != nil {
		return fmt.Errorf("failed to sync destination file: %w", err)
	}

	// Set same permissions as source
	if err = os.Chmod(dst, sourceInfo.Mode()); err != nil {
		logging.I("Failed to set file permissions, is destination folder remote? (%v)", err)
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

// shouldProcess determines if the file move/rename should be processed.
func shouldProcess(src, dst string, isVid, skipVids bool) bool {
	switch {
	case skipVids && isVid:
		logging.I("Not processing video files. Skip vids is %v", skipVids)
		return false

	case strings.EqualFold(src, dst):
		logging.I("Not processing files. Source and destination match: Src: %v, Dest %v", src, dst)
		return false

	case src == "", dst == "":
		logging.I("Not processing files. Source or destination path empty: Src: %v, Dest %v", src, dst)
		return false

	default:
		logging.I("Processing file operations for %q", src)
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
		if err := file.Close(); err != nil {
			logging.E(0, "Failed to close %q: %v", file.Name(), err)
		}
	}()

	hash := sha256.New()
	buf := make([]byte, consts.Buffer4MB)
	reader := bufio.NewReaderSize(file, consts.Buffer4MB)

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
