package metawriters

import (
	"bufio"
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"metarr/internal/abstractions"
	"metarr/internal/domain/keys"
	"metarr/internal/models"
	"metarr/internal/utils/logging"
	"metarr/internal/utils/prompt"
	"os"
	"strings"
	"sync"
)

// NFOFileRW is used to write NFO files.
type NFOFileRW struct {
	ctx   context.Context
	mu    sync.Mutex
	Model *models.NFOData
	Meta  string
	File  *os.File
}

// NewNFOFileRW creates a new instance of the NFO file reader/writer.
func NewNFOFileRW(ctx context.Context, file *os.File) *NFOFileRW {
	logging.D(3, "Retrieving new meta writer/rewriter for file %q...", file.Name())
	return &NFOFileRW{
		ctx:  ctx,
		File: file,
	}
}

// DecodeMetadata decodes XML from a file into a map, stores, and returns it.
func (rw *NFOFileRW) DecodeMetadata(file *os.File) (*models.NFOData, error) {
	rw.mu.Lock()
	defer rw.mu.Unlock()

	// Read entire file content
	content, err := io.ReadAll(file)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	// Ensure XML structure exists
	contentStr := rw.ensureXMLStructure(string(content))
	content = []byte(contentStr)

	// Decode into NFOData model
	var input models.NFOData
	if err := xml.Unmarshal(content, &input); err != nil {
		return nil, fmt.Errorf("failed to decode XML: %w", err)
	}

	rw.Model = &input
	rw.Meta = string(content)

	// Reset file pointer for future operations
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return nil, fmt.Errorf("failed to seek file: %w", err)
	}

	logging.D(3, "Decoded metadata: %+v", rw.Model)
	return rw.Model, nil
}

// RefreshMetadata reloads the metadata map from the file after updates.
func (rw *NFOFileRW) RefreshMetadata() (*models.NFOData, error) {
	rw.mu.Lock()
	defer rw.mu.Unlock()

	if rw.Model == nil {
		return nil, errors.New("NFOFileRW's stored metadata map is empty or null, decode must be called first")
	}

	// Reset file pointer
	if _, err := rw.File.Seek(0, io.SeekStart); err != nil {
		return nil, fmt.Errorf("failed to seek file: %w", err)
	}

	// Read file content
	content, err := io.ReadAll(rw.File)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	// Decode XML into model
	var input models.NFOData
	if err := xml.Unmarshal(content, &input); err != nil {
		return nil, fmt.Errorf("failed to decode XML: %w", err)
	}

	rw.Model = &input
	rw.Meta = string(content)

	logging.D(3, "Refreshed metadata: %+v", rw.Model)
	return rw.Model, nil
}

// MakeMetaEdits applies a series of transformations and writes the final result to the file.
func (rw *NFOFileRW) MakeMetaEdits(data string, file *os.File, fd *models.FileData) (bool, error) {
	// Ensure we have valid XML
	if !strings.Contains(data, "<movie>") {
		return false, errors.New("invalid XML: missing movie tag")
	}

	var (
		edited, ok bool
		newContent string
		err        error

		trimPfx   []models.MetaReplacePrefix
		trimSfx   []models.MetaReplaceSuffix
		apnd      []models.MetaAppend
		pfx       []models.MetaPrefix
		newField  []models.MetaSetField
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
	if len(fd.MetaOps.ReplacePrefixes) > 0 {
		logging.I("Model for file %q trimming prefixes", fd.OriginalVideoBaseName)
		trimPfx = fd.MetaOps.ReplacePrefixes
	}

	if len(fd.MetaOps.ReplaceSuffixes) > 0 {
		logging.I("Model for file %q trimming suffixes", fd.OriginalVideoBaseName)
		trimSfx = fd.MetaOps.ReplaceSuffixes
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
	if len(fd.MetaOps.SetFields) > 0 {
		logging.I("Model for file %q applying new field additions", fd.OriginalVideoBaseName)
		newField = fd.MetaOps.SetFields
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

	logging.W("Copy to %q and paste from %q not currently implemented.", copyTo, pasteFrom)

	// Make edits:
	// Replace
	if len(replace) > 0 {
		if newContent, ok = rw.replaceXML(data, replace); ok {
			edited = true
		}
	}

	// Trim
	if len(trimPfx) > 0 {
		if newContent, ok = rw.trimXMLPrefix(data, trimPfx); ok {
			edited = true
		}
	}

	if len(trimSfx) > 0 {
		if newContent, ok = rw.trimXMLSuffix(data, trimSfx); ok {
			edited = true
		}
	}

	// Append and prefix
	if len(apnd) > 0 {
		if newContent, ok = rw.xmlAppend(data, apnd); ok {
			edited = true
		}
	}

	if len(pfx) > 0 {
		if newContent, ok = rw.xmlPrefix(data, pfx); ok {
			edited = true
		}
	}

	// Add new
	if len(newField) > 0 {
		if newContent, ok, err = rw.addNewXMLFields(data, fd.ModelMOverwrite, newField); err != nil {
			logging.E("failed to add new XML fields with %+v: %v", newField, err)
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

// Helper function to ensure XML structure.
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

// refreshMetadataInternal safely reloads the metadata from the file.
func (rw *NFOFileRW) refreshMetadataInternal(file *os.File) error {
	rw.mu.Lock()
	defer rw.mu.Unlock()

	// Seek to start
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return fmt.Errorf("failed to seek file: %w", err)
	}

	// Read the full file content
	content, err := io.ReadAll(file)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}

	// If file is empty, initialize an empty model
	if len(content) == 0 {
		rw.Model = &models.NFOData{}
		return nil
	}

	// Initialize Model if nil
	if rw.Model == nil {
		rw.Model = &models.NFOData{}
	}

	// Decode XML from content
	if err := xml.Unmarshal(content, rw.Model); err != nil {
		return fmt.Errorf("failed to decode xml: %w", err)
	}
	return nil
}

// writeMetadataToFile is a private metadata writing helper function.
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

	// Flush **before** reading the file again
	if err := writer.Flush(); err != nil {
		return fmt.Errorf("flush content: %w", err)
	}

	if err := rw.refreshMetadataInternal(file); err != nil {
		return fmt.Errorf("failed to refresh metadata: %w", err)
	}

	return nil
}

// replaceXML applies meta replacement to the fields in the xml data.
func (rw *NFOFileRW) replaceXML(data string, replace []models.MetaReplace) (dataRtn string, edited bool) {
	logging.D(5, "Entering replaceXml with data: %v", data)

	if len(replace) == 0 {
		return data, false // No replacements to apply
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
	return data, edited
}

// trimXMLPrefix applies meta replacement to the fields in the XML data.
func (rw *NFOFileRW) trimXMLPrefix(data string, trimPfx []models.MetaReplacePrefix) (dataRtn string, edited bool) {
	logging.D(5, "Entering trimXmlPrefix with data: %v", data)

	if len(trimPfx) == 0 {
		return data, false // No replacements to apply
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
	return data, edited
}

// trimXMLSuffix trims specified.
func (rw *NFOFileRW) trimXMLSuffix(data string, trimSfx []models.MetaReplaceSuffix) (dataRtn string, edited bool) {
	logging.D(5, "Entering trimXmlSuffix with data: %v", data)

	if len(trimSfx) == 0 {
		return data, false // No replacements to apply
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
	return data, edited
}

// xmlPrefix applies meta replacement to the fields in the XML data.
func (rw *NFOFileRW) xmlPrefix(data string, pfx []models.MetaPrefix) (dataRtn string, edited bool) {

	logging.D(5, "Entering xmlPrefix with data: %v", data)

	if len(pfx) == 0 {
		return data, false // No replacements to apply
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
	return data, edited
}

// xmlAppend appends elements to XML fields.
func (rw *NFOFileRW) xmlAppend(data string, apnd []models.MetaAppend) (dataRtn string, edited bool) {

	logging.D(5, "Entering xmlAppend with data: %v", data)

	if len(apnd) == 0 {
		return data, false // No replacements to apply
	}

	for _, append := range apnd {
		if append.Field == "" || append.Append == "" {
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

		logging.D(2, "Identified input xml field %q, appending suffix %q", append.Field, append.Append)

		data = data[:contentStart] + content + append.Append + data[endIdx:]
		edited = true
	}
	logging.D(5, "After meta replacements: %v", data)
	return data, edited
}

// addNewXMLFields can insert new fields into the NFO metadata file.
func (rw *NFOFileRW) addNewXMLFields(data string, ow bool, newField []models.MetaSetField) (dataRtn string, newAddition bool, err error) {

	var (
		metaOW,
		metaPS bool
	)

	logging.D(5, "Entering addNewXmlFields with data: %v", data)

	if len(newField) == 0 {
		return data, false, nil // No replacements to apply
	}

	if ow {
		metaOW = true
	} else {
		metaOW = abstractions.GetBool(keys.MOverwrite)
		metaPS = abstractions.GetBool(keys.MPreserve)
	}

	logging.D(3, "Retrieved additions for new field data: %v", newField)

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
			case <-rw.ctx.Done():
				return data, false, fmt.Errorf("operation canceled for field %q: %w", addition.Field, rw.ctx.Err())
			default:
			}

			if !metaOW && !metaPS {
				promptMsg := fmt.Sprintf("Field %q already exists with value '%v' in file '%v'. Overwrite? (y/n) to proceed, (Y/N) to apply to whole queue",
					addition.Field, content, rw.File.Name())

				reply, err := prompt.MetaReplace(rw.ctx, promptMsg, metaOW, metaPS)
				if err != nil {
					logging.E("Failed to retrieve reply from user prompt: %v", err)
				}

				switch reply {
				case "Y":
					abstractions.Set(keys.MOverwrite, true)
					metaOW = true
					fallthrough
				case "y":
					data = data[:startContent] + addition.Value + data[endIdx:]
					newAddition = true
				case "N":
					abstractions.Set(keys.MPreserve, true)
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

// addNewField adds a new field into the NFO data.
func (rw *NFOFileRW) addNewField(data, addition string) (string, bool) {
	insertIdx := strings.Index(data, "<movie>")
	insertAfter := insertIdx + len("<movie>")

	if insertIdx != -1 {
		data = data[:insertAfter] + "\n" + addition + "\n" + data[insertAfter:]
	}
	return data, true
}

// addNewActorField adds a new actor into the file.
func (rw *NFOFileRW) addNewActorField(data, name string) (string, bool) {
	castStart := strings.Index(data, "<cast>")
	castEnd := strings.Index(data, "</cast>")

	if castStart == -1 && castEnd == -1 {
		// No cast tag exists, create new structure
		movieStart := strings.Index(data, "<movie>")
		if movieStart == -1 {
			logging.E("Invalid XML structure: no movie tag found")
			return data, false
		}

		movieEnd := strings.Index(data, "</movie>")
		if movieEnd == -1 {
			logging.E("Invalid XML structure: no closing movie tag found")
			return data, false
		}

		// Create new cast section
		newCast := fmt.Sprintf("    <cast>\n        <actor>\n            <name>%s</name>\n        </actor>\n    </cast>", name)

		// Find the right spot to insert
		contentStart := movieStart + len("<movie>")
		if contentStart >= len(data) {
			logging.E("Invalid XML structure: movie tag at end of data")
			return data, false
		}

		return data[:contentStart] + "\n" + newCast + "\n" + data[contentStart:], true
	}

	// Cast exists, validate indices
	if castStart == -1 || castEnd == -1 || castStart >= len(data) || castEnd > len(data) {
		logging.E("Invalid XML structure: mismatched cast tags")
		return data, false
	}

	// Insert new actor
	newActor := fmt.Sprintf("    <actor>\n            <name>%s</name>\n        </actor>", name)

	if castEnd-castStart > 1 {
		// Cast has content, insert with proper spacing
		return data[:castEnd] + newActor + "\n    " + data[castEnd:], true
	}
	// Empty cast tag
	insertPoint := castStart + len("<cast>")
	return data[:insertPoint] + newActor + "\n    " + data[insertPoint:], true
}

// flattenField flattens the metadata field for comparison.
func (rw *NFOFileRW) flattenField(s string) string {

	rtn := strings.TrimSpace(s)
	rtn = strings.ReplaceAll(rtn, " ", "")
	rtn = strings.ReplaceAll(rtn, "\n", "")
	rtn = strings.ReplaceAll(rtn, "\r", "")
	rtn = strings.ReplaceAll(rtn, "\t", "")

	return rtn
}
