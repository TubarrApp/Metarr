package utils

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

// createBackup creates a backup copy of the original file before modifying it.
func BackupFile(file *os.File) error {

	originalFilePath := file.Name()

	backupFilePath := generateBackupFilename(originalFilePath)
	logging.D(3, "Creating backup of file %q as %q", originalFilePath, backupFilePath)

	// Current position
	currentPos, err := file.Seek(0, io.SeekCurrent)
	if err != nil {
		return fmt.Errorf("failed to get current file position: %w", err)
	}
	defer func() {
		file.Seek(currentPos, io.SeekStart)
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
	defer backupFile.Close()

	// Copy the content of the original file to the backup file
	buf := make([]byte, 4*1024*1024)
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
	return fmt.Sprintf(base + consts.BackupTag + ext)
}

// RenameToBackup renames the passed in file
func RenameToBackup(filename string) (backupName string, err error) {

	if filename == "" {
		logging.E(0, "filename was passed in to backup empty")
	}

	backupName = generateBackupFilename(filename)

	if err := os.Rename(filename, backupName); err != nil {
		return "", fmt.Errorf("failed to backup filename %q → %q", filename, backupName)
	}
	return backupName, nil
}
