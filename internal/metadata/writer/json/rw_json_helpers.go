package metadata

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
)

// writeJsonToFile is a private metadata writing helper function
func (rw *JSONFileRW) writeJsonToFile(file *os.File, data map[string]interface{}) error {

	if file == nil {
		return fmt.Errorf("nil file handle provided")
	}
	if data == nil {
		return fmt.Errorf("nil data provided")
	}

	rw.muFileWrite.Lock()
	defer rw.muFileWrite.Unlock()

	// Marshal data
	updatedFileContent, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal updated JSON: %w", err)
	}

	// Seek file start
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return fmt.Errorf("failed to seek to beginning of file: %w", err)
	}

	// File ops
	if err := file.Truncate(0); err != nil {
		return fmt.Errorf("failed to truncate file: %w", err)
	}

	if _, err := file.Write(updatedFileContent); err != nil {
		return fmt.Errorf("failed to write to file: %w", err)
	}

	// Ensure changes are persisted
	if err := file.Sync(); err != nil {
		return fmt.Errorf("failed to sync file: %w", err)
	}

	return nil
}

// cleanFieldValue trims leading/trailing whitespaces after deletions
func cleanFieldValue(value string) string {
	cleaned := strings.TrimSpace(value)
	cleaned = strings.Join(strings.Fields(cleaned), " ")
	return cleaned
}

// copyMeta creates a deep copy of the metadata map under read lock
func (rw *JSONFileRW) copyMeta() map[string]interface{} {
	rw.mu.RLock()
	defer rw.mu.RUnlock()

	if rw.Meta == nil {
		return make(map[string]interface{})
	}

	currentMeta := make(map[string]interface{}, len(rw.Meta))
	for k, v := range rw.Meta {
		currentMeta[k] = v
	}
	return currentMeta
}

// updateMeta safely updates the metadata map under write lock
func (rw *JSONFileRW) updateMeta(newMeta map[string]interface{}) {
	if newMeta == nil {
		newMeta = make(map[string]interface{})
	}

	rw.mu.Lock()
	defer rw.mu.Unlock()
	rw.Meta = newMeta
}
