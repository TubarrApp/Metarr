package metadata

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"metarr/internal/cfg"
	enums "metarr/internal/domain/enums"
	keys "metarr/internal/domain/keys"
	"metarr/internal/models"
	backup "metarr/internal/utils/fs/backup"
	logging "metarr/internal/utils/logging"
	"os"
	"sync"
)

type JSONFileRW struct {
	mu          sync.RWMutex
	muFileWrite sync.Mutex
	Meta        map[string]any
	File        *os.File
	encoder     *json.Encoder
	buffer      *bytes.Buffer
}

// NewJSONFileRW creates a new instance of the JSON file reader/writer
func NewJSONFileRW(file *os.File) *JSONFileRW {
	logging.D(3, "Retrieving new meta writer/rewriter for file %q...", file.Name())
	return &JSONFileRW{
		File: file,
		Meta: metaMapPool.Get().(map[string]any),
	}
}

// DecodeJSON parses and stores JSON metadata into a map and returns it
func (rw *JSONFileRW) DecodeJSON(file *os.File) (map[string]any, error) {
	if file == nil {
		return nil, fmt.Errorf("file passed in nil")
	}

	currentPos, err := file.Seek(0, io.SeekCurrent)
	if err != nil {
		return nil, fmt.Errorf("failed to get current position: %w", err)
	}
	success := false
	defer func() {
		if !success {
			if _, err := file.Seek(currentPos, io.SeekStart); err != nil {
				logging.E(0, err.Error())
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
		return nil, fmt.Errorf("file passed in nil")
	}
	return rw.DecodeJSON(rw.File)
}

// WriteJSON inserts metadata into the JSON file from a map
func (rw *JSONFileRW) WriteJSON(fieldMap map[string]*string) (map[string]any, error) {
	if fieldMap == nil {
		return nil, fmt.Errorf("field map passed in nil")
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

			} else if currentStrVal, ok := currentVal.(string); !ok || currentStrVal != *ptr || cfg.GetBool(keys.MOverwrite) {
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
	if cfg.GetBool(keys.NoFileOverwrite) {
		if err := backup.BackupFile(rw.File); err != nil {
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
		return false, fmt.Errorf("file passed in nil")
	}

	currentMeta := rw.copyMeta()

	logging.D(5, "Entering MakeJSONEdits.\nData: %v", currentMeta)

	// SHOULD MOVE THESE INTO THE RESPECTIVE FUNCTIONS
	// THESE PRESENTLY ESCAPE TO HEAP FOR NO GOOD REASON
	var (
		edited, ok bool
		trimPfx    []models.MetaTrimPrefix
		trimSfx    []models.MetaTrimSuffix
		apnd       []models.MetaAppend
		pfx        []models.MetaPrefix
		newField   []models.MetaNewField
		replace    []models.MetaReplace
		copyTo     []models.CopyToField
		pasteFrom  []models.PasteFromField
	)

	// Initialize:
	// Replacements
	if len(fd.ModelMReplace) > 0 {
		logging.I("Model for file %q making replacements", fd.OriginalVideoBaseName)
		replace = fd.ModelMReplace
	} else if cfg.IsSet(keys.MReplaceText) {
		if replace, ok = cfg.Get(keys.MReplaceText).([]models.MetaReplace); !ok {
			logging.E(0, "Could not retrieve prefix trim, wrong type: '%T'", replace)
		}
	}

	// Field trim
	if len(fd.ModelMTrimPrefix) > 0 {
		logging.I("Model for file %q trimming prefixes", fd.OriginalVideoBaseName)
		trimPfx = fd.ModelMTrimPrefix
	} else if cfg.IsSet(keys.MTrimPrefix) {
		if trimPfx, ok = cfg.Get(keys.MTrimPrefix).([]models.MetaTrimPrefix); !ok {
			logging.E(0, "Could not retrieve prefix trim, wrong type: '%T'", trimPfx)
		}
	}

	if len(fd.ModelMTrimSuffix) > 0 {
		logging.I("Model for file %q trimming suffixes", fd.OriginalVideoBaseName)
		trimSfx = fd.ModelMTrimSuffix
	} else if cfg.IsSet(keys.MTrimSuffix) {
		if trimSfx, ok = cfg.Get(keys.MTrimSuffix).([]models.MetaTrimSuffix); !ok {
			logging.E(0, "Could not retrieve suffix trim, wrong type: '%T'", trimSfx)
		}
	}

	// Append and prefix
	if len(fd.ModelMAppend) > 0 {
		logging.I("Model for file %q adding appends", fd.OriginalVideoBaseName)
		apnd = fd.ModelMAppend
	} else if cfg.IsSet(keys.MAppend) {
		if apnd, ok = cfg.Get(keys.MAppend).([]models.MetaAppend); !ok {
			logging.E(0, "Could not retrieve appends, wrong type: '%T'", apnd)
		}
	}

	if len(fd.ModelMPrefix) > 0 {
		logging.I("Model for file %q adding prefixes", fd.OriginalVideoBaseName)
		pfx = fd.ModelMPrefix
	} else if cfg.IsSet(keys.MPrefix) {
		if pfx, ok = cfg.Get(keys.MPrefix).([]models.MetaPrefix); !ok {
			logging.E(0, "Could not retrieve prefix, wrong type: '%T'", pfx)
		}
	}

	// New fields
	if len(fd.ModelMNewField) > 0 {
		logging.I("Model for file %q applying preset new field additions", fd.OriginalVideoBaseName)
		newField = fd.ModelMNewField
	} else if cfg.IsSet(keys.MNewField) {
		if newField, ok = cfg.Get(keys.MNewField).([]models.MetaNewField); !ok {
			logging.E(0, "Could not retrieve new fields, wrong type: '%T'", newField)
		}
	}

	// Copy/paste
	if cfg.IsSet(keys.MCopyToField) {
		if copyTo, ok = cfg.Get(keys.MCopyToField).([]models.CopyToField); !ok {
			logging.E(0, "Could not retrieve copy operations, wrong type: '%T'", copyTo)
		}
	}

	if cfg.IsSet(keys.MPasteFromField) {
		if pasteFrom, ok = cfg.Get(keys.MPasteFromField).([]models.PasteFromField); !ok {
			logging.E(0, "Could not retrieve paste operations, wrong type: '%T'", pasteFrom)
		}
	}

	// Make edits:
	// Replace
	if len(replace) > 0 {
		if ok, err := replaceJSON(currentMeta, replace); err != nil {
			logging.E(0, err.Error())
		} else if ok {
			edited = true
		}
	}

	// Trim
	if len(trimPfx) > 0 {
		if ok, err := trimJSONPrefix(currentMeta, trimPfx); err != nil {
			logging.E(0, err.Error())
		} else if ok {
			edited = true
		}
	}

	if len(trimSfx) > 0 {
		if ok, err := trimJSONSuffix(currentMeta, trimSfx); err != nil {
			logging.E(0, err.Error())
		} else if ok {
			edited = true
		}
	}

	// Append and prefix
	if len(apnd) > 0 {
		if ok, err := jsonAppend(currentMeta, apnd); err != nil {
			logging.E(0, err.Error())
		} else if ok {
			edited = true
		}
	}

	if len(pfx) > 0 {
		if ok, err := jsonPrefix(currentMeta, pfx); err != nil {
			logging.E(0, err.Error())
		} else if ok {
			edited = true
		}
	}

	// Copy/paste
	if len(copyTo) > 0 {
		if ok, err := copyToField(currentMeta, copyTo); err != nil {
			logging.E(0, err.Error())
		} else if ok {
			edited = true
		}
	}

	if len(pasteFrom) > 0 {
		if ok, err := pasteFromField(currentMeta, pasteFrom); err != nil {
			logging.E(0, err.Error())
		} else if ok {
			edited = true
		}
	}

	// Add new
	if len(newField) > 0 {
		if ok, err := setJSONField(currentMeta, rw.File.Name(), fd.ModelMOverwrite, newField); err != nil {
			logging.E(0, err.Error())
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

	fmt.Println()
	logging.S(0, "Successfully applied metadata edits to: %v", file.Name())

	return edited, nil
}

// JSONDateTagEdits is a public function to add date tags into the metafile, this is useful because
// the dates may not yet be scraped when the initial MakeJSONEdits runs
func (rw *JSONFileRW) JSONDateTagEdits(file *os.File, fd *models.FileData) (edited bool, err error) {
	if file == nil {
		return false, fmt.Errorf("file passed in nil")
	}

	logging.D(4, "Entering MakeDateTagEdits for file %q", file.Name())

	currentMeta := rw.copyMeta()

	logging.D(4, "About to perform MakeDateTagEdits operations for file %q", file.Name())

	// Delete date tag first, user's may want to delete and re-build
	if cfg.IsSet(keys.MDelDateTagMap) {
		logging.D(3, "Stripping metafield date tag...")
		if delDateTagMap, ok := cfg.Get(keys.MDelDateTagMap).(map[string]models.MetaDateTag); ok {

			if len(delDateTagMap) > 0 {

				if ok, err := jsonFieldDateTag(currentMeta, delDateTagMap, fd, enums.DATE_TAG_DEL_OP); err != nil {
					logging.E(0, err.Error())
				} else if ok {
					edited = true
				}
			} else {
				logging.E(0, "delDateTagMap grabbed empty")
			}
		} else {
			logging.E(0, "Got null or wrong type for %s: %T", keys.MDelDateTagMap, delDateTagMap)
		}
	}

	// Add date tag
	if cfg.IsSet(keys.MDateTagMap) {
		logging.D(3, "Adding metafield date tag...")
		if dateTagMap, ok := cfg.Get(keys.MDateTagMap).(map[string]models.MetaDateTag); ok {

			if len(dateTagMap) > 0 {

				if ok, err := jsonFieldDateTag(currentMeta, dateTagMap, fd, enums.DATE_TAG_ADD_OP); err != nil {
					logging.E(0, err.Error())
				} else if ok {
					edited = true
				}
			} else {
				logging.E(0, "dateTagMap grabbed empty")
			}
		} else {
			logging.E(0, "Got null or wrong type for %s: %T", keys.MDateTagMap, dateTagMap)
		}
	}

	if !edited {
		logging.D(2, "No date tag edits made, returning...")
		return false, nil
	}

	// Write back to file
	if err = rw.writeJSONToFile(file, currentMeta); err != nil {
		return false, fmt.Errorf("failed to write updated JSON to file: %w", err)
	}

	rw.updateMeta(currentMeta)

	fmt.Println()
	logging.S(0, "Successfully applied date tag JSON edits to: %v", file.Name())

	return edited, nil
}
