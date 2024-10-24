package naming

import (
	"Metarr/internal/backup"
	"Metarr/internal/cmd"
	"Metarr/internal/keys"
	"Metarr/internal/logging"
	"Metarr/internal/models"
	"Metarr/internal/shared"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/araddon/dateparse"
)

type TransformationFunc func(map[string]interface{}, *models.FileData) error

var (
	FieldOverwrite bool = false
	FieldPreserve  bool = false
)

// makeMetaEdits applies a series of transformations and writes the final result to the file
func MakeMetaEdits(fileContent []byte, jsonData map[string]interface{}, file *os.File, m *models.FileData) error {

	err := json.Unmarshal(fileContent, &jsonData)
	if err != nil {
		return fmt.Errorf("failed to unmarshal JSON: %w", err)
	}

	if cmd.IsSet(keys.MReplacePfx) {
		replaceMetaPrefix(jsonData)
	}

	if cmd.IsSet(keys.MReplaceSfx) {
		replaceMetaSuffix(jsonData)
	}

	if cmd.IsSet(keys.MNewField) {
		addNewMetaField(jsonData, m)
	}

	logging.PrintD(3, "JSON after transformations: %v", jsonData)

	// Marshal the updated JSON back to a byte slice
	updatedFileContent, err := json.MarshalIndent(jsonData, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal updated JSON: %w", err)
	}

	err = writeToFile(file, updatedFileContent)
	if err != nil {
		return fmt.Errorf("failed to write updated JSON to file: %w", err)
	}

	fmt.Println()
	logging.PrintS(0, "Successfully applied metadata edits to: %v", file.Name())
	return nil
}

// replaceMetaSuffix applies suffix replacement to the fields in the JSON data
func replaceMetaSuffix(jsonData map[string]interface{}) error {

	logging.PrintD(3, "Entering replaceMetaSuffix with data: %v", jsonData)

	if !cmd.IsSet(keys.MReplaceSfx) {
		logging.PrintD(2, "Key %s is not set in Viper", keys.MReplaceSfx)
		return nil // No additions to apply
	}

	replacements, ok := cmd.Get(keys.MReplaceSfx).([]models.MetaReplaceSuffix)
	if !ok || len(replacements) == 0 {
		return nil // No replacements to apply
	}

	for _, replace := range replacements {
		if replace.Field == "" || replace.Suffix == "" {
			continue
		}

		if value, found := jsonData[replace.Field]; found {

			if strValue, ok := value.(string); ok {

				logging.PrintD(2, "Identified input JSON field '%v', trimming off '%v'", value, replace.Suffix)

				if strings.HasSuffix(strValue, replace.Suffix) {
					newValue := strings.TrimSuffix(strValue, replace.Suffix) + replace.Replacement
					newValue = strings.TrimSpace(newValue)

					logging.PrintD(2, "Changing '%v' to new value '%v'", replace.Field, newValue)
					jsonData[replace.Field] = newValue
				}
			}
		}
	}
	return nil
}

// replaceMetaPrefix applies prefix replacement to the fields in the JSON data
func replaceMetaPrefix(jsonData map[string]interface{}) error {

	logging.PrintD(2, "Entering replaceMetaPrefix with data: %v", jsonData)

	replacements, ok := cmd.Get(keys.MReplacePfx).([]models.MetaReplacePrefix)
	if !ok || len(replacements) == 0 {
		return nil // No replacements to apply
	}

	for _, replace := range replacements {
		if replace.Field == "" || replace.Prefix == "" {
			continue
		}

		if value, found := jsonData[replace.Field]; found {
			if strValue, ok := value.(string); ok {

				if strings.HasPrefix(strValue, replace.Prefix) {
					newValue := strings.TrimPrefix(strValue, replace.Prefix) + replace.Replacement
					newValue = strings.TrimSpace(newValue)
					jsonData[replace.Field] = newValue
				}
			}
		}
	}
	return nil
}

// addNewField can insert a new field which does not yet exist into the metadata file
func addNewMetaField(jsonData map[string]interface{}, m *models.FileData) error {

	ctx := cmd.Get(keys.Context).(context.Context)

	if !cmd.IsSet(keys.MNewField) {
		logging.PrintD(2, "Key %s is not set in Viper", keys.MNewField)
		return nil
	}
	additions, ok := cmd.Get(keys.MNewField).([]models.MetaNewField)
	if !ok || len(additions) == 0 {
		return nil
	}

	logging.PrintD(3, "Retrieved additions for new field data: %v", additions)
	processedFields := make(map[string]bool)

	for _, addition := range additions {
		if addition.Field == "" || addition.Value == "" {
			continue
		}
		if !FieldOverwrite {

			// Check for context cancellation before proceeding
			select {
			case <-ctx.Done():
				logging.PrintI("Operation canceled for field: %s", addition.Field)
				return fmt.Errorf("operation canceled")
			default:
				// Proceed
			}
			if _, alreadyProcessed := processedFields[addition.Field]; alreadyProcessed {
				continue
			}

			if existingValue, exists := jsonData[addition.Field]; exists {

				if !FieldOverwrite && !FieldPreserve {

					promptMsg := fmt.Sprintf("Field '%s' already exists with value '%v' in file '%v'. Overwrite? (y/n) to proceed, (Y/N) to apply to whole queue", addition.Field, existingValue, m.JSONFilePath)

					reply, err := shared.PromptMetaReplace(promptMsg, m.JSONFilePath, &FieldOverwrite, &FieldPreserve)
					if err != nil {
						logging.PrintE(0, err.Error())
					}

					switch reply {
					case "Y":
						logging.PrintD(2, "Received meta overwrite reply as 'Y' for %s in %s, falling through to 'y'", existingValue, m.JSONFilePath)
						FieldOverwrite = true
						fallthrough
					case "y":
						logging.PrintD(2, logging.PrintD(2, "Received meta overwrite reply as 'y' for %s in %s", existingValue, m.JSONFilePath))
						addition.Field = strings.TrimSpace(addition.Field)
						logging.PrintD(3, "Adjusted field from '%s' to '%s'\n", jsonData[addition.Field], addition.Field)

						jsonData[addition.Field] = addition.Value
						processedFields[addition.Field] = true

					case "N":
						logging.PrintD(2, "Received meta overwrite reply as 'N' for %s in %s, falling through to 'n'", existingValue, m.JSONFilePath)
						FieldPreserve = true
						fallthrough
					case "n":
						logging.PrintD(2, "Received meta overwrite reply as 'n' for %s in %s", existingValue, m.JSONFilePath)
						logging.Print("Skipping field '%s'\n", addition.Field)
						processedFields[addition.Field] = true
					}
				} else if FieldOverwrite { // FieldOverwrite is set

					jsonData[addition.Field] = addition.Value
					processedFields[addition.Field] = true

				} else if FieldPreserve { // FieldPreserve is set

					continue
				}
			}
		} else {
			// Add the field if it doesn't exist yet, or overwrite is true
			jsonData[addition.Field] = addition.Value
			processedFields[addition.Field] = true
		}
	}
	return nil
}

// writeToFile truncates the file and writes the updated content
func writeToFile(file *os.File, content []byte) error {

	if cmd.GetBool(keys.NoFileOverwrite) {
		err := backup.BackupFile(file)
		if err != nil {
			return fmt.Errorf("failed to create a backup of file '%s'", file.Name())
		}
	}

	_, err := file.Seek(0, io.SeekStart)
	if err != nil {
		return fmt.Errorf("failed to seek file: %w", err)
	}

	err = file.Truncate(0) // Clear the file before writing new content
	if err != nil {
		return fmt.Errorf("failed to truncate file: %w", err)
	}

	_, err = file.Write(content)
	if err != nil {
		return fmt.Errorf("failed to write updated JSON to file: %w", err)
	}

	return nil
}

// ParseAndFormatDate parses and formats the inputted date string
func ParseAndFormatDate(dateString string) (string, error) {

	t, err := dateparse.ParseAny(dateString)
	if err != nil {
		return "", fmt.Errorf("unable to parse date: %s", dateString)
	}

	return t.Format("20060102"), nil
}
