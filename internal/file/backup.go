package file

import (
	"fmt"
	"io"
	"metarr/internal/domain/consts"
	"metarr/internal/domain/logger"
	"metarr/internal/parsing"
	"os"
	"path/filepath"
	"sync"
)

var (
	muBackup sync.Mutex
)

// BackupFile creates a backup copy of the original file before modifying it.
func BackupFile(file *os.File) error {
	originalFilePath := file.Name()

	backupFilePath := generateBackupFilename(originalFilePath)
	logger.Pl.D(3, "Creating backup of file %q as %q", originalFilePath, backupFilePath)

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

	// Seek to start for backup.
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return fmt.Errorf("failed to seek to beginning of original file: %w", err)
	}

	// Open the backup file for writing.
	backupFile, err := os.Create(backupFilePath)
	if err != nil {
		return fmt.Errorf("failed to create backup file: %w", err)
	}
	defer func() {
		if err := backupFile.Close(); err != nil {
			logger.Pl.E("Failed to close file %q: %v", backupFile.Name(), err)
		}
	}()

	// Copy the content of the original file to the backup file.
	buf := make([]byte, (4 * consts.MB))
	_, err = io.CopyBuffer(backupFile, file, buf)
	if err != nil {
		return fmt.Errorf("failed to copy content to backup file: %w", err)
	}

	logger.Pl.D(3, "Backup successfully created at %q", backupFilePath)
	return nil
}

// generateBackupFilename creates a backup filename by appending "_backup" to the original filename.
func generateBackupFilename(originalFilePath string) string {
	ext := filepath.Ext(originalFilePath)
	base := parsing.GetFilepathWithoutExt(originalFilePath)

	return base + consts.BackupTag + ext
}

// RenameToBackup renames the passed in file to a backup version.
func RenameToBackup(filename string) (backupName string, err error) {
	if filename == "" {
		logger.Pl.E("filename was passed in to backup empty")
	}

	// Get backup name.
	backupName = generateBackupFilename(filename)

	// Rename existing file to backup.
	if err := os.Rename(filename, backupName); err != nil {
		return "", fmt.Errorf("failed to backup filename %q → %q", filename, backupName)
	}
	return backupName, nil
}
