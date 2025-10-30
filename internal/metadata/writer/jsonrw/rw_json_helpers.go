package jsonrw

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"metarr/internal/utils/logging"
	"os"
	"strings"
	"sync"
)

// Map buffer.
var metaMapPool = sync.Pool{
	New: func() any {
		return make(map[string]any, 81) // 81 objects in tested JSON file received from yt-dlp
	},
}

// JSON pool buffer.
var jsonBufferPool = sync.Pool{
	New: func() any {
		return bytes.NewBuffer(make([]byte, 0, 4096)) // i.e. 4KiB
	},
}

// writeJSONToFile is a private metadata writing helper function.
func (rw *JSONFileRW) writeJSONToFile(file *os.File, j map[string]any) error {
	if file == nil {
		return errors.New("file passed in nil")
	}
	if j == nil {
		return errors.New("JSON metadata passed in nil")
	}

	// Get buffer from pool
	buf := jsonBufferPool.Get().(*bytes.Buffer)
	buf.Reset()
	defer jsonBufferPool.Put(buf)

	// Create encoder each time (cheap operation)
	encoder := json.NewEncoder(buf)
	encoder.SetIndent("", "  ")

	// Marshal data
	if err := encoder.Encode(j); err != nil {
		return fmt.Errorf("marshal error: %w", err)
	}

	// Begin file ops
	rw.mu.Lock()
	defer rw.mu.Unlock()

	currentPos, err := file.Seek(0, io.SeekCurrent)
	if err != nil {
		return fmt.Errorf("failed to get current position: %w", err)
	}

	success := false
	defer func() {
		if !success {
			if _, seekErr := file.Seek(currentPos, io.SeekStart); seekErr != nil {
				logging.E("Failed to seek file %q: %v", file.Name(), seekErr)
			}
		}
	}()

	// Seek file start
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return fmt.Errorf("failed to seek to beginning of file: %w", err)
	}

	// File ops
	if err := file.Truncate(0); err != nil {
		return fmt.Errorf("failed to truncate file: %w", err)
	}

	if _, err := buf.WriteTo(file); err != nil {
		return fmt.Errorf("failed to write to file: %w", err)
	}

	// Ensure changes are persisted
	if err := file.Sync(); err != nil {
		return fmt.Errorf("failed to sync file: %w", err)
	}

	success = true
	return nil
}

// copyMeta creates a deep copy of the metadata map under lock.
func (rw *JSONFileRW) copyMeta() map[string]any {
	rw.mu.Lock()
	defer rw.mu.Unlock()
	if rw.Meta == nil {
		return make(map[string]any)
	}
	cloned := maps.Clone(rw.Meta)
	if cloned == nil {
		return make(map[string]any)
	}
	return cloned
}

// updateMeta safely updates the metadata map under write lock.
func (rw *JSONFileRW) updateMeta(newMeta map[string]any) {
	if newMeta == nil {
		newMeta = metaMapPool.Get().(map[string]any)
	}

	rw.mu.Lock()
	oldMeta := rw.Meta
	rw.Meta = newMeta
	rw.mu.Unlock()

	if oldMeta != nil {
		clear(oldMeta)
		metaMapPool.Put(oldMeta)
	}
}

// cleanFieldValue trims leading/trailing whitespaces after deletions.
func (rw *JSONFileRW) cleanFieldValue(value string) string {
	cleaned := strings.TrimSpace(value)
	cleaned = strings.Join(strings.Fields(cleaned), " ")
	return cleaned
}
