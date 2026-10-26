package file

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"metarr/internal/domain/logger"
	"metarr/internal/parsing"
	"os"
	"path/filepath"
	"sync"

	"github.com/TubarrApp/gocommon/sharedconsts"
)

var (
	muBackup sync.Mutex
)

// BackupFile creates a backup copy of the original file before modifying it.
func BackupFile(file *os.File) error {
	originalFilePath := file.Name()

	// Current position.
	currentPos, err := file.Seek(0, io.SeekCurrent)
	if err != nil {
		return fmt.Errorf("failed to get current file position: %w", err)
	}
	defer func() {
		if _, err := file.Seek(currentPos, io.SeekStart); err != nil {
			logger.Pl.E("Failed to seek file %q: %v", file.Name(), err)
		}
	}()

	muBackup.Lock()
	defer muBackup.Unlock()

	backupFilePath, err := nextBackupPath(originalFilePath)
	if err != nil {
		return err
	}
	if backupFilePath == "" {
		logger.Pl.D(3, "Identical backup of %q already exists, skipping", originalFilePath)
		return nil
	}
	logger.Pl.D(3, "Creating backup of file %q as %q", originalFilePath, backupFilePath)

	// Seek to start for backup.
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return fmt.Errorf("failed to seek to beginning of original file: %w", err)
	}

	backupFile, err := os.OpenFile(backupFilePath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o666)
	if err != nil {
		return fmt.Errorf("failed to create backup file: %w", err)
	}
	if _, err := io.Copy(backupFile, file); err != nil {
		_ = backupFile.Close()
		return fmt.Errorf("failed to copy content to backup file: %w", err)
	}
	if err := backupFile.Close(); err != nil {
		return fmt.Errorf("failed to write backup file %q: %w", backupFilePath, err)
	}

	logger.Pl.D(3, "Backup successfully created at %q", backupFilePath)
	return nil
}

// RenameToBackup renames the file to a backup version, returning "" if an identical backup already exists.
func RenameToBackup(filename string) (backupName string, err error) {
	if filename == "" {
		return "", errors.New("filename passed in to backup is empty")
	}

	muBackup.Lock()
	defer muBackup.Unlock()

	if backupName, err = nextBackupPath(filename); err != nil || backupName == "" {
		return "", err
	}
	if err := os.Rename(filename, backupName); err != nil {
		return "", fmt.Errorf("failed to backup filename %q → %q: %w", filename, backupName, err)
	}
	return backupName, nil
}

// nextBackupPath returns the first free backup path for a file ("name_mbackup.ext", then "name_mbackup (2).ext"...).
//
// Returns "" if an existing backup is identical to the file.
func nextBackupPath(originalFilePath string) (string, error) {
	ext := filepath.Ext(originalFilePath)
	base := parsing.GetFilepathWithoutExt(originalFilePath) + sharedconsts.BackupTag

	for n := 1; ; n++ {
		candidate := base + ext
		if n > 1 {
			candidate = fmt.Sprintf("%s (%d)%s", base, n, ext)
		}
		same, err := sameContents(originalFilePath, candidate)
		if errors.Is(err, fs.ErrNotExist) {
			return candidate, nil
		}
		if err != nil {
			return "", err
		}
		if same {
			return "", nil
		}
	}
}

// sameContents reports whether file a holds the same bytes as backup b, returning fs.ErrNotExist if b doesn't exist.
func sameContents(a, b string) (bool, error) {
	bInfo, err := os.Stat(b)
	if err != nil {
		return false, err
	}
	aInfo, err := os.Stat(a)
	if err != nil {
		return false, fmt.Errorf("failed to stat %q: %w", a, err)
	}
	if aInfo.Size() != bInfo.Size() {
		return false, nil
	}
	aHash, err := calculateFileHash(a)
	if err != nil {
		return false, err
	}
	bHash, err := calculateFileHash(b)
	if err != nil {
		return false, err
	}
	return bytes.Equal(aHash, bHash), nil
}
