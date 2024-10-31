package metadata

import (
	"Metarr/internal/config"
	keys "Metarr/internal/domain/keys"
	"Metarr/internal/types"
	logging "Metarr/internal/utils/logging"
	prompt "Metarr/internal/utils/prompt"
	"bufio"
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
)

var (
	muNWrite   sync.Mutex
	muNRefresh sync.Mutex
	muNDecode  sync.Mutex
)

type NFOFileRW struct {
	Model *types.NFOData
	Meta  string
	File  *os.File
}

// NewNFOFileRW creates a new instance of the NFO file reader/writer
func NewNFOFileRW(file *os.File) *NFOFileRW {
	logging.PrintD(3, "Retrieving new meta writer/rewriter for file '%s'...", file.Name())
	return &NFOFileRW{
		File: file,
	}
}

// DecodeMetadata decodes XML from a file into a map, stores, and returns it
func (rw *NFOFileRW) DecodeMetadata(file *os.File) (*types.NFOData, error) {
	muNDecode.Lock()
	defer muNDecode.Unlock()

	// Read the entire file content first
	content, err := io.ReadAll(file)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	rtn := rw.ensureXMLStructure(string(content))
	if rtn != "" {
		content = []byte(rtn)
	}

	// Store the raw content
	rw.Meta = string(content)

	// Reset file pointer
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return nil, fmt.Errorf("failed to seek file: %w", err)
	}

	// Single decode for the model
	decoder := xml.NewDecoder(file)
	var input *types.NFOData
	if err := decoder.Decode(&input); err != nil {
		return nil, fmt.Errorf("failed to decode XML: %w", err)
	}

	rw.Model = input
	logging.PrintD(3, "Decoded metadata: %v", rw.Model)

	return rw.Model, nil
}

// RefreshMetadata reloads the metadata map from the file after updates
func (rw *NFOFileRW) RefreshMetadata() (*types.NFOData, error) {

	muNRefresh.Lock()
	defer muNRefresh.Unlock()

	if _, err := rw.File.Seek(0, io.SeekStart); err != nil {
		return nil, fmt.Errorf("failed to seek file: %w", err)
	}

	// Decode metadata
	decoder := xml.NewDecoder(rw.File)

	if err := decoder.Decode(&rw.Model); err != nil {
		return nil, fmt.Errorf("failed to decode xml: %w", err)
	}

	logging.PrintD(3, "Decoded metadata: %v", rw.Model)

	return rw.Model, nil
}

// func (rw *NFOFileRW) WriteMetadata(fieldMap map[string]*string) error {

// 	muNWrite.Lock()
// 	defer muNWrite.Unlock()

// 	file := rw.File

// 	if err := file.Truncate(0); err != nil {
// 		return fmt.Errorf("truncate file: %w", err)
// 	}

// 	if _, err := file.Seek(0, io.SeekStart); err != nil {
// 		return fmt.Errorf("seek file: %w", err)
// 	}

// 	// Use buffered writer for efficiency
// 	writer := bufio.NewWriter(file)
// 	if _, err := writer.Write(fieldMap); err != nil {
// 		return fmt.Errorf("write content: %w", err)
// 	}

// 	if err := rw.refreshMetadataInternal(file); err != nil {
// 		return fmt.Errorf("failed to refresh metadata: %w", err)
// 	}

// 	return writer.Flush()
// }

// MakeMetaEdits applies a series of transformations and writes the final result to the file
func (rw *NFOFileRW) MakeMetaEdits(data string, file *os.File) (bool, error) {
	var edited bool

	// Ensure we have valid XML
	if !strings.Contains(data, "<movie>") {
		return false, fmt.Errorf("invalid XML: missing movie tag")
	}

	var (
		ok  bool
		pfx []types.MetaReplacePrefix
		sfx []types.MetaReplaceSuffix
		new []types.MetaNewField
	)

	if config.IsSet(keys.MReplacePfx) {
		pfx, ok = config.Get(keys.MReplacePfx).([]types.MetaReplacePrefix)
		if !ok {
			return false, fmt.Errorf("invalid prefix configuration")
		}
	}
	if config.IsSet(keys.MReplaceSfx) {
		sfx, ok = config.Get(keys.MReplaceSfx).([]types.MetaReplaceSuffix)
		if !ok {
			return false, fmt.Errorf("invalid suffix configuration")
		}
	}
	if config.IsSet(keys.MNewField) {
		new, ok = config.Get(keys.MNewField).([]types.MetaNewField)
		if !ok {
			return false, fmt.Errorf("invalid new field configuration")
		}
	}

	// Track content at each step
	currentContent := data

	// Add new fields first
	if len(new) > 0 {
		modified, updated, err := rw.addMetaFields(currentContent)
		if err != nil {
			logging.PrintE(0, "New field addition error: %v", err)
		}
		if updated {
			currentContent = string(modified)
			edited = true
		}
		logging.PrintD(3, "After new field additions: %s", currentContent)
	}

	// Prefix replacements
	if len(pfx) > 0 {
		modified, updated, err := rw.replaceMetaPrefix(currentContent)
		if err != nil {
			logging.PrintE(0, "Prefix replacement error: %v", err)
		}
		if updated {
			currentContent = string(modified)
			edited = true
		}
	}

	// Suffix replacements
	if len(sfx) > 0 {
		modified, updated, err := rw.replaceMetaSuffix(currentContent)
		if err != nil {
			logging.PrintE(0, "Suffix replacement error: %v", err)
		}
		if updated {
			currentContent = string(modified)
			edited = true
		}
	}

	// Only write if changes were made
	if edited {
		if err := rw.writeMetadataToFile(file, []byte(currentContent)); err != nil {
			return false, fmt.Errorf("failed to refresh metadata: %w", err)
		}
	}

	return edited, nil
}

// Helper function to ensure XML structure
func (rw *NFOFileRW) ensureXMLStructure(content string) string {
	// Ensure XML declaration
	if !strings.HasPrefix(content, "<?xml") {
		content = `<?xml version="1.0" encoding="UTF-8"?>` + "\n" + content
	}

	// Ensure movie tag exists
	if !strings.Contains(content, "<movie>") {
		content = strings.TrimSpace(content)
		content = content + "\n<movie>\n</movie>"
	}

	return content
}

// refreshMetadataInternal is a private metadata refresh function
func (rw *NFOFileRW) refreshMetadataInternal(file *os.File) error {
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return fmt.Errorf("failed to seek file: %w", err)
	}

	if rw.Model == nil {
		return fmt.Errorf("NFOFileRW's stored metadata map is empty or null, did you forget to decode?")
	}

	decoder := xml.NewDecoder(file)
	if err := decoder.Decode(&rw.Model); err != nil {
		return fmt.Errorf("failed to decode xml: %w", err)
	}

	return nil
}

// writeMetadataToFile is a private metadata writing helper function
func (rw *NFOFileRW) writeMetadataToFile(file *os.File, content []byte) error {

	muNWrite.Lock()
	defer muNWrite.Unlock()

	if err := file.Truncate(0); err != nil {
		return fmt.Errorf("truncate file: %w", err)
	}

	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return fmt.Errorf("seek file: %w", err)
	}

	// Use buffered writer for efficiency
	writer := bufio.NewWriter(file)
	if _, err := writer.Write(content); err != nil {
		return fmt.Errorf("write content: %w", err)
	}

	if err := rw.refreshMetadataInternal(file); err != nil {
		return fmt.Errorf("failed to refresh metadata: %w", err)
	}

	return writer.Flush()
}

// replaceMetaSuffix applies suffix replacement to the fields in the xml data
func (rw *NFOFileRW) replaceMetaSuffix(data string) (string, bool, error) {
	sfx, ok := config.Get(keys.MReplaceSfx).([]types.MetaReplaceSuffix)
	if !ok {
		logging.PrintE(0, "Could not retrieve suffixes, wrong type: '%T'", sfx)
	}

	logging.PrintD(3, "Entering replaceMetaSuffix with data: %v", string(data))

	if len(sfx) == 0 {
		return data, false, nil // No replacements to apply
	}

	newAddition := false
	for _, replace := range sfx {
		if replace.Field == "" || replace.Suffix == "" {
			continue
		}

		startTag := fmt.Sprintf("<%s>", replace.Field)
		endTag := fmt.Sprintf("</%s>", replace.Field)

		startIdx := strings.Index(data, startTag)
		endIdx := strings.Index(data, endTag)
		if startIdx == -1 || endIdx == -1 {
			continue // One or both tags missing
		}

		contentStart := startIdx + len(startTag)
		content := strings.TrimSpace(data[contentStart:endIdx])

		logging.PrintD(2, "Identified input xml field '%v', trimming off '%v'", content, replace.Suffix)

		if strings.HasSuffix(data, replace.Suffix) {
			newContent := strings.TrimSuffix(data, replace.Suffix) + replace.Replacement
			newContent = strings.TrimSpace(newContent)

			logging.PrintD(2, "Changing '%v' to new value '%v'", replace.Field, newContent)

			data = data[:contentStart] + newContent + data[endIdx:]
			newAddition = true
		}
	}
	return data, newAddition, nil
}

// replaceMetaPrefix applies Prefix replacement to the fields in the xml data
func (rw *NFOFileRW) replaceMetaPrefix(data string) (string, bool, error) {
	sfx, ok := config.Get(keys.MReplaceSfx).([]types.MetaReplacePrefix)
	if !ok {
		logging.PrintE(0, "Could not retrieve prefixes, wrong type: '%T'", sfx)
	}

	logging.PrintD(3, "Entering replaceMetaPrefix with data: %v", data)

	if len(sfx) == 0 {
		return data, false, nil // No replacements to apply
	}

	newAddition := false
	for _, replace := range sfx {
		if replace.Field == "" || replace.Prefix == "" {
			continue
		}

		startTag := fmt.Sprintf("<%s>", replace.Field)
		endTag := fmt.Sprintf("</%s>", replace.Field)

		startIdx := strings.Index(data, startTag)
		endIdx := strings.Index(data, endTag)
		if startIdx == -1 || endIdx == -1 {
			continue // One or both tags missing
		}

		contentStart := startIdx + len(startTag)
		content := strings.TrimSpace(data[contentStart:endIdx])

		logging.PrintD(2, "Identified input xml field '%v', trimming off '%v'", content, replace.Prefix)

		if strings.HasPrefix(data, replace.Prefix) {
			newContent := strings.TrimPrefix(data, replace.Prefix) + replace.Replacement
			newContent = strings.TrimSpace(newContent)

			logging.PrintD(2, "Changing '%v' to new value '%v'", replace.Field, newContent)

			data = data[:contentStart] + newContent + data[endIdx:]
			newAddition = true
		}
	}
	return data, newAddition, nil
}

// addNewField can insert a new field which does not yet exist into the metadata file
func (rw *NFOFileRW) addMetaFields(data string) (string, bool, error) {
	new, ok := config.Get(keys.MNewField).([]types.MetaNewField)
	if !ok {
		logging.PrintE(0, "Could not retrieve new fields, wrong type: '%T'", new)
	}
	metaOW := config.GetBool(keys.MOverwrite)
	metaPS := config.GetBool(keys.MPreserve)

	if len(new) == 0 {
		logging.PrintD(2, "Key %s is not set in Viper", keys.MNewField)
		return data, false, nil
	}

	logging.PrintD(3, "Retrieved additions for new field data: %v", new)

	newAddition := false
	ctx := context.Background()

	for _, addition := range new {
		if addition.Field == "" || addition.Value == "" {
			continue
		}

		// Special handling for actor fields
		if addition.Field == "actor" {
			// Check if actor already exists
			flatData := rw.flattenField(data)
			actorNameCheck := fmt.Sprintf("<name>%s</name>", rw.flattenField(addition.Value))

			if strings.Contains(flatData, actorNameCheck) {
				logging.PrintI("Actor '%s' is already inserted in the metadata, no need to add...", addition.Value)
			} else {
				if modified, ok := rw.addNewActorField(data, addition.Value); ok {
					data = modified
					newAddition = true
				}
			}
			continue
		}

		// Handle non-actor fields
		tagStart := fmt.Sprintf("<%s>", addition.Field)
		tagEnd := fmt.Sprintf("</%s>", addition.Field)

		startIdx := strings.Index(data, tagStart)
		if startIdx == -1 {
			// Field doesn't exist, add it
			if modified, ok := rw.addNewField(data, fmt.Sprintf("%s%s%s", tagStart, addition.Value, tagEnd)); ok {
				data = modified
				newAddition = true
			}
			continue
		}

		// Field exists, handle overwrite
		if !metaOW {
			startContent := startIdx + len(tagStart)
			endIdx := strings.Index(data, tagEnd)
			content := strings.TrimSpace(data[startContent:endIdx])

			// Check for context cancellation
			select {
			case <-ctx.Done():
				logging.PrintI("Operation canceled for field: %s", addition.Field)
				return data, false, fmt.Errorf("operation canceled")
			default:
				// Proceed
			}

			if !metaOW && !metaPS {
				promptMsg := fmt.Sprintf("Field '%s' already exists with value '%v' in file '%v'. Overwrite? (y/n) to proceed, (Y/N) to apply to whole queue",
					addition.Field, content, rw.File.Name())

				reply, err := prompt.PromptMetaReplace(promptMsg, rw.File.Name(), &metaOW, &metaPS)
				if err != nil {
					logging.PrintE(0, err.Error())
				}

				switch reply {
				case "Y":
					metaOW = true
					fallthrough
				case "y":
					data = data[:startContent] + addition.Value + data[endIdx:]
					newAddition = true
				case "N":
					metaPS = true
					fallthrough
				case "n":
					logging.PrintD(2, "Skipping field: %s", addition.Field)
				}
			} else if metaOW {
				data = data[:startContent] + addition.Value + data[endIdx:]
				newAddition = true
			}
		}
	}

	return data, newAddition, nil
}

// addNewField adds a new field into the NFO
func (rw *NFOFileRW) addNewField(data, addition string) (string, bool) {

	insertIdx := strings.Index(data, "<movie>")
	insertAfter := insertIdx + len("<movie>")

	if insertIdx != -1 {
		data = data[:insertAfter] + "\n" + addition + "\n" + data[insertAfter:]
	}
	return data, true
}

// addNewActorField adds a new actor into the file
func (rw *NFOFileRW) addNewActorField(data, name string) (string, bool) {
	castStart := strings.Index(data, "<cast>")
	castEnd := strings.Index(data, "</cast>")

	if castStart == -1 && castEnd == -1 {
		// No cast tag exists, create new structure
		movieStart := strings.Index(data, "<movie>")
		if movieStart == -1 {
			logging.PrintE(0, "Invalid XML structure: no movie tag found")
			return data, false
		}

		movieEnd := strings.Index(data, "</movie>")
		if movieEnd == -1 {
			logging.PrintE(0, "Invalid XML structure: no closing movie tag found")
			return data, false
		}

		// Create new cast section
		newCast := fmt.Sprintf("    <cast>\n        <actor>\n            <name>%s</name>\n        </actor>\n    </cast>", name)

		// Find the right spot to insert
		contentStart := movieStart + len("<movie>")
		if contentStart >= len(data) {
			logging.PrintE(0, "Invalid XML structure: movie tag at end of data")
			return data, false
		}

		return data[:contentStart] + "\n" + newCast + "\n" + data[contentStart:], true
	}

	// Cast exists, validate indices
	if castStart == -1 || castEnd == -1 || castStart >= len(data) || castEnd > len(data) {
		logging.PrintE(0, "Invalid XML structure: mismatched cast tags")
		return data, false
	}

	// Insert new actor
	newActor := fmt.Sprintf("    <actor>\n            <name>%s</name>\n        </actor>", name)

	if castEnd-castStart > 1 {
		// Cast has content, insert with proper spacing
		return data[:castEnd] + newActor + "\n    " + data[castEnd:], true
	} else {
		// Empty cast tag
		insertPoint := castStart + len("<cast>")
		return data[:insertPoint] + newActor + "\n    " + data[insertPoint:], true
	}
}

// flattenField flattens the metadata field for comparison
func (rw *NFOFileRW) flattenField(s string) string {

	rtn := strings.TrimSpace(s)
	rtn = strings.ReplaceAll(rtn, " ", "")
	rtn = strings.ReplaceAll(rtn, "\n", "")
	rtn = strings.ReplaceAll(rtn, "\r", "")
	rtn = strings.ReplaceAll(rtn, "\t", "")

	return rtn
}
