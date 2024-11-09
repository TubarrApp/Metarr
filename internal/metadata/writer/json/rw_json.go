package metadata

import (
	"encoding/json"
	"fmt"
	"io"
	"metarr/internal/config"
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
	Meta        map[string]interface{}
	File        *os.File
}

// NewJSONFileRW creates a new instance of the JSON file reader/writer
func NewJSONFileRW(file *os.File) *JSONFileRW {
	logging.D(3, "Retrieving new meta writer/rewriter for file '%s'...", file.Name())
	return &JSONFileRW{
		File: file,
		Meta: make(map[string]interface{}),
	}
}

// DecodeJSON parses and stores JSON metadata into a map and returns it
func (rw *JSONFileRW) DecodeJSON(file *os.File) (map[string]interface{}, error) {

	if file == nil {
		return nil, fmt.Errorf("nil file handle provided")
	}

	// Seek start
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return nil, fmt.Errorf("failed to seek file: %w", err)
	}

	// Decode to map
	decoder := json.NewDecoder(file)
	input := make(map[string]interface{})

	if err := decoder.Decode(&input); err != nil {
		return nil, fmt.Errorf("failed to decode JSON in DecodeMetadata: %w", err)
	}

	switch {
	case len(input) <= 0, input == nil:
		logging.D(3, "Metadata not stored, is blank: %v", input)
		return input, nil
	default:
		rw.updateMeta(input)
		logging.D(5, "Decoded and stored metadata: %v", input)
		return input, nil
	}
}

// RefreshJSON reloads the metadata map from the file after updates
func (rw *JSONFileRW) RefreshJSON() (map[string]interface{}, error) {

	if rw.File == nil {
		return nil, fmt.Errorf("no file handle available")
	}

	return rw.DecodeJSON(rw.File)
}

// WriteJSON inserts metadata into the JSON file from a map
func (rw *JSONFileRW) WriteJSON(fieldMap map[string]*string) (map[string]interface{}, error) {

	if fieldMap == nil {
		return nil, fmt.Errorf("fieldMap cannot be nil")
	}

	// Create a copy of the current metadata
	currentMeta := rw.copyMeta()

	logging.D(4, "Entering WriteMetadata for file '%s'", rw.File.Name())

	// Update metadata with new fields
	updated := false
	for field, value := range fieldMap {
		if field == "all-credits" {
			continue
		}

		if value != nil && *value != "" {

			if currentVal, exists := currentMeta[field]; !exists {
				logging.D(3, "Adding new field '%s' with value '%s'", field, *value)
				currentMeta[field] = *value
				updated = true

			} else if currentStrVal, ok := currentVal.(string); !ok || currentStrVal != *value || config.GetBool(keys.MOverwrite) {
				logging.D(3, "Updating field '%s' from '%v' to '%s'", field, currentVal, *value)
				currentMeta[field] = *value
				updated = true

			} else {
				logging.D(3, "Skipping field '%s' - value unchanged and overwrite not forced", field)
			}
		}
	}

	// Return if no updates
	if !updated {
		logging.D(2, "No fields were updated")
		return currentMeta, nil
	}

	// Backup if option set
	if config.GetBool(keys.NoFileOverwrite) {
		if err := backup.BackupFile(rw.File); err != nil {
			return currentMeta, fmt.Errorf("failed to create backup: %w", err)
		}
	}

	// Write file
	if err := rw.writeJsonToFile(rw.File, currentMeta); err != nil {
		return currentMeta, err
	}

	rw.updateMeta(currentMeta)

	logging.D(3, "Successfully updated JSON file with new metadata")
	return currentMeta, nil
}

// MakeJSONEdits applies a series of transformations and writes the final result to the file
func (rw *JSONFileRW) MakeJSONEdits(file *os.File, fd *models.FileData) (bool, error) {

	currentMeta := rw.copyMeta()

	logging.D(5, "Entering MakeJSONEdits.\nData: %v", currentMeta)

	var (
		edited, ok bool
		trimPfx    []*models.MetaTrimPrefix
		trimSfx    []*models.MetaTrimSuffix

		apnd []*models.MetaAppend
		pfx  []*models.MetaPrefix

		new []*models.MetaNewField

		replace []*models.MetaReplace
	)

	// Replacements
	if len(fd.ModelMReplace) > 0 {
		logging.I("Model for file '%s' making replacements", fd.OriginalVideoBaseName)
		replace = fd.ModelMReplace
	} else if config.IsSet(keys.MReplaceText) {
		if replace, ok = config.Get(keys.MReplaceText).([]*models.MetaReplace); !ok {
			logging.E(0, "Could not retrieve prefix trim, wrong type: '%T'", replace)
		}
	}

	// Field trim
	if len(fd.ModelMTrimPrefix) > 0 {
		logging.I("Model for file '%s' trimming prefixes", fd.OriginalVideoBaseName)
		trimPfx = fd.ModelMTrimPrefix
	} else if config.IsSet(keys.MTrimPrefix) {
		if trimPfx, ok = config.Get(keys.MTrimPrefix).([]*models.MetaTrimPrefix); !ok {
			logging.E(0, "Could not retrieve prefix trim, wrong type: '%T'", trimPfx)
		}
	}

	if len(fd.ModelMTrimSuffix) > 0 {
		logging.I("Model for file '%s' trimming suffixes", fd.OriginalVideoBaseName)
		trimSfx = fd.ModelMTrimSuffix
	} else if config.IsSet(keys.MTrimSuffix) {
		if trimSfx, ok = config.Get(keys.MTrimSuffix).([]*models.MetaTrimSuffix); !ok {
			logging.E(0, "Could not retrieve suffix trim, wrong type: '%T'", trimSfx)
		}
	}

	// Append and prefix
	if len(fd.ModelMAppend) > 0 {
		logging.I("Model for file '%s' adding appends", fd.OriginalVideoBaseName)
		apnd = fd.ModelMAppend
	} else if config.IsSet(keys.MAppend) {
		if apnd, ok = config.Get(keys.MAppend).([]*models.MetaAppend); !ok {
			logging.E(0, "Could not retrieve appends, wrong type: '%T'", apnd)
		}
	}

	if len(fd.ModelMPrefix) > 0 {
		logging.I("Model for file '%s' adding prefixes", fd.OriginalVideoBaseName)
		pfx = fd.ModelMPrefix
	} else if config.IsSet(keys.MPrefix) {
		if pfx, ok = config.Get(keys.MPrefix).([]*models.MetaPrefix); !ok {
			logging.E(0, "Could not retrieve prefix, wrong type: '%T'", pfx)
		}
	}

	// New fields
	if len(fd.ModelMNewField) > 0 {
		logging.I("Model for file '%s' applying preset new field additions", fd.OriginalVideoBaseName)
		new = fd.ModelMNewField
	} else if config.IsSet(keys.MNewField) {
		if new, ok = config.Get(keys.MNewField).([]*models.MetaNewField); !ok {
			logging.E(0, "Could not retrieve new fields, wrong type: '%T'", pfx)
		}
	}

	// Make edits:
	// Replace
	if len(replace) > 0 {
		if ok, err := rw.replaceJson(currentMeta, replace); err != nil {
			logging.E(0, err.Error())
		} else if ok {
			edited = true
		}
	}

	// Trim
	if len(trimPfx) > 0 {
		if ok, err := rw.trimJsonPrefix(currentMeta, trimPfx); err != nil {
			logging.E(0, err.Error())
		} else if ok {
			edited = true
		}
	}

	if len(trimSfx) > 0 {
		if ok, err := rw.trimJsonSuffix(currentMeta, trimSfx); err != nil {
			logging.E(0, err.Error())
		} else if ok {
			edited = true
		}
	}

	// Append and prefix
	if len(apnd) > 0 {
		if ok, err := rw.jsonAppend(currentMeta, apnd); err != nil {
			logging.E(0, err.Error())
		} else if ok {
			edited = true
		}
	}

	if len(pfx) > 0 {
		if ok, err := rw.jsonPrefix(currentMeta, pfx); err != nil {
			logging.E(0, err.Error())
		} else if ok {
			edited = true
		}
	}

	// Add new
	if len(new) > 0 {
		if ok, err := rw.setJsonField(currentMeta, fd.ModelMOverwrite, new); err != nil {
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
	if err := rw.writeJsonToFile(file, currentMeta); err != nil {
		return false, fmt.Errorf("failed to write updated JSON to file: %w", err)
	}

	rw.updateMeta(currentMeta)

	fmt.Println()
	logging.S(0, "Successfully applied metadata edits to: %v", file.Name())

	return edited, nil
}

// JSONDateTagEdits is a public function to add date tags into the metafile, this is useful because
// the dates may not yet be scraped when the initial MakeMetaEdits runs
func (rw *JSONFileRW) JSONDateTagEdits(file *os.File, fd *models.FileData) (edited bool, err error) {

	logging.D(4, "Entering MakeDateTagEdits for file '%s'", file.Name())

	currentMeta := rw.copyMeta()

	logging.D(4, "About to perform MakeDateTagEdits operations for file '%s'", file.Name())

	// Delete date tag first, user's may want to delete and re-build
	if config.IsSet(keys.MDelDateTagMap) {
		logging.D(3, "Stripping metafield date tag...")
		if delDateTagMap, ok := config.Get(keys.MDelDateTagMap).(map[string]*models.MetaDateTag); ok {

			if len(delDateTagMap) > 0 {

				if ok, err := rw.jsonFieldDateTag(currentMeta, delDateTagMap, fd, enums.DATE_TAG_DEL_OP); err != nil {
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
	if config.IsSet(keys.MDateTagMap) {
		logging.D(3, "Adding metafield date tag...")
		if dateTagMap, ok := config.Get(keys.MDateTagMap).(map[string]*models.MetaDateTag); ok {

			if len(dateTagMap) > 0 {

				if ok, err := rw.jsonFieldDateTag(currentMeta, dateTagMap, fd, enums.DATE_TAG_ADD_OP); err != nil {
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
	if err = rw.writeJsonToFile(file, currentMeta); err != nil {
		return false, fmt.Errorf("failed to write updated JSON to file: %w", err)
	}

	rw.updateMeta(currentMeta)

	fmt.Println()
	logging.S(0, "Successfully applied date tag JSON edits to: %v", file.Name())

	return edited, nil
}
