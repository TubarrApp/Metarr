// Package jsonrw performs JSON read and write operations.
package jsonrw

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"metarr/internal/abstractions"
	"metarr/internal/domain/enums"
	"metarr/internal/domain/keys"
	"metarr/internal/models"
	"metarr/internal/utils/fs/backup"
	"metarr/internal/utils/logging"
	"os"
	"sync"
)

// JSONFileRW is used to access JSON reading/writing utilities.
type JSONFileRW struct {
	ctx         context.Context
	mu          sync.RWMutex
	muFileWrite sync.Mutex
	Meta        map[string]any
	File        *os.File
	encoder     *json.Encoder
	buffer      *bytes.Buffer
}

// NewJSONFileRW creates a new instance of the JSON file reader/writer
func NewJSONFileRW(ctx context.Context, file *os.File) *JSONFileRW {
	logging.D(3, "Retrieving new meta writer/rewriter for file %q...", file.Name())
	return &JSONFileRW{
		ctx:  ctx,
		File: file,
		Meta: metaMapPool.Get().(map[string]any),
	}
}

// DecodeJSON parses and stores JSON metadata into a map and returns it
func (rw *JSONFileRW) DecodeJSON(file *os.File) (map[string]any, error) {
	if file == nil {
		return nil, errors.New("file passed in nil")
	}

	currentPos, err := file.Seek(0, io.SeekCurrent)
	if err != nil {
		return nil, fmt.Errorf("failed to get current position: %w", err)
	}
	success := false
	defer func() {
		if !success {
			if _, err := file.Seek(currentPos, io.SeekStart); err != nil {
				logging.E("Failed to seek file %q: %v", file.Name(), err)
			}
		}
	}()

	// Seek start
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return nil, fmt.Errorf("failed to seek file: %w", err)
	}

	// Decode to map
	decoder := json.NewDecoder(file)
	data := metaMapPool.Get().(map[string]any)

	if err := decoder.Decode(&data); err != nil {
		return nil, fmt.Errorf("failed to decode JSON in DecodeMetadata: %w", err)
	}

	switch {
	case len(data) == 0, data == nil:
		logging.D(3, "Metadata not stored, is blank: %v", data)
		return data, nil
	default:
		rw.updateMeta(data)
		logging.D(5, "Decoded and stored metadata: %v", data)
		success = true
		return data, nil
	}
}

// RefreshJSON reloads the metadata map from the file after updates
func (rw *JSONFileRW) RefreshJSON() (map[string]any, error) {
	if rw.File == nil {
		return nil, errors.New("file passed in nil")
	}
	return rw.DecodeJSON(rw.File)
}

// WriteJSON inserts metadata into the JSON file from a map
func (rw *JSONFileRW) WriteJSON(fieldMap map[string]*string) (map[string]any, error) {
	if fieldMap == nil {
		return nil, errors.New("field map passed in nil")
	}

	// Create a copy of the current metadata
	currentMeta := rw.copyMeta()
	logging.D(4, "Entering WriteMetadata for file %q", rw.File.Name())

	// Update metadata with new fields
	updated := false
	for k, ptr := range fieldMap {
		if ptr == nil {
			continue
		}

		if *ptr != "" {

			if currentVal, exists := currentMeta[k]; !exists {
				logging.D(3, "Adding new field %q with value %q", k, *ptr)
				currentMeta[k] = *ptr
				updated = true

			} else if currentStrVal, ok := currentVal.(string); !ok || currentStrVal != *ptr || abstractions.GetBool(keys.MOverwrite) {
				logging.D(3, "Updating field %q from '%v' to %q", k, currentVal, *ptr)
				currentMeta[k] = *ptr
				updated = true

			} else {
				logging.D(3, "Skipping field %q - value unchanged and overwrite not forced", k)
			}
		}
	}

	// Return if no updates
	if !updated {
		logging.D(2, "No fields were updated")
		return currentMeta, nil
	}

	// Backup if option set
	if abstractions.GetBool(keys.NoFileOverwrite) {
		if err := backup.File(rw.File); err != nil {
			return currentMeta, fmt.Errorf("failed to create backup: %w", err)
		}
	}

	// Write file
	if err := rw.writeJSONToFile(rw.File, currentMeta); err != nil {
		return currentMeta, err
	}

	rw.updateMeta(currentMeta)

	logging.D(3, "Successfully updated JSON file with new metadata")
	return currentMeta, nil
}

// MakeJSONEdits applies a series of transformations and writes the final result to the file
func (rw *JSONFileRW) MakeJSONEdits(file *os.File, fd *models.FileData) (bool, error) {
	if file == nil {
		return false, errors.New("file passed in nil")
	}

	currentMeta := rw.copyMeta()

	logging.D(5, "Entering MakeJSONEdits.\nData: %v", currentMeta)

	var (
		edited    bool
		trimPfx   []models.MetaTrimPrefix
		trimSfx   []models.MetaTrimSuffix
		apnd      []models.MetaAppend
		pfx       []models.MetaPrefix
		newField  []models.MetaNewField
		replace   []models.MetaReplace
		copyTo    []models.CopyToField
		pasteFrom []models.PasteFromField
	)

	// Initialize:
	// Replacements
	if len(fd.MetaOps.Replaces) > 0 {
		logging.I("Model for file %q making replacements", fd.OriginalVideoBaseName)
		replace = fd.MetaOps.Replaces
	}

	// Field trim
	if len(fd.MetaOps.TrimPrefixes) > 0 {
		logging.I("Model for file %q trimming prefixes", fd.OriginalVideoBaseName)
		trimPfx = fd.MetaOps.TrimPrefixes
	}

	if len(fd.MetaOps.TrimSuffixes) > 0 {
		logging.I("Model for file %q trimming suffixes", fd.OriginalVideoBaseName)
		trimSfx = fd.MetaOps.TrimSuffixes
	}

	// Append and prefix
	if len(fd.MetaOps.Appends) > 0 {
		logging.I("Model for file %q adding appends", fd.OriginalVideoBaseName)
		apnd = fd.MetaOps.Appends
	}

	if len(fd.MetaOps.Prefixes) > 0 {
		logging.I("Model for file %q adding prefixes", fd.OriginalVideoBaseName)
		pfx = fd.MetaOps.Prefixes
	}

	// New fields
	if len(fd.MetaOps.NewFields) > 0 {
		logging.I("Model for file %q applying new field additions", fd.OriginalVideoBaseName)
		newField = fd.MetaOps.NewFields
	}

	// Copy/paste
	if len(fd.MetaOps.CopyToFields) > 0 {
		logging.I("Model for file %q copying to fields", fd.MetaOps.CopyToFields)
		copyTo = fd.MetaOps.CopyToFields
	}

	if len(fd.MetaOps.PasteFromFields) > 0 {
		logging.I("Model for file %q copying to fields", fd.MetaOps.PasteFromFields)
		pasteFrom = fd.MetaOps.PasteFromFields
	}

	filename := rw.File.Name()

	// Make edits:
	// Replace
	if len(replace) > 0 {
		if changesMade := rw.replaceJSON(currentMeta, replace); changesMade {
			edited = true
		}
	}

	// Trim
	if len(trimPfx) > 0 {
		if changesMade := rw.trimJSONPrefix(currentMeta, trimPfx); changesMade {
			edited = true
		}
	}

	if len(trimSfx) > 0 {
		if changesMade := rw.trimJSONSuffix(currentMeta, trimSfx); changesMade {
			edited = true
		}
	}

	// Append and prefix
	if len(apnd) > 0 {
		if changesMade := rw.jsonAppend(currentMeta, filename, apnd); changesMade {
			edited = true
		}
	}

	if len(pfx) > 0 {
		if changesMade := rw.jsonPrefix(currentMeta, filename, pfx); changesMade {
			edited = true
		}
	}

	// Copy/paste
	if len(copyTo) > 0 {
		if ok := rw.copyToField(currentMeta, copyTo); ok {
			edited = true
		}
	}

	if len(pasteFrom) > 0 {
		if ok := rw.pasteFromField(currentMeta, pasteFrom); ok {
			edited = true
		}
	}

	// Add new
	if len(newField) > 0 {
		if ok, err := rw.setJSONField(currentMeta, filename, fd.ModelMOverwrite, newField); err != nil {
			logging.E("Failed to set fields with %+v: %v", newField, err)
		} else if ok {
			edited = true
		}
	}

	if !edited {
		logging.D(3, "No JSON metadata edits made")
		return false, nil
	}

	// Write new metadata to file
	if err := rw.writeJSONToFile(file, currentMeta); err != nil {
		return false, fmt.Errorf("failed to write updated JSON to file: %w", err)
	}

	// Save the meta back into the model
	rw.updateMeta(currentMeta)
	logging.S("Successfully applied metadata edits to: %v", file.Name())

	return edited, nil
}

// JSONDateTagEdits is a public function to add date tags into the metafile.
//
// This is useful because the dates may not yet be scraped when the initial MakeJSONEdits runs.
func (rw *JSONFileRW) JSONDateTagEdits(file *os.File, fd *models.FileData) (edited bool, err error) {
	if file == nil {
		return false, errors.New("file passed in nil")
	}

	logging.D(4, "Entering MakeDateTagEdits for file %q", file.Name())

	currentMeta := rw.copyMeta()

	logging.D(4, "About to perform MakeDateTagEdits operations for file %q", file.Name())

	// Delete date tag first, user's may want to delete and re-build
	if len(fd.MetaOps.DeleteDateTags) > 0 {
		logging.I("Stripping metafield date tags (User entered: %v)", fd.MetaOps.DeleteDateTags)

		if ok, err := rw.jsonFieldDateTag(currentMeta, fd.MetaOps.DeleteDateTags, fd, enums.DatetagDelOp); err != nil {
			logging.E("failed to delete date tag in %q: %v", fd.JSONFilePath, err)
		} else if ok {
			edited = true
		}
	}

	// Add date tag
	if len(fd.MetaOps.DateTags) > 0 {
		logging.I("Adding metafield date tags (User entered: %v)", fd.MetaOps.DateTags)

		if ok, err := rw.jsonFieldDateTag(currentMeta, fd.MetaOps.DateTags, fd, enums.DatetagAddOp); err != nil {
			logging.E("failed to delete date tag in %q: %v", fd.JSONFilePath, err)
		} else if ok {
			edited = true
		}
	}

	if !edited {
		logging.D(1, "No date tag edits made, returning...")
		return false, nil
	}

	// Write back to file
	if err = rw.writeJSONToFile(file, currentMeta); err != nil {
		return false, fmt.Errorf("failed to write updated JSON to file: %w", err)
	}

	rw.updateMeta(currentMeta)
	logging.S("Successfully applied date tag JSON edits to: %v", file.Name())

	return edited, nil
}
