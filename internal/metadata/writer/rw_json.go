package metadata

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"metarr/internal/config"
	enums "metarr/internal/domain/enums"
	keys "metarr/internal/domain/keys"
	tags "metarr/internal/metadata/tags"
	"metarr/internal/models"
	backup "metarr/internal/utils/fs/backup"
	logging "metarr/internal/utils/logging"
	prompt "metarr/internal/utils/prompt"
	"os"
	"strings"
	"sync"
)

type JSONFileRW struct {
	mu   sync.RWMutex
	Meta map[string]interface{}
	File *os.File
}

// NewJSONFileRW creates a new instance of the JSON file reader/writer
func NewJSONFileRW(file *os.File) *JSONFileRW {
	logging.D(3, "Retrieving new meta writer/rewriter for file '%s'...", file.Name())
	return &JSONFileRW{
		File: file,
	}
}

// DecodeMetadata parses and stores XML metadata into a map and returns it
func (rw *JSONFileRW) DecodeMetadata(file *os.File) (map[string]interface{}, error) {

	rw.mu.Lock()
	defer rw.mu.Unlock()

	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return nil, fmt.Errorf("failed to seek file: %w", err)
	}

	// Create a decoder to read directly from file
	decoder := json.NewDecoder(file)

	// Decode to map
	input := make(map[string]interface{})
	if err := decoder.Decode(&input); err != nil {
		return nil, fmt.Errorf("failed to decode JSON: %w", err)
	}

	switch {
	case len(input) <= 0, input == nil:
		logging.D(3, "Metadata not stored, is blank: %v", input)
	default:
		rw.Meta = input
		logging.D(3, "Decoded and stored metadata: %v", rw.Meta)
	}

	return rw.Meta, nil
}

// RefreshMetadata reloads the metadata map from the file after updates
func (rw *JSONFileRW) RefreshMetadata() (map[string]interface{}, error) {

	rw.mu.RLock()
	defer rw.mu.RUnlock()

	if _, err := rw.File.Seek(0, io.SeekStart); err != nil {
		return nil, fmt.Errorf("failed to seek file: %w", err)
	}

	// Decode metadata
	decoder := json.NewDecoder(rw.File)

	if err := decoder.Decode(&rw.Meta); err != nil {
		return nil, fmt.Errorf("failed to decode JSON: %w", err)
	}

	logging.D(3, "Decoded metadata: %v", rw.Meta)

	return rw.Meta, nil
}

// WriteMetadata inserts metadata into the JSON file from a map
func (rw *JSONFileRW) WriteMetadata(fieldMap map[string]*string) (map[string]interface{}, error) {

	rw.mu.Lock()
	defer rw.mu.Unlock()

	logging.D(3, "Entering WriteMetadata for file '%s'", rw.File.Name())
	noFileOW := config.GetBool(keys.NoFileOverwrite)
	metaOW := config.GetBool(keys.MOverwrite)

	if noFileOW {
		err := backup.BackupFile(rw.File)
		if err != nil {
			return rw.Meta, fmt.Errorf("failed to create a backup of file '%s'", rw.File.Name())
		}
	}
	// Refresh metadata without lock
	if err := rw.refreshMetadataInternal(rw.File); err != nil {
		return rw.Meta, err
	}

	// Update metadata with new fields
	updated := false
	for field, value := range fieldMap {
		if field == "all-credits" {
			continue
		}

		if value != nil && *value != "" {
			currentVal, exists := rw.Meta[field]
			if !exists {
				logging.D(3, "Adding new field '%s' with value '%s'", field, *value)
				rw.Meta[field] = *value
				updated = true
			} else if currentStrVal, ok := currentVal.(string); !ok || currentStrVal != *value || metaOW {
				logging.D(3, "Updating field '%s' from '%v' to '%s'", field, currentVal, *value)
				rw.Meta[field] = *value
				updated = true
			} else {
				logging.D(3, "Skipping field '%s' - value unchanged and overwrite not forced", field)
			}
		}
	}

	// Return if no updates
	if !updated {
		logging.D(2, "No fields were updated")
		return rw.Meta, nil
	}
	// Format the updated metadata for writing to file
	updatedContent, err := json.MarshalIndent(rw.Meta, "", "  ")
	if err != nil {
		return rw.Meta, fmt.Errorf("failed to marshal updated JSON: %w", err)
	}

	if err = rw.writeMetadataToFile(rw.File, updatedContent); err != nil {
		return rw.Meta, err
	}

	logging.D(3, "Successfully updated JSON file with new metadata")
	return rw.Meta, nil
}

// MakeMetaEdits applies a series of transformations and writes the final result to the file
func (rw *JSONFileRW) MakeMetaEdits(data map[string]interface{}, file *os.File, fd *models.FileData) (bool, error) {

	logging.D(5, "Entering MakeMetaEdits.\nData: %v", data)

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
		if ok, err := rw.replaceJson(data, replace); err != nil {
			logging.E(0, err.Error())
		} else if ok {
			edited = true
		}
	}

	// Trim
	if len(trimPfx) > 0 {
		if ok, err := rw.trimJsonPrefix(data, trimPfx); err != nil {
			logging.E(0, err.Error())
		} else if ok {
			edited = true
		}
	}

	if len(trimSfx) > 0 {
		if ok, err := rw.trimJsonSuffix(data, trimSfx); err != nil {
			logging.E(0, err.Error())
		} else if ok {
			edited = true
		}
	}

	// Append and prefix
	if len(apnd) > 0 {
		if ok, err := rw.jsonAppend(data, apnd); err != nil {
			logging.E(0, err.Error())
		} else if ok {
			edited = true
		}
	}

	if len(pfx) > 0 {
		if ok, err := rw.jsonPrefix(data, pfx); err != nil {
			logging.E(0, err.Error())
		} else if ok {
			edited = true
		}
	}

	// Add new
	if len(new) > 0 {
		if ok, err := rw.addNewJsonField(data, fd.ModelMOverwrite, new); err != nil {
			logging.E(0, err.Error())
		} else if ok {
			edited = true
		}
	}

	// Add date tag
	if config.IsSet(keys.MDateTagMap) {
		logging.D(3, "Adding metafield date tag...")
		if dateTagMap, ok := config.Get(keys.MDateTagMap).(map[string]*models.MetaDateTag); ok {
			if len(dateTagMap) > 0 {

				if ok, err := rw.jsonFieldDateTag(data, dateTagMap, fd); err != nil {
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

	// Marshal the updated JSON back to a byte slice
	updatedFileContent, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return false, fmt.Errorf("failed to marshal updated JSON: %w", err)
	}

	if err = rw.writeMetadataToFile(file, updatedFileContent); err != nil {
		return false, fmt.Errorf("failed to write updated JSON to file: %w", err)
	}

	fmt.Println()
	logging.S(0, "Successfully applied metadata edits to: %v", file.Name())

	return edited, nil
}

// refreshMetadataInternal is a private metadata refresh function
func (rw *JSONFileRW) refreshMetadataInternal(file *os.File) error {

	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return fmt.Errorf("failed to seek file: %w", err)
	}

	if len(rw.Meta) <= 0 || rw.Meta == nil {
		return fmt.Errorf("JSONFileRW's stored metadata map is empty or null, did you forget to decode?")
	}

	decoder := json.NewDecoder(file)
	if err := decoder.Decode(&rw.Meta); err != nil {
		return fmt.Errorf("failed to decode JSON: %w", err)
	}

	logging.D(3, "Decoded metadata: %v", rw.Meta)
	return nil
}

// writeMetadataToFile is a private metadata writing helper function
func (rw *JSONFileRW) writeMetadataToFile(file *os.File, content []byte) error {

	if err := file.Truncate(0); err != nil {
		return fmt.Errorf("failed to truncate file: %w", err)
	}

	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return fmt.Errorf("failed to seek to beginning of file: %w", err)
	}

	if _, err := file.Write(content); err != nil {
		return fmt.Errorf("failed to write to file: %w", err)
	}

	return nil
}

// replaceMeta makes user defined meta replacements
func (rw *JSONFileRW) replaceJson(data map[string]interface{}, replace []*models.MetaReplace) (bool, error) {

	logging.D(5, "Entering replaceJson with data: %v", data)

	if len(replace) == 0 {
		logging.E(0, "Called replaceMeta without replacements")
		return false, nil
	}

	edited := false
	for _, replacement := range replace {
		if replacement.Field == "" || replacement.Value == "" {
			continue
		}

		if val, exists := data[replacement.Field]; exists {
			if strVal, ok := val.(string); ok {
				logging.D(3, "Identified field '%s', replacing '%s' with '%s'", replacement.Field, replacement.Value, replacement.Replacement)
				data[replacement.Field] = strings.ReplaceAll(strVal, replacement.Value, replacement.Replacement)
				edited = true
			}
		}
	}
	logging.D(5, "After meta replace: %v", data)
	return edited, nil
}

// trimMetaPrefix trims defined prefixes from specified fields
func (rw *JSONFileRW) trimJsonPrefix(data map[string]interface{}, trimPfx []*models.MetaTrimPrefix) (bool, error) {

	logging.D(5, "Entering trimJsonPrefix with data: %v", data)

	if len(trimPfx) == 0 {
		logging.E(0, "Called trimMetaPrefix without prefixes to trim")
		return false, nil
	}

	edited := false
	for _, prefix := range trimPfx {
		if prefix.Field == "" || prefix.Prefix == "" {
			continue
		}

		if val, exists := data[prefix.Field]; exists {
			if strVal, ok := val.(string); ok {
				logging.D(3, "Identified field '%s', trimming '%s'", prefix.Field, prefix.Prefix)
				data[prefix.Field] = strings.TrimPrefix(strVal, prefix.Prefix)
				edited = true
			}
		}
	}
	logging.D(5, "After prefix trim: %v", data)
	return edited, nil
}

// trimMetaSuffix trims defined suffixes from specified fields
func (rw *JSONFileRW) trimJsonSuffix(data map[string]interface{}, trimSfx []*models.MetaTrimSuffix) (bool, error) {

	logging.D(5, "Entering trimJsonSuffix with data: %v", data)

	if len(trimSfx) == 0 {
		logging.E(0, "Called trimMetaSuffix without prefixes to trim")
		return false, nil
	}

	edited := false
	for _, suffix := range trimSfx {
		if suffix.Field == "" || suffix.Suffix == "" {
			continue
		}

		if val, exists := data[suffix.Field]; exists {
			if strVal, ok := val.(string); ok {
				logging.D(3, "Identified field '%s', trimming '%s'", suffix.Field, suffix.Suffix)
				data[suffix.Field] = strings.TrimSuffix(strVal, suffix.Suffix)
				edited = true
			}
		}
	}
	logging.D(5, "After suffix trim: %v", data)
	return edited, nil
}

// metaAppend appends to the fields in the JSON data
func (rw *JSONFileRW) jsonAppend(data map[string]interface{}, apnd []*models.MetaAppend) (bool, error) {

	logging.D(5, "Entering jsonAppend with data: %v", data)

	if len(apnd) == 0 {
		logging.E(0, "No new suffixes to append", keys.MAppend)
		return false, nil // No replacements to apply
	}

	edited := false
	for _, suffix := range apnd {
		if suffix.Field == "" || suffix.Suffix == "" {
			continue
		}

		if value, exists := data[suffix.Field]; exists {
			if strVal, ok := value.(string); ok {
				logging.D(3, "Identified input JSON field '%v', appending '%v'", suffix.Field, suffix.Suffix)
				strVal += suffix.Suffix
				data[suffix.Field] = strVal
				edited = true
			}
		}
	}
	logging.D(5, "After meta suffix append: %v", data)

	return edited, nil
}

// metaPrefix applies prefixes to the fields in the JSON data
func (rw *JSONFileRW) jsonPrefix(data map[string]interface{}, pfx []*models.MetaPrefix) (bool, error) {

	logging.D(5, "Entering jsonPrefix with data: %v", data)

	if len(pfx) == 0 {
		logging.E(0, "No new prefix replacements found", keys.MPrefix)
		return false, nil // No replacements to apply
	}

	edited := false
	for _, prefix := range pfx {
		if prefix.Field == "" || prefix.Prefix == "" {
			continue
		}

		if value, found := data[prefix.Field]; found {
			if strVal, ok := value.(string); ok {
				logging.D(3, "Identified input JSON field '%v', adding prefix '%v'", prefix.Field, prefix.Prefix)
				strVal = prefix.Prefix + strVal
				data[prefix.Field] = strVal
				edited = true

			}
		}
	}
	logging.D(5, "After adding prefixes: %v", data)

	return edited, nil
}

// addNewField can insert a new field which does not yet exist into the metadata file
func (rw *JSONFileRW) addNewJsonField(data map[string]interface{}, modelOW bool, new []*models.MetaNewField) (bool, error) {

	if len(new) == 0 {
		logging.E(0, "No new field additions found", keys.MNewField)
		return false, nil
	}

	var (
		metaOW,
		metaPS bool
	)

	if !config.IsSet(keys.MOverwrite) && !config.IsSet(keys.MPreserve) {
		metaOW = modelOW
	} else {
		metaOW = config.GetBool(keys.MOverwrite)
		metaPS = config.GetBool(keys.MPreserve)
	}

	logging.D(3, "Retrieved additions for new field data: %v", new)
	processedFields := make(map[string]bool, len(new))

	newAddition := false
	ctx := context.Background()
	for _, addition := range new {
		if addition.Field == "" || addition.Value == "" {
			continue
		}

		// If field doesn't exist at all, add it
		if _, exists := data[addition.Field]; !exists {
			data[addition.Field] = addition.Value
			processedFields[addition.Field] = true
			newAddition = true
			continue
		}
		if !metaOW {

			// Check for context cancellation before proceeding
			select {
			case <-ctx.Done():
				logging.I("Operation canceled for field: %s", addition.Field)
				return false, fmt.Errorf("operation canceled")
			default:
				// Proceed
			}
			if _, alreadyProcessed := processedFields[addition.Field]; alreadyProcessed {
				continue
			}

			if existingValue, exists := data[addition.Field]; exists {

				if !metaOW && !metaPS {
					promptMsg := fmt.Sprintf("Field '%s' already exists with value '%v' in file '%v'. Overwrite? (y/n) to proceed, (Y/N) to apply to whole queue", addition.Field, existingValue, rw.File.Name())

					reply, err := prompt.PromptMetaReplace(promptMsg, metaOW, metaPS)
					if err != nil {
						logging.E(0, err.Error())
					}
					switch reply {
					case "Y":
						logging.D(2, "Received meta overwrite reply as 'Y' for %s in %s, falling through to 'y'", existingValue, rw.File.Name())
						config.Set(keys.MOverwrite, true)
						metaOW = true
						fallthrough
					case "y":
						logging.D(2, "Received meta overwrite reply as 'y' for %s in %s", existingValue, rw.File.Name())
						addition.Field = strings.TrimSpace(addition.Field)
						logging.D(3, "Adjusted field from '%s' to '%s'\n", data[addition.Field], addition.Field)

						data[addition.Field] = addition.Value
						processedFields[addition.Field] = true
						newAddition = true

					case "N":
						logging.D(2, "Received meta overwrite reply as 'N' for %s in %s, falling through to 'n'", existingValue, rw.File.Name())
						config.Set(keys.MPreserve, true)
						metaPS = true
						fallthrough
					case "n":
						logging.D(2, "Received meta overwrite reply as 'n' for %s in %s", existingValue, rw.File.Name())
						logging.P("Skipping field '%s'\n", addition.Field)
						processedFields[addition.Field] = true
					}
				} else if metaOW { // FieldOverwrite is set

					data[addition.Field] = addition.Value
					processedFields[addition.Field] = true
					newAddition = true

				} else if metaPS { // FieldPreserve is set
					continue
				}
			}
		} else {
			// Add the field if it doesn't exist yet, or overwrite is true
			data[addition.Field] = addition.Value
			processedFields[addition.Field] = true
			newAddition = true
		}
	}
	logging.D(3, "JSON after transformations: %v", data)

	return newAddition, nil
}

// jsonFieldDateTag sets date tags in designated meta fields
func (rw *JSONFileRW) jsonFieldDateTag(data map[string]interface{}, dateTagMap map[string]*models.MetaDateTag, fd *models.FileData) (bool, error) {

	logging.D(2, "Making metadata date tag for '%s'...", fd.OriginalVideoBaseName)

	if len(dateTagMap) == 0 {
		logging.D(3, "No date tag operations to perform")
		return false, nil
	}
	if fd == nil {
		return false, fmt.Errorf("JsonFieldDateTag called with null FileData model")
	}

	edited := false
	for field, dateTag := range dateTagMap {
		if dateTag == nil {
			logging.E(0, "Nil date tag configuration for field '%s'", field)
			continue
		}

		val, exists := data[field]
		if !exists {
			logging.D(3, "Field '%s' not found in metadata", field)
			continue
		}

		strVal, ok := val.(string)
		if !ok {
			logging.D(3, "Field '%s' is not a string value, type: %T", field, val)
			continue
		}

		// Generate the date tag
		tag, err := tags.MetafieldDateTag(data, strVal, dateTag.Format)
		if err != nil {
			return false, fmt.Errorf("failed to generate date tag for field '%s': %w", field, err)
		}
		if len(tag) < 3 {
			return false, fmt.Errorf("generated date tag too short for field '%s': '%s'", field, tag)
		}

		// Apply the tag based on location
		switch dateTag.Loc {
		case enums.DATE_TAG_LOC_PFX:
			data[field] = tag + " " + strVal
			logging.I("Added date tag '%s' as prefix to field '%s'", tag, field)
			edited = true

		case enums.DATE_TAG_LOC_SFX:
			data[field] = strVal + " " + tag
			logging.I("Added date tag '%s' as suffix to field '%s'", tag, field)
			edited = true

		default:
			return false, fmt.Errorf("invalid date tag location enum: %v", dateTag.Loc)
		}
	}

	return edited, nil
}
