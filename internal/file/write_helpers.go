package file

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"metarr/internal/domain/consts"
	"metarr/internal/domain/logger"
	"os"
	"path/filepath"
	"syscall"
)

// moveOrCopyFile renames src to dst, or copies and removes src when they are on different filesystems.
func moveOrCopyFile(src, dst string) error {
	src = filepath.Clean(src)
	dst = filepath.Clean(dst)

	if src == dst {
		return nil
	}
	if err := checkFreeDestination(src, dst); err != nil {
		return err
	}

	err := os.Rename(src, dst)
	if err == nil {
		logger.Pl.S("Moved file: %q → %q", src, dst)
		return nil
	}
	if !errors.Is(err, syscall.EXDEV) {
		return fmt.Errorf("failed to move %q → %q: %w", src, dst, err)
	}

	logger.Pl.I("%q and %q are on different filesystems, copying instead...", src, dst)
	if err := copyFile(src, dst); err != nil {
		return fmt.Errorf("failed to copy file %q → %q: %w", src, dst, err)
	}
	if err := os.Remove(src); err != nil {
		logger.Pl.W("Failed to remove source file after verified copy due to error: %v", err)
	}

	logger.Pl.S("Copied file and removed original: %q → %q", src, dst)
	return nil
}

// checkFreeDestination returns an error if dst exists and is not src itself (e.g. a case-only rename on a case-insensitive filesystem).
func checkFreeDestination(src, dst string) error {
	dstInfo, err := os.Lstat(dst)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("failed to check destination %q: %w", dst, err)
	}
	if srcInfo, err := os.Lstat(src); err == nil && os.SameFile(srcInfo, dstInfo) {
		return nil
	}
	return fmt.Errorf("destination %q already exists, not overwriting it", dst)
}

// copyFile copies src to a new file at dst, verifying the copy against the source's hash.
func copyFile(src, dst string) (err error) {
	logger.Pl.I("Copying:\n%q\nto\n%q...", src, dst)

	sourceFile, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("failed to open source file: %w", err)
	}
	defer func() {
		if err := sourceFile.Close(); err != nil {
			logger.Pl.E("Failed to close %q: %v", sourceFile.Name(), err)
		}
	}()

	sourceInfo, err := sourceFile.Stat()
	if err != nil {
		return fmt.Errorf("failed to stat source file: %w", err)
	}
	if !sourceInfo.Mode().IsRegular() {
		return fmt.Errorf("source is not a regular file: %s", src)
	}
	if sourceInfo.Size() == 0 {
		return fmt.Errorf("source file is empty: %s", src)
	}

	if err := os.MkdirAll(filepath.Dir(dst), consts.PermsGenericDir); err != nil {
		return fmt.Errorf("failed to create or find destination directory: %w", err)
	}
	destFile, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_EXCL, sourceInfo.Mode().Perm())
	if err != nil {
		return fmt.Errorf("failed to create destination file: %w", err)
	}
	defer func() {
		if err == nil {
			return
		}
		_ = destFile.Close()
		if rmErr := os.Remove(dst); rmErr != nil {
			logger.Pl.E("Failed to remove incomplete copy %q: %v", dst, rmErr)
		}
	}()

	srcHash := sha256.New()
	if _, err = io.Copy(io.MultiWriter(destFile, srcHash), sourceFile); err != nil {
		return fmt.Errorf("failed to copy file contents: %w", err)
	}
	if err = destFile.Sync(); err != nil {
		return fmt.Errorf("failed to sync destination file: %w", err)
	}
	if err = destFile.Close(); err != nil {
		return fmt.Errorf("failed to write destination file: %w", err)
	}
	if chmodErr := os.Chmod(dst, sourceInfo.Mode()); chmodErr != nil {
		logger.Pl.I("Failed to set file permissions, is destination folder remote? (%v)", chmodErr)
	}

	dstHash, err := calculateFileHash(dst)
	if err != nil {
		return err
	}
	if !bytes.Equal(srcHash.Sum(nil), dstHash) {
		return fmt.Errorf("hash mismatch after copy (source: %x, destination: %x)", srcHash.Sum(nil), dstHash)
	}
	return nil
}

// shouldProcess determines if the file move/rename should be processed.
func shouldProcess(src, dst string, isVid, skipVids bool) bool {
	switch {
	case skipVids && isVid:
		logger.Pl.I("Not moving or renaming video files. Skip vids is %v", skipVids)
		return false

	case src == dst:
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

// ReplaceContents atomically replaces an open file's contents via a renamed temp file, then reopens f onto the new file.
func ReplaceContents(f *os.File, data []byte) error {
	path := f.Name()
	info, err := f.Stat()
	if err != nil {
		return fmt.Errorf("failed to stat %q: %w", path, err)
	}

	tmp, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".*.tmp")
	if err != nil {
		return fmt.Errorf("failed to create temp file for %q: %w", path, err)
	}
	defer func() { _ = os.Remove(tmp.Name()) }() // No-op once renamed.

	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("failed to write temp file for %q: %w", path, err)
	}
	if err := tmp.Chmod(info.Mode().Perm()); err != nil {
		logger.Pl.W("Failed to copy permissions onto %q: %v", path, err)
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("failed to sync temp file for %q: %w", path, err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("failed to write temp file for %q: %w", path, err)
	}
	if err := os.Rename(tmp.Name(), path); err != nil {
		return fmt.Errorf("failed to replace %q: %w", path, err)
	}

	reopened, err := os.OpenFile(path, os.O_RDWR, 0)
	if err != nil {
		return fmt.Errorf("failed to reopen %q: %w", path, err)
	}
	old := *f
	*f = *reopened
	if err := old.Close(); err != nil {
		logger.Pl.W("Failed to close replaced file %q: %v", path, err)
	}
	return nil
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
	if _, err := io.Copy(hash, file); err != nil {
		return nil, fmt.Errorf("error reading file for hash: %w", err)
	}
	return hash.Sum(nil), nil
}
