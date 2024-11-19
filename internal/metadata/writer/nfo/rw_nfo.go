package metadata

import (
	"bufio"
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"metarr/internal/cfg"
	keys "metarr/internal/domain/keys"
	"metarr/internal/models"
	logging "metarr/internal/utils/logging"
	prompt "metarr/internal/utils/prompt"
	"os"
	"strings"
	"sync"
)

type NFOFileRW struct {
	mu    sync.RWMutex
	Model *models.NFOData
	Meta  string
	File  *os.File
}

// NewNFOFileRW creates a new instance of the NFO file reader/writer
func NewNFOFileRW(file *os.File) *NFOFileRW {
	logging.D(3, "Retrieving new meta writer/rewriter for file %q...", file.Name())
	return &NFOFileRW{
		File: file,
	}
}

// DecodeMetadata decodes XML from a file into a map, stores, and returns it
func (rw *NFOFileRW) DecodeMetadata(file *os.File) (*models.NFOData, error) {
	rw.mu.Lock()
	defer rw.mu.Unlock()

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
	var input *models.NFOData
	if err := decoder.Decode(&input); err != nil {
		return nil, fmt.Errorf("failed to decode XML: %w", err)
	}

	rw.Model = input
	logging.D(3, "Decoded metadata: %v", rw.Model)

	return rw.Model, nil
}

// RefreshMetadata reloads the metadata map from the file after updates
func (rw *NFOFileRW) RefreshMetadata() (*models.NFOData, error) {

	rw.mu.RLock()
	defer rw.mu.RUnlock()

	if _, err := rw.File.Seek(0, io.SeekStart); err != nil {
		return nil, fmt.Errorf("failed to seek file: %w", err)
	}

	// Decode metadata
	decoder := xml.NewDecoder(rw.File)

	if err := decoder.Decode(&rw.Model); err != nil {
		return nil, fmt.Errorf("failed to decode xml: %w", err)
	}

	logging.D(3, "Decoded metadata: %v", rw.Model)

	return rw.Model, nil
}

// MakeMetaEdits applies a series of transformations and writes the final result to the file
func (rw *NFOFileRW) MakeMetaEdits(data string, file *os.File, fd *models.FileData) (bool, error) {
	// Ensure we have valid XML
	if !strings.Contains(data, "<movie>") {
		return false, fmt.Errorf("invalid XML: missing movie tag")
	}

	var (
		edited, ok bool
		newContent string
		err        error

		trimPfx []models.MetaTrimPrefix
		trimSfx []models.MetaTrimSuffix

		apnd []models.MetaAppend
		pfx  []models.MetaPrefix

		newField []models.MetaNewField

		replace []models.MetaReplace
	)

	// Replacements
	if len(fd.ModelMReplace) > 0 {
		logging.I("Model for file %q making replacements", fd.OriginalVideoBaseName)
		replace = fd.ModelMReplace
	} else if cfg.IsSet(keys.MReplaceText) {
		if replace, ok = cfg.Get(keys.MReplaceText).([]models.MetaReplace); !ok {
			logging.E(0, "Count not retrieve prefix trim, wrong type: '%T'", replace)
		}
	}

	// Field trim
	if len(fd.ModelMTrimPrefix) > 0 {
		logging.I("Model for file %q trimming prefixes", fd.OriginalVideoBaseName)
		trimPfx = fd.ModelMTrimPrefix
	} else if cfg.IsSet(keys.MTrimPrefix) {
		if trimPfx, ok = cfg.Get(keys.MTrimPrefix).([]models.MetaTrimPrefix); !ok {
			logging.E(0, "Count not retrieve prefix trim, wrong type: '%T'", trimPfx)
		}
	}

	if len(fd.ModelMTrimSuffix) > 0 {
		logging.I("Model for file %q trimming suffixes", fd.OriginalVideoBaseName)
		trimSfx = fd.ModelMTrimSuffix
	} else if cfg.IsSet(keys.MTrimSuffix) {
		if trimSfx, ok = cfg.Get(keys.MTrimSuffix).([]models.MetaTrimSuffix); !ok {
			logging.E(0, "Count not retrieve suffix trim, wrong type: '%T'", trimSfx)
		}
	}

	// Append and prefix
	if len(fd.ModelMAppend) > 0 {
		logging.I("Model for file %q adding appends", fd.OriginalVideoBaseName)
		apnd = fd.ModelMAppend
	} else if cfg.IsSet(keys.MAppend) {
		if apnd, ok = cfg.Get(keys.MAppend).([]models.MetaAppend); !ok {
			logging.E(0, "Count not retrieve appends, wrong type: '%T'", apnd)
		}
	}

	if len(fd.ModelMPrefix) > 0 {
		logging.I("Model for file %q adding prefixes", fd.OriginalVideoBaseName)
		pfx = fd.ModelMPrefix
	} else if cfg.IsSet(keys.MPrefix) {
		if pfx, ok = cfg.Get(keys.MPrefix).([]models.MetaPrefix); !ok {
			logging.E(0, "Count not retrieve prefix, wrong type: '%T'", pfx)
		}
	}

	// New fields
	if len(fd.ModelMNewField) > 0 {
		logging.I("Model for file %q applying preset new field additions", fd.OriginalVideoBaseName)
		newField = fd.ModelMNewField
	} else if cfg.IsSet(keys.MNewField) {
		if newField, ok = cfg.Get(keys.MNewField).([]models.MetaNewField); !ok {
			logging.E(0, "Could not retrieve new fields, wrong type: '%T'", pfx)
		}
	}

	// Make edits:
	// Replace
	if len(replace) > 0 {
		if newContent, ok, err = rw.replaceXml(data, replace); err != nil {
			logging.E(0, err.Error())
		} else if ok {
			edited = true
		}
	}

	// Trim
	if len(trimPfx) > 0 {
		if newContent, ok, err = rw.trimXmlPrefix(data, trimPfx); err != nil {
			logging.E(0, err.Error())
		} else if ok {
			edited = true
		}
	}

	if len(trimSfx) > 0 {
		if newContent, ok, err = rw.trimXmlSuffix(data, trimSfx); err != nil {
			logging.E(0, err.Error())
		} else if ok {
			edited = true
		}
	}

	// Append and prefix
	if len(apnd) > 0 {
		if newContent, ok, err = rw.xmlAppend(data, apnd); err != nil {
			logging.E(0, err.Error())
		} else if ok {
			edited = true
		}
	}

	if len(pfx) > 0 {
		if newContent, ok, err = rw.xmlPrefix(data, pfx); err != nil {
			logging.E(0, err.Error())
		} else if ok {
			edited = true
		}
	}

	// Add new
	if len(newField) > 0 {
		if newContent, ok, err = rw.addNewXmlFields(data, fd.ModelMOverwrite, newField); err != nil {
			logging.E(0, err.Error())
		} else if ok {
			edited = true
		}
	}

	// Only write if changes were made
	if edited {
		if err := rw.writeMetadataToFile(file, []byte(newContent)); err != nil {
			return false, fmt.Errorf("failed to refresh metadata: %w", err)
		}
	}

	return edited, nil
}

// Helper function to ensure XML structure
func (rw *NFOFileRW) ensureXMLStructure(content string) string {
	// Ensure XML declaration
	if !strings.HasPrefix(content, "<?xml") {

		content = fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
%s`, content)
	}

	// Ensure movie tag exists
	if !strings.Contains(content, "<movie>") {
		content = strings.TrimSpace(content)
		content = fmt.Sprintf("%s\n<movie>\n</movie>", content)
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

// replaceMeta applies meta replacement to the fields in the xml data
func (rw *NFOFileRW) replaceXml(data string, replace []models.MetaReplace) (dataRtn string, edited bool, err error) {

	logging.D(5, "Entering replaceXml with data: %v", string(data))

	if len(replace) == 0 {
		return data, false, nil // No replacements to apply
	}

	for _, replacement := range replace {
		if replacement.Field == "" || replacement.Value == "" {
			continue
		}

		startTag := fmt.Sprintf("<%s>", replacement.Field)
		endTag := fmt.Sprintf("</%s>", replacement.Field)

		startIdx := strings.Index(data, startTag)
		endIdx := strings.Index(data, endTag)
		if startIdx == -1 || endIdx == -1 {
			continue // One or both tags missing
		}

		contentStart := startIdx + len(startTag)
		content := strings.TrimSpace(data[contentStart:endIdx])

		logging.D(2, "Identified input xml field %q, replacing %q with %q", replacement.Field, replacement.Value, replacement.Replacement)

		content = strings.ReplaceAll(content, replacement.Value, replacement.Replacement)
		data = data[:contentStart] + content + data[endIdx:]
		edited = true
	}
	logging.D(5, "After meta replacements: %v", data)
	return data, edited, nil
}

// trimMetaPrefix applies meta replacement to the fields in the xml data
func (rw *NFOFileRW) trimXmlPrefix(data string, trimPfx []models.MetaTrimPrefix) (dataRtn string, edited bool, err error) {

	logging.D(5, "Entering trimXmlPrefix with data: %v", string(data))

	if len(trimPfx) == 0 {
		return data, false, nil // No replacements to apply
	}

	for _, prefix := range trimPfx {
		if prefix.Field == "" || prefix.Prefix == "" {
			continue
		}

		startTag := fmt.Sprintf("<%s>", prefix.Field)
		endTag := fmt.Sprintf("</%s>", prefix.Field)

		startIdx := strings.Index(data, startTag)
		endIdx := strings.Index(data, endTag)
		if startIdx == -1 || endIdx == -1 {
			continue // One or both tags missing
		}

		contentStart := startIdx + len(startTag)
		content := strings.TrimSpace(data[contentStart:endIdx])

		logging.D(2, "Identified input xml field %q, trimming prefix %q", prefix.Field, prefix.Prefix)

		content = strings.TrimPrefix(content, prefix.Prefix)
		data = data[:contentStart] + content + data[endIdx:]
		edited = true
	}
	logging.D(5, "After trimming prefixes: %v", data)
	return data, edited, nil
}

// trimMetaSuffix trims specified
func (rw *NFOFileRW) trimXmlSuffix(data string, trimSfx []models.MetaTrimSuffix) (dataRtn string, edited bool, err error) {

	logging.D(5, "Entering trimXmlSuffix with data: %v", string(data))

	if len(trimSfx) == 0 {
		return data, false, nil // No replacements to apply
	}

	for _, suffix := range trimSfx {
		if suffix.Field == "" || suffix.Suffix == "" {
			continue
		}

		startTag := fmt.Sprintf("<%s>", suffix.Field)
		endTag := fmt.Sprintf("</%s>", suffix.Field)

		startIdx := strings.Index(data, startTag)
		endIdx := strings.Index(data, endTag)
		if startIdx == -1 || endIdx == -1 {
			continue // One or both tags missing
		}

		contentStart := startIdx + len(startTag)
		content := strings.TrimSpace(data[contentStart:endIdx])

		logging.D(2, "Identified input xml field %q, trimming suffix %q", suffix.Field, suffix.Suffix)

		content = strings.TrimSuffix(content, suffix.Suffix)
		data = data[:contentStart] + content + data[endIdx:]
		edited = true
	}
	logging.D(5, "After meta replacements: %v", data)
	return data, edited, nil
}

// trimMetaPrefix applies meta replacement to the fields in the xml data
func (rw *NFOFileRW) xmlPrefix(data string, pfx []models.MetaPrefix) (dataRtn string, edited bool, err error) {

	logging.D(5, "Entering xmlPrefix with data: %v", string(data))

	if len(pfx) == 0 {
		return data, false, nil // No replacements to apply
	}

	for _, prefix := range pfx {
		if prefix.Field == "" || prefix.Prefix == "" {
			continue
		}

		startTag := fmt.Sprintf("<%s>", prefix.Field)
		endTag := fmt.Sprintf("</%s>", prefix.Field)

		startIdx := strings.Index(data, startTag)
		endIdx := strings.Index(data, endTag)
		if startIdx == -1 || endIdx == -1 {
			continue // One or both tags missing
		}

		contentStart := startIdx + len(startTag)
		content := strings.TrimSpace(data[contentStart:endIdx])

		logging.D(2, "Identified input xml field %q, adding prefix %q", prefix.Field, prefix.Prefix)

		data = data[:contentStart] + prefix.Prefix + content + data[endIdx:]
		edited = true
	}
	logging.D(5, "After trimming prefixes: %v", data)
	return data, edited, nil
}

// trimMetaSuffix trims specified
func (rw *NFOFileRW) xmlAppend(data string, apnd []models.MetaAppend) (dataRtn string, edited bool, err error) {

	logging.D(5, "Entering xmlAppend with data: %v", string(data))

	if len(apnd) == 0 {
		return data, false, nil // No replacements to apply
	}

	for _, append := range apnd {
		if append.Field == "" || append.Suffix == "" {
			continue
		}

		startTag := fmt.Sprintf("<%s>", append.Field)
		endTag := fmt.Sprintf("</%s>", append.Field)

		startIdx := strings.Index(data, startTag)
		endIdx := strings.Index(data, endTag)
		if startIdx == -1 || endIdx == -1 {
			continue // One or both tags missing
		}

		contentStart := startIdx + len(startTag)
		content := strings.TrimSpace(data[contentStart:endIdx])

		logging.D(2, "Identified input xml field %q, appending suffix %q", append.Field, append.Suffix)

		data = data[:contentStart] + content + append.Suffix + data[endIdx:]
		edited = true
	}
	logging.D(5, "After meta replacements: %v", data)
	return data, edited, nil
}

// addNewField can insert a new field which does not yet exist into the metadata file
func (rw *NFOFileRW) addNewXmlFields(data string, ow bool, newField []models.MetaNewField) (dataRtn string, newAddition bool, err error) {

	var (
		metaOW,
		metaPS bool
	)

	logging.D(5, "Entering addNewXmlFields with data: %v", string(data))

	if len(newField) == 0 {
		return data, false, nil // No replacements to apply
	}

	if ow {
		metaOW = true
	} else {
		metaOW = cfg.GetBool(keys.MOverwrite)
		metaPS = cfg.GetBool(keys.MPreserve)
	}

	logging.D(3, "Retrieved additions for new field data: %v", newField)

	ctx := context.Background()

	for _, addition := range newField {
		if addition.Field == "" || addition.Value == "" {
			continue
		}

		// Special handling for actor fields
		if addition.Field == "actor" {
			// Check if actor already exists
			flatData := rw.flattenField(data)
			actorNameCheck := fmt.Sprintf("<name>%s</name>", rw.flattenField(addition.Value))

			if strings.Contains(flatData, actorNameCheck) {
				logging.I("Actor %q is already inserted in the metadata, no need to add...", addition.Value)
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
				logging.I("Operation canceled for field: %s", addition.Field)
				return data, false, fmt.Errorf("operation canceled")
			default:
				// Proceed
			}

			if !metaOW && !metaPS {
				promptMsg := fmt.Sprintf("Field %q already exists with value '%v' in file '%v'. Overwrite? (y/n) to proceed, (Y/N) to apply to whole queue",
					addition.Field, content, rw.File.Name())

				reply, err := prompt.PromptMetaReplace(promptMsg, metaOW, metaPS)
				if err != nil {
					logging.E(0, err.Error())
				}

				switch reply {
				case "Y":
					cfg.Set(keys.MOverwrite, true)
					metaOW = true
					fallthrough
				case "y":
					data = data[:startContent] + addition.Value + data[endIdx:]
					newAddition = true
				case "N":
					cfg.Set(keys.MPreserve, true)
					metaPS = true
					fallthrough
				case "n":
					logging.D(2, "Skipping field: %s", addition.Field)
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
			logging.E(0, "Invalid XML structure: no movie tag found")
			return data, false
		}

		movieEnd := strings.Index(data, "</movie>")
		if movieEnd == -1 {
			logging.E(0, "Invalid XML structure: no closing movie tag found")
			return data, false
		}

		// Create new cast section
		newCast := fmt.Sprintf("    <cast>\n        <actor>\n            <name>%s</name>\n        </actor>\n    </cast>", name)

		// Find the right spot to insert
		contentStart := movieStart + len("<movie>")
		if contentStart >= len(data) {
			logging.E(0, "Invalid XML structure: movie tag at end of data")
			return data, false
		}

		return data[:contentStart] + "\n" + newCast + "\n" + data[contentStart:], true
	}

	// Cast exists, validate indices
	if castStart == -1 || castEnd == -1 || castStart >= len(data) || castEnd > len(data) {
		logging.E(0, "Invalid XML structure: mismatched cast tags")
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
