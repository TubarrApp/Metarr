// Package backup handles the nacking up of files.
package backup

import (
	"fmt"
	"io"
	"metarr/internal/domain/consts"
	"metarr/internal/utils/logging"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

var (
	muBackup sync.Mutex
)

// File creates a backup copy of the original file before modifying it.
func File(file *os.File) error {

	originalFilePath := file.Name()

	backupFilePath := generateBackupFilename(originalFilePath)
	logging.D(3, "Creating backup of file %q as %q", originalFilePath, backupFilePath)

	// Current position
	currentPos, err := file.Seek(0, io.SeekCurrent)
	if err != nil {
		return fmt.Errorf("failed to get current file position: %w", err)
	}
	defer func() {
		if _, err := file.Seek(currentPos, io.SeekStart); err != nil {
			logging.E("Failed to seek file %q: %v", file.Name(), err)
		}
	}()

	muBackup.Lock()
	defer muBackup.Unlock()

	// Seek to start for backup
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return fmt.Errorf("failed to seek to beginning of original file: %w", err)
	}

	// Open the backup file for writing
	backupFile, err := os.Create(backupFilePath)
	if err != nil {
		return fmt.Errorf("failed to create backup file: %w", err)
	}
	defer func() {
		if err := backupFile.Close(); err != nil {
			logging.E("Failed to close file %q: %v", backupFile.Name(), err)
		}
	}()

	// Copy the content of the original file to the backup file
	buf := make([]byte, consts.Buffer4MB)
	_, err = io.CopyBuffer(backupFile, file, buf)
	if err != nil {
		return fmt.Errorf("failed to copy content to backup file: %w", err)
	}

	logging.D(3, "Backup successfully created at %q", backupFilePath)
	return nil
}

// generateBackupFilename creates a backup filename by appending "_backup" to the original filename
func generateBackupFilename(originalFilePath string) string {
	ext := filepath.Ext(originalFilePath)
	base := strings.TrimSuffix(originalFilePath, ext)
	return base + consts.BackupTag + ext
}

// RenameToBackup renames the passed in file
func RenameToBackup(filename string) (backupName string, err error) {

	if filename == "" {
		logging.E("filename was passed in to backup empty")
	}

	backupName = generateBackupFilename(filename)

	if err := os.Rename(filename, backupName); err != nil {
		return "", fmt.Errorf("failed to backup filename %q → %q", filename, backupName)
	}
	return backupName, nil
}
