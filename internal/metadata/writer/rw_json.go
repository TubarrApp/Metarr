package metadata

import (
	"Metarr/internal/config"
	keys "Metarr/internal/domain/keys"
	"Metarr/internal/models"
	backup "Metarr/internal/utils/fs/backup"
	logging "Metarr/internal/utils/logging"
	prompt "Metarr/internal/utils/prompt"
	"context"
	"encoding/json"
	"fmt"
	"io"
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
	logging.PrintD(3, "Retrieving new meta writer/rewriter for file '%s'...", file.Name())
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
		logging.PrintD(3, "Metadata not stored, is blank: %v", input)
	default:
		rw.Meta = input
		logging.PrintD(3, "Decoded and stored metadata: %v", rw.Meta)
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

	logging.PrintD(3, "Decoded metadata: %v", rw.Meta)

	return rw.Meta, nil
}

// WriteMetadata inserts metadata into the JSON file from a map
func (rw *JSONFileRW) WriteMetadata(fieldMap map[string]*string) (map[string]interface{}, error) {

	rw.mu.Lock()
	defer rw.mu.Unlock()

	logging.PrintD(3, "Entering WriteMetadata for file '%s'", rw.File.Name())
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
		if value != nil && *value != "" {
			currentVal, exists := rw.Meta[field]
			if !exists {
				logging.PrintD(3, "Adding new field '%s' with value '%s'", field, *value)
				rw.Meta[field] = *value
				updated = true
			} else if currentStrVal, ok := currentVal.(string); !ok || currentStrVal != *value || metaOW {
				logging.PrintD(3, "Updating field '%s' from '%v' to '%s'", field, currentVal, *value)
				rw.Meta[field] = *value
				updated = true
			} else {
				logging.PrintD(3, "Skipping field '%s' - value unchanged and overwrite not forced", field)
			}
		}
	}

	// Return if no updates
	if !updated {
		logging.PrintD(2, "No fields were updated")
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

	logging.PrintD(3, "Successfully updated JSON file with new metadata")
	return rw.Meta, nil
}

// MakeMetaEdits applies a series of transformations and writes the final result to the file
func (rw *JSONFileRW) MakeMetaEdits(data map[string]interface{}, file *os.File) (bool, error) {

	var (
		edited, ok bool
		pfx        []models.MetaReplacePrefix
		sfx        []models.MetaReplaceSuffix
		new        []models.MetaNewField
	)

	if config.IsSet(keys.MReplacePfx) {
		pfx, ok = config.Get(keys.MReplacePfx).([]models.MetaReplacePrefix)
		if !ok {
			logging.PrintE(0, "Could not retrieve prefixes, wrong type: '%T'", pfx)
		}
	}
	if config.IsSet(keys.MReplaceSfx) {
		sfx, ok = config.Get(keys.MReplaceSfx).([]models.MetaReplaceSuffix)
		if !ok {
			logging.PrintE(0, "Could not retrieve suffixes, wrong type: '%T'", pfx)
		}
	}
	if config.IsSet(keys.MNewField) {
		new, ok = config.Get(keys.MNewField).([]models.MetaNewField)
		if !ok {
			logging.PrintE(0, "Could not retrieve new fields, wrong type: '%T'", pfx)
		}
	}

	if len(pfx) > 0 {
		newPrefix, err := rw.replaceMetaPrefix(data)
		if err != nil {
			logging.PrintE(0, err.Error())
		}
		if newPrefix {
			edited = true
		}
	}
	logging.PrintD(3, "After meta prefix replace: %v", data)

	if len(sfx) > 0 {
		newSuffix, err := rw.replaceMetaSuffix(data)
		if err != nil {
			logging.PrintE(0, err.Error())
		}
		if newSuffix {
			edited = true
		}
	}
	logging.PrintD(3, "After meta suffix replace: %v", data)

	if len(new) > 0 {
		newField, err := rw.addNewMetaField(data)
		if err != nil {
			logging.PrintE(0, err.Error())
		}
		if newField {
			edited = true
		}
	}

	logging.PrintD(3, "JSON after transformations: %v", data)

	// Marshal the updated JSON back to a byte slice
	updatedFileContent, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return false, fmt.Errorf("failed to marshal updated JSON: %w", err)
	}

	if err = rw.writeMetadataToFile(file, updatedFileContent); err != nil {
		return false, fmt.Errorf("failed to write updated JSON to file: %w", err)
	}

	fmt.Println()
	logging.PrintS(0, "Successfully applied metadata edits to: %v", file.Name())

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

	logging.PrintD(3, "Decoded metadata: %v", rw.Meta)
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

// replaceMetaSuffix applies suffix replacement to the fields in the JSON data
func (rw *JSONFileRW) replaceMetaSuffix(data map[string]interface{}) (bool, error) {

	sfx, ok := config.Get(keys.MReplaceSfx).([]models.MetaReplaceSuffix)
	if !ok {
		logging.PrintE(0, "Could not retrieve prefixes, wrong type: '%T'", sfx)
	}

	logging.PrintD(3, "Entering replaceMetaSuffix with data: %v", data)

	if len(sfx) == 0 {
		return false, nil // No replacements to apply
	}

	newAddition := false
	for _, replace := range sfx {
		if replace.Field == "" || replace.Suffix == "" {
			continue
		}

		if value, found := data[replace.Field]; found {

			if strValue, ok := value.(string); ok {

				logging.PrintD(2, "Identified input JSON field '%v', trimming off '%v'", value, replace.Suffix)

				if strings.HasSuffix(strValue, replace.Suffix) {
					newValue := strings.TrimSuffix(strValue, replace.Suffix) + replace.Replacement
					newValue = strings.TrimSpace(newValue)

					logging.PrintD(2, "Changing '%v' to new value '%v'", replace.Field, newValue)
					data[replace.Field] = newValue
					newAddition = true
				}
			}
		}
	}
	return newAddition, nil
}

// replaceMetaPrefix applies prefix replacement to the fields in the JSON data
func (rw *JSONFileRW) replaceMetaPrefix(data map[string]interface{}) (bool, error) {

	pfx, ok := config.Get(keys.MReplacePfx).([]models.MetaReplacePrefix)
	if !ok {
		logging.PrintE(0, "Could not retrieve prefixes, wrong type: '%T'", pfx)
	}
	logging.PrintD(2, "Entering replaceMetaPrefix with data: %v", data)
	if len(pfx) == 0 {
		return false, nil // No replacements to apply
	}

	newAddition := false
	for _, replace := range pfx {
		if replace.Field == "" || replace.Prefix == "" {
			continue
		}

		if value, found := data[replace.Field]; found {
			if strValue, ok := value.(string); ok {

				if strings.HasPrefix(strValue, replace.Prefix) {
					newValue := strings.TrimPrefix(strValue, replace.Prefix) + replace.Replacement
					newValue = strings.TrimSpace(newValue)
					data[replace.Field] = newValue
					newAddition = true
				}
			}
		}
	}
	return newAddition, nil
}

// addNewField can insert a new field which does not yet exist into the metadata file
func (rw *JSONFileRW) addNewMetaField(data map[string]interface{}) (bool, error) {

	new, ok := config.Get(keys.MNewField).([]models.MetaNewField)
	if !ok {
		logging.PrintE(0, "Could not retrieve prefixes, wrong type: '%T'", new)
	}
	metaOW := config.GetBool(keys.MOverwrite)
	metaPS := config.GetBool(keys.MPreserve)

	if len(new) == 0 {
		logging.PrintD(2, "Key %s is not set in Viper", keys.MNewField)
		return false, nil
	}

	logging.PrintD(3, "Retrieved additions for new field data: %v", new)
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
				logging.PrintI("Operation canceled for field: %s", addition.Field)
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
						logging.PrintE(0, err.Error())
					}
					switch reply {
					case "Y":
						logging.PrintD(2, "Received meta overwrite reply as 'Y' for %s in %s, falling through to 'y'", existingValue, rw.File.Name())
						config.Set(keys.MOverwrite, true)
						metaOW = true
						fallthrough
					case "y":
						logging.PrintD(2, "Received meta overwrite reply as 'y' for %s in %s", existingValue, rw.File.Name())
						addition.Field = strings.TrimSpace(addition.Field)
						logging.PrintD(3, "Adjusted field from '%s' to '%s'\n", data[addition.Field], addition.Field)

						data[addition.Field] = addition.Value
						processedFields[addition.Field] = true
						newAddition = true

					case "N":
						logging.PrintD(2, "Received meta overwrite reply as 'N' for %s in %s, falling through to 'n'", existingValue, rw.File.Name())
						config.Set(keys.MPreserve, true)
						metaPS = true
						fallthrough
					case "n":
						logging.PrintD(2, "Received meta overwrite reply as 'n' for %s in %s", existingValue, rw.File.Name())
						logging.Print("Skipping field '%s'\n", addition.Field)
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
	return newAddition, nil
}
