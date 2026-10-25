package metawriters

import (
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"html"
	"io"
	"metarr/internal/abstractions"
	"metarr/internal/dates"
	"metarr/internal/domain/enums"
	"metarr/internal/domain/keys"
	"metarr/internal/domain/logger"
	"metarr/internal/file"
	"metarr/internal/models"
	"metarr/internal/textedit"
	"metarr/internal/utils/prompt"
	"os"
	"strings"
	"sync"

	"github.com/TubarrApp/gocommon/sharedconsts"
	"github.com/TubarrApp/gocommon/sharedtags"
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
	logger.Pl.D(3, "Retrieving new meta writer/rewriter for file %q...", file.Name())
	return &NFOFileRW{
		ctx:  ctx,
		File: file,
	}
}

// DecodeMetadata decodes XML from a file into a map, stores, and returns it.
func (rw *NFOFileRW) DecodeMetadata(file *os.File) (*models.NFOData, error) {
	rw.mu.Lock()
	defer rw.mu.Unlock()

	// Read entire file content.
	content, err := io.ReadAll(file)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	// Ensure XML structure exists.
	contentStr := rw.ensureXMLStructure(string(content))
	content = []byte(contentStr)

	// Decode into NFOData model.
	var input models.NFOData
	if err := xml.Unmarshal(content, &input); err != nil {
		return nil, fmt.Errorf("failed to decode XML: %w", err)
	}

	rw.Model = &input
	rw.Meta = string(content)

	// Reset file pointer for future operations.
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return nil, fmt.Errorf("failed to seek file: %w", err)
	}

	logger.Pl.D(3, "Decoded metadata: %+v", rw.Model)
	return rw.Model, nil
}

// RefreshMetadata reloads the metadata map from the file after updates.
func (rw *NFOFileRW) RefreshMetadata() (*models.NFOData, error) {
	rw.mu.Lock()
	defer rw.mu.Unlock()

	if rw.Model == nil {
		return nil, errors.New("NFOFileRW's stored metadata map is empty or null, decode must be called first")
	}

	// Reset file pointer.
	if _, err := rw.File.Seek(0, io.SeekStart); err != nil {
		return nil, fmt.Errorf("failed to seek file: %w", err)
	}

	// Read file content.
	content, err := io.ReadAll(rw.File)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	// Decode XML into model.
	var input models.NFOData
	if err := xml.Unmarshal(content, &input); err != nil {
		return nil, fmt.Errorf("failed to decode XML: %w", err)
	}

	rw.Model = &input
	rw.Meta = string(content)

	logger.Pl.D(3, "Refreshed metadata: %+v", rw.Model)
	return rw.Model, nil
}

// MakeMetaEdits applies a series of transformations and writes the final result to the file.
func (rw *NFOFileRW) MakeMetaEdits(data string, file *os.File, fd *models.FileData) (bool, error) {
	// Ensure valid XML.
	if !strings.Contains(data, "<movie>") {
		return false, errors.New("invalid XML: missing movie tag")
	}

	var (
		edited, ok bool
		newContent string
		err        error
	)

	if fd.MetaOps.DatesFrom != "" {
		val, _ := firstXMLContent(data, fd.MetaOps.DatesFrom)
		resolveDatesFrom(fd.MetaOps, val)
	}
	ops := fd.MetaOps.ExpandGroups(models.NFOGroupFields)

	// Make edits, each building on the last, in the same order as JSON.
	// Delete date tags before anything else, so other operations can't move them from where they're looked for.
	for fld, d := range ops.DeleteDateTags {
		if newContent, ok = editXMLField(data, fld, func(s string) (string, bool) {
			_, result := dates.StripDateTags(s, d.Loc)
			result = strings.Join(strings.Fields(result), " ")
			return result, result != s
		}); ok {
			logger.Pl.I("Deleted date tags from XML field %q", fld)
			data, edited = newContent, true
		}
	}

	// 0. "all-x" overrides first, so per-field operations build on them.
	if newContent, ok = rw.overrideXMLFields(data, ops.SetOverrides); ok {
		data, edited = newContent, true
	}

	// 1. Set fields (establishes baseline values).
	if len(ops.SetFields) > 0 {
		logger.Pl.I("Model for file %q applying new field additions", fd.MetaFilePath)
		if newContent, ok, err = rw.addNewXMLFields(data, fd.ModelMOverwrite, ops.SetFields); err != nil {
			logger.Pl.E("failed to add new XML fields with %+v: %v", ops.SetFields, err)
		} else if ok {
			data, edited = newContent, true
		}
	}

	// 2. Copy/paste.
	for _, c := range ops.CopyToFields {
		if newContent, ok = rw.pasteXMLField(data, c.Field, c.Dest); ok {
			data, edited = newContent, true
		}
	}

	for _, p := range ops.PasteFromFields {
		if newContent, ok = rw.pasteXMLField(data, p.Origin, p.Field); ok {
			data, edited = newContent, true
		}
	}

	// 3. Replace.
	if newContent, ok = rw.replaceXML(data, ops.Replaces); ok {
		data, edited = newContent, true
	}

	if newContent, ok = rw.trimXMLPrefix(data, ops.ReplacePrefixes); ok {
		data, edited = newContent, true
	}

	if newContent, ok = rw.trimXMLSuffix(data, ops.ReplaceSuffixes); ok {
		data, edited = newContent, true
	}

	for _, t := range ops.Cuts {
		if newContent, ok = editXMLField(data, t.Field, func(s string) (string, bool) {
			return textedit.Cut(s, t.Delim, t.KeepBefore)
		}); ok {
			data, edited = newContent, true
		}
	}

	for _, r := range ops.RegexReplaces {
		if newContent, ok = editXMLField(data, r.Field, func(s string) (string, bool) {
			result := r.Regex.ReplaceAllString(s, r.Replacement)
			return result, result != s
		}); ok {
			data, edited = newContent, true
		}
	}

	// 4. Prefix and suffix.
	if newContent, ok = rw.xmlPrefix(data, ops.Prefixes); ok {
		data, edited = newContent, true
	}

	if newContent, ok = rw.xmlSuffix(data, ops.Suffixes); ok {
		data, edited = newContent, true
	}

	// Only write if changes were made.
	if edited {
		if err := rw.writeMetadataToFile(file, []byte(data)); err != nil {
			return false, fmt.Errorf("failed to refresh metadata: %w", err)
		}
	}

	return edited, nil
}

// Helper function to ensure XML structure.
func (rw *NFOFileRW) ensureXMLStructure(content string) string {
	// Ensure XML declaration.
	if !strings.HasPrefix(content, "<?xml") {
		content = fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
%s`, content)
	}

	// Ensure movie tag exists.
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

	// Seek to start.
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return fmt.Errorf("failed to seek file: %w", err)
	}

	// Read the full file content.
	content, err := io.ReadAll(file)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}

	// If file is empty, initialize an empty model.
	if len(content) == 0 {
		rw.Model = &models.NFOData{}
		return nil
	}

	// Initialize Model if nil.
	if rw.Model == nil {
		rw.Model = &models.NFOData{}
	}

	// Decode XML from content.
	if err := xml.Unmarshal(content, rw.Model); err != nil {
		return fmt.Errorf("failed to decode xml: %w", err)
	}
	return nil
}

// writeMetadataToFile is a private metadata writing helper function.
func (rw *NFOFileRW) writeMetadataToFile(f *os.File, content []byte) error {
	if err := file.ReplaceContents(f, content); err != nil {
		return err
	}

	// Refresh metadata in struct.
	if err := rw.refreshMetadataInternal(f); err != nil {
		return fmt.Errorf("failed to refresh metadata: %w", err)
	}

	return nil
}

// editXMLField rewrites the text of every <field> element with fn, or the <name> of every <actor>,
// reporting whether fn changed any. Text is unescaped for fn and escaped again after, while
// content holding nested elements is passed as-is.
func editXMLField(data, field string, fn func(string) (string, bool)) (string, bool) {
	if field == "" {
		return data, false
	}
	text := func(raw string) (string, bool) {
		raw = strings.TrimSpace(raw)
		if strings.Contains(raw, "<") {
			return fn(raw)
		}
		result, changed := fn(html.UnescapeString(raw))
		return escapeXML(result), changed
	}

	if field == sharedtags.NActors {
		return editXMLElements(data, field, func(block string) (string, bool) {
			return editXMLElements(block, "name", text)
		})
	}
	return editXMLElements(data, field, text)
}

// editXMLElements rewrites the raw content of every <tag> element with fn, reporting whether fn changed any.
func editXMLElements(data, tag string, fn func(string) (string, bool)) (dataRtn string, edited bool) {
	startTag, endTag := "<"+tag+">", "</"+tag+">"
	for searchFrom := 0; ; {
		startIdx := strings.Index(data[searchFrom:], startTag)
		if startIdx == -1 {
			break
		}
		contentStart := searchFrom + startIdx + len(startTag)
		endIdx := strings.Index(data[contentStart:], endTag)
		if endIdx == -1 {
			break
		}
		contentEnd := contentStart + endIdx

		if content, changed := fn(data[contentStart:contentEnd]); changed {
			data = data[:contentStart] + content + data[contentEnd:]
			contentEnd = contentStart + len(content)
			edited = true
		}
		searchFrom = contentEnd + len(endTag)
	}
	return data, edited
}

// firstXMLContent returns the unescaped text of the first <field> element, or the first actor's name.
func firstXMLContent(data, field string) (val string, found bool) {
	editXMLField(data, field, func(s string) (string, bool) {
		if !found {
			val, found = s, true
		}
		return s, false
	})
	return val, found
}

// setXMLField sets every <tag> element to val without prompting, adding the tag when missing.
func (rw *NFOFileRW) setXMLField(data, tag, val string) (dataRtn string, edited bool) {
	data, edited = editXMLField(data, tag, func(s string) (string, bool) {
		return val, s != val
	})
	if strings.Contains(data, "<"+tag+">") {
		return data, edited
	}

	logger.Pl.I("Adding XML field %q with value %q", tag, val)
	if tag == sharedtags.NActors {
		return rw.addNewActorField(data, escapeXML(val))
	}
	return rw.addNewField(data, "<"+tag+">"+escapeXML(val)+"</"+tag+">")
}

// pasteXMLField sets every dest element to the text of the first origin element, adding dest when missing.
func (rw *NFOFileRW) pasteXMLField(data, origin, dest string) (dataRtn string, edited bool) {
	val, found := firstXMLContent(data, origin)
	if !found || dest == "" {
		logger.Pl.D(2, "XML field %q not found, skipping paste into %q", origin, dest)
		return data, false
	}
	logger.Pl.I("Pasting XML field %q into %q", origin, dest)
	return rw.setXMLField(data, dest, val)
}

// overrideXMLFields sets every tag in each "all-x" group to its override value, adding missing tags.
func (rw *NFOFileRW) overrideXMLFields(data string, overrides map[enums.OverrideMetaType]string) (dataRtn string, edited bool) {
	set := func(val string, tags ...string) {
		for _, tag := range tags {
			var ok bool
			if data, ok = rw.setXMLField(data, tag, val); ok {
				edited = true
			}
		}
	}

	for overrideType, val := range overrides {
		switch overrideType {
		case enums.OverrideMetaCredits:
			set(val, models.NFOGroupFields[sharedconsts.MetaOpAllCredits]...)
		case enums.OverrideMetaDates:
			if len(val) >= 4 {
				set(val[:4], sharedtags.NYear)
			}
			set(val, sharedtags.NPremiereDate, sharedtags.NAired, sharedtags.NReleaseDate, sharedtags.NReleased)
		case enums.OverrideMetaDescriptions:
			set(val, models.NFOGroupFields[sharedconsts.MetaOpAllDescriptions]...)
		case enums.OverrideMetaTitles:
			set(val, models.NFOGroupFields[sharedconsts.MetaOpAllTitles]...)
		}
	}
	return data, edited
}

// DateTagEdits adds date tags to the NFO's fields. MakeMetaEdits deletes date tags first, so one deleted there and
// added here is rebuilt.
//
// Runs after the NFO's dates are filled, since the tag is made from them.
func (rw *NFOFileRW) DateTagEdits(file *os.File, fd *models.FileData) (edited bool, err error) {
	data := rw.Meta
	ops := fd.MetaOps.ExpandGroups(models.NFOGroupFields)

	for fld, d := range ops.DateTags {
		tag, err := dates.MakeDateTag(nil, fd, d.Format)
		if err != nil {
			return false, fmt.Errorf("failed to generate date tag for field %q: %w", fld, err)
		}
		if tag == "" {
			continue
		}

		var ok bool
		if data, ok = editXMLField(data, fld, func(s string) (string, bool) {
			return dates.AddDateTag(s, tag, d.Loc)
		}); ok {
			logger.Pl.I("Added date tag %q to XML field %q", tag, fld)
			edited = true
		}
	}

	if !edited {
		return false, nil
	}
	if err := rw.writeMetadataToFile(file, []byte(data)); err != nil {
		return false, fmt.Errorf("failed to write date tag edits: %w", err)
	}
	return true, nil
}

// escapeXML escapes s for use as XML element content.
func escapeXML(s string) string {
	var b strings.Builder
	if err := xml.EscapeText(&b, []byte(s)); err != nil {
		logger.Pl.E("Failed to escape value %q for XML: %v", s, err)
		return ""
	}
	return b.String()
}

// replaceXML applies meta replacement to the fields in the xml data.
func (rw *NFOFileRW) replaceXML(data string, replace []models.MetaReplace) (dataRtn string, edited bool) {
	for _, r := range replace {
		if r.Value == "" {
			continue
		}
		var ok bool
		if data, ok = editXMLField(data, r.Field, func(s string) (string, bool) {
			result := strings.ReplaceAll(s, r.Value, r.Replacement)
			return result, result != s
		}); ok {
			logger.Pl.D(2, "Replaced %q with %q in XML field %q", r.Value, r.Replacement, r.Field)
			edited = true
		}
	}
	return data, edited
}

// trimXMLPrefix replaces a prefix in the fields in the XML data.
func (rw *NFOFileRW) trimXMLPrefix(data string, trimPfx []models.MetaReplacePrefix) (dataRtn string, edited bool) {
	for _, p := range trimPfx {
		if p.Prefix == "" {
			continue
		}
		var ok bool
		if data, ok = editXMLField(data, p.Field, func(s string) (string, bool) {
			return textedit.ReplacePrefix(s, p.Prefix, p.Replacement)
		}); ok {
			logger.Pl.D(2, "Replaced prefix %q with %q in XML field %q", p.Prefix, p.Replacement, p.Field)
			edited = true
		}
	}
	return data, edited
}

// trimXMLSuffix replaces a suffix in the fields in the XML data.
func (rw *NFOFileRW) trimXMLSuffix(data string, trimSfx []models.MetaReplaceSuffix) (dataRtn string, edited bool) {
	for _, sfx := range trimSfx {
		if sfx.Suffix == "" {
			continue
		}
		var ok bool
		if data, ok = editXMLField(data, sfx.Field, func(s string) (string, bool) {
			return textedit.ReplaceSuffix(s, sfx.Suffix, sfx.Replacement)
		}); ok {
			logger.Pl.D(2, "Replaced suffix %q with %q in XML field %q", sfx.Suffix, sfx.Replacement, sfx.Field)
			edited = true
		}
	}
	return data, edited
}

// xmlPrefix prefixes text onto the fields in the XML data.
func (rw *NFOFileRW) xmlPrefix(data string, pfx []models.MetaPrefix) (dataRtn string, edited bool) {
	fields, values := byField(pfx, func(p models.MetaPrefix) (string, string, bool) { return p.Field, p.Prefix, p.Prefix != "" })
	for _, field := range fields {
		var ok bool
		if data, ok = editXMLField(data, field, func(s string) (string, bool) {
			return textedit.AddPrefixes(s, values[field])
		}); ok {
			logger.Pl.D(2, "Added prefixes %q to XML field %q", values[field], field)
			edited = true
		}
	}
	return data, edited
}

// xmlSuffix suffixes text onto the fields in the XML data.
func (rw *NFOFileRW) xmlSuffix(data string, sfx []models.MetaSuffix) (dataRtn string, edited bool) {
	fields, values := byField(sfx, func(s models.MetaSuffix) (string, string, bool) { return s.Field, s.Suffix, s.Suffix != "" })
	for _, field := range fields {
		var ok bool
		if data, ok = editXMLField(data, field, func(s string) (string, bool) {
			return textedit.AddSuffixes(s, values[field])
		}); ok {
			logger.Pl.D(2, "Added suffixes %q to XML field %q", values[field], field)
			edited = true
		}
	}
	return data, edited
}

// addNewXMLFields can insert new fields into the NFO metadata file.
func (rw *NFOFileRW) addNewXMLFields(data string, ow bool, newField []models.MetaSetField) (dataRtn string, newAddition bool, err error) {

	var (
		metaOW,
		metaPS bool
	)

	logger.Pl.D(5, "Entering addNewXmlFields with data: %v", data)

	if len(newField) == 0 {
		return data, false, nil // No replacements to apply.
	}

	if ow {
		metaOW = true
	} else {
		metaOW = abstractions.GetBool(keys.MOverwrite)
		metaPS = abstractions.GetBool(keys.MPreserve)
	}

	logger.Pl.D(3, "Retrieved additions for new field data: %v", newField)

	for _, addition := range newField {
		if addition.Field == "" || addition.Value == "" {
			continue
		}

		// Special handling for actor fields.
		if addition.Field == "actor" {
			// Check if actor already exists.
			flatData := rw.flattenField(data)
			actorNameCheck := fmt.Sprintf("<name>%s</name>", rw.flattenField(addition.Value))

			if strings.Contains(flatData, actorNameCheck) {
				logger.Pl.I("Actor %q is already inserted in the metadata, no need to add...", addition.Value)
			} else {
				if modified, ok := rw.addNewActorField(data, addition.Value); ok {
					data = modified
					newAddition = true
				}
			}
			continue
		}

		// Handle non-actor fields.
		tagStart := fmt.Sprintf("<%s>", addition.Field)
		tagEnd := fmt.Sprintf("</%s>", addition.Field)

		startIdx := strings.Index(data, tagStart)
		if startIdx == -1 {
			// Field doesn't exist, add it.
			if modified, ok := rw.addNewField(data, fmt.Sprintf("%s%s%s", tagStart, addition.Value, tagEnd)); ok {
				data = modified
				newAddition = true
			}
			continue
		}

		// Field exists, handle overwrite.
		if !metaOW {
			startContent := startIdx + len(tagStart)
			endIdx := strings.Index(data, tagEnd)
			content := strings.TrimSpace(data[startContent:endIdx])

			// Check for context cancellation.
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
					logger.Pl.E("Failed to retrieve reply from user prompt: %v", err)
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
					logger.Pl.D(2, "Skipping field: %s", addition.Field)
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
		// No cast tag exists, create new structure.
		movieStart := strings.Index(data, "<movie>")
		if movieStart == -1 {
			logger.Pl.E("Invalid XML structure: no movie tag found")
			return data, false
		}

		if !strings.Contains(data, "</movie>") {
			logger.Pl.E("Invalid XML structure: no closing movie tag found")
			return data, false
		}

		// Create new cast section.
		newCast := fmt.Sprintf("    <cast>\n        <actor>\n            <name>%s</name>\n        </actor>\n    </cast>", name)

		// Find the right spot to insert.
		contentStart := movieStart + len("<movie>")
		if contentStart >= len(data) {
			logger.Pl.E("Invalid XML structure: movie tag at end of data")
			return data, false
		}

		return data[:contentStart] + "\n" + newCast + "\n" + data[contentStart:], true
	}

	// Cast exists, validate indices.
	if castStart == -1 || castEnd == -1 || castStart >= len(data) || castEnd > len(data) {
		logger.Pl.E("Invalid XML structure: mismatched cast tags")
		return data, false
	}

	// Insert new actor.
	newActor := fmt.Sprintf("    <actor>\n            <name>%s</name>\n        </actor>", name)

	if castEnd-castStart > 1 {
		// Cast has content, insert with proper spacing.
		return data[:castEnd] + newActor + "\n    " + data[castEnd:], true
	}
	// Empty cast tag.
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
