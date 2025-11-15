// Package metawriters is used for reading, decoding, and writing JSON files.
package metawriters

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"metarr/internal/abstractions"
	"metarr/internal/dates"
	"metarr/internal/domain/enums"
	"metarr/internal/domain/keys"
	"metarr/internal/file"
	"metarr/internal/models"
	"metarr/internal/parsing"
	"metarr/internal/utils/logging"
	"metarr/internal/utils/prompt"
	"os"
	"strings"
	"sync"
)

// JSONFileRW is used to access JSON reading/writing utilities.
type JSONFileRW struct {
	ctx  context.Context
	mu   sync.Mutex
	Meta map[string]any
	File *os.File
}

// NewJSONFileRW creates a new instance of the JSON file reader/writer.
func NewJSONFileRW(ctx context.Context, file *os.File) *JSONFileRW {
	logging.D(3, "Retrieving new meta writer/rewriter for file %q...", file.Name())
	return &JSONFileRW{
		ctx:  ctx,
		File: file,
		Meta: metaMapPool.Get().(map[string]any),
	}
}

// DecodeJSON parses and stores JSON metadata into a map and returns it.
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
			if _, seekErr := file.Seek(currentPos, io.SeekStart); seekErr != nil {
				logging.E("Failed to seek file %q: %v", file.Name(), seekErr)
			}
		}
	}()

	// Seek start
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return nil, fmt.Errorf("failed to seek file: %w", err)
	}

	// Decode to map
	decoder := json.NewDecoder(file)
	poolData := metaMapPool.Get().(map[string]any)
	clear(poolData)
	defer metaMapPool.Put(poolData)

	if err := decoder.Decode(&poolData); err != nil {
		return nil, fmt.Errorf("failed to decode JSON in DecodeMetadata: %w", err)
	}

	switch {
	case len(poolData) == 0, poolData == nil:
		logging.D(3, "Metadata not stored, is blank: %v", poolData)
		return make(map[string]any), nil

	default:
		result := maps.Clone(poolData)
		rw.updateMeta(result)
		logging.D(5, "Decoded and stored metadata: %v", result)
		success = true
		return result, nil
	}
}

// RefreshJSON reloads the metadata map from the file after updates.
func (rw *JSONFileRW) RefreshJSON() (map[string]any, error) {
	if rw.File == nil {
		return nil, errors.New("file passed in nil")
	}
	return rw.DecodeJSON(rw.File)
}

// WriteJSON inserts metadata into the JSON file from a map.
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
		if err := file.BackupFile(rw.File); err != nil {
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

// MakeJSONEdits applies a series of transformations and writes the final result to the file.
func (rw *JSONFileRW) MakeJSONEdits(file *os.File, fd *models.FileData) (edited bool, err error) {
	if file == nil {
		return false, errors.New("file passed in nil")
	}
	currentMeta := rw.copyMeta()
	logging.D(5, "Entering MakeJSONEdits.\nData: %v", currentMeta)

	filename := rw.File.Name()
	mtp := parsing.NewMetaTemplateParser(file.Name())
	ops := fd.MetaOps

	// 1. Set fields first (establishes baseline values)
	if len(ops.SetFields) > 0 {
		logging.I("Model for file %q applying new field additions", fd.OriginalVideoPath)
		if ok, err := rw.setJSONField(currentMeta, filename, fd.ModelMOverwrite, ops.SetFields, mtp); err != nil {
			logging.E("Failed to set fields with %+v: %v", ops.SetFields, err)
		} else if ok {
			edited = true
		}
	}

	// 2. Copy/Paste operations (move data between fields)
	if len(ops.CopyToFields) > 0 {
		logging.I("Model for file %q copying to fields", ops.CopyToFields)
		if changesMade := rw.copyToField(currentMeta, ops.CopyToFields); changesMade {
			edited = true
		}
	}

	if len(ops.PasteFromFields) > 0 {
		logging.I("Model for file %q pasting from fields", ops.PasteFromFields)
		if changesMade := rw.pasteFromField(currentMeta, ops.PasteFromFields); changesMade {
			edited = true
		}
	}

	// 3. Replace operations (modify existing content)
	if len(ops.Replaces) > 0 {
		logging.I("Model for file %q making replacements", fd.OriginalVideoPath)
		if changesMade := rw.replaceJSON(currentMeta, ops.Replaces, mtp); changesMade {
			edited = true
		}
	}

	if len(ops.ReplacePrefixes) > 0 {
		logging.I("Model for file %q replacing prefixes", fd.OriginalVideoPath)
		if changesMade := rw.replaceJSONPrefix(currentMeta, ops.ReplacePrefixes, mtp); changesMade {
			edited = true
		}
	}

	if len(ops.ReplaceSuffixes) > 0 {
		logging.I("Model for file %q replacing suffixes", fd.OriginalVideoPath)
		if changesMade := rw.replaceJSONSuffix(currentMeta, ops.ReplaceSuffixes, mtp); changesMade {
			edited = true
		}
	}

	// 4. Add content (prefix/append)
	if len(ops.Prefixes) > 0 {
		logging.I("Model for file %q adding prefixes", fd.OriginalVideoPath)
		if changesMade := rw.jsonPrefix(currentMeta, filename, ops.Prefixes, mtp); changesMade {
			edited = true
		}
	}

	if len(ops.Appends) > 0 {
		logging.I("Model for file %q adding appends", fd.OriginalVideoPath)
		if changesMade := rw.jsonAppend(currentMeta, filename, ops.Appends, mtp); changesMade {
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
	currentMeta := rw.copyMeta()

	logging.D(4, "About to perform MakeDateTagEdits operations for file %q", file.Name())

	// Delete date tag first, user's may want to delete and re-build
	if len(fd.MetaOps.DeleteDateTags) > 0 {
		logging.I("Stripping metafield date tags (User entered: %v)", fd.MetaOps.DeleteDateTags)

		if ok, err := rw.jsonFieldDeleteDateTag(currentMeta, fd.MetaOps.DeleteDateTags, fd); err != nil {
			logging.E("failed to delete date tag in %q: %v", fd.MetaFilePath, err)
		} else if ok {
			edited = true
		}
	}

	// Add date tag
	if len(fd.MetaOps.DateTags) > 0 {
		logging.I("Adding metafield date tags (User entered: %v)", fd.MetaOps.DateTags)

		if ok, err := rw.jsonFieldAddDateTag(currentMeta, fd.MetaOps.DateTags, fd); err != nil {
			logging.E("failed to delete date tag in %q: %v", fd.MetaFilePath, err)
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

// replaceJSON makes user defined JSON replacements.
func (rw *JSONFileRW) replaceJSON(j map[string]any, rplce []models.MetaReplace, mtp *parsing.MetaTemplateParser) (edited bool) {
	logging.D(5, "Entering replaceJson with data: %v", j)

	if len(rplce) == 0 {
		logging.E("Called replaceJson without replacements")
		return false
	}
	for _, r := range rplce {
		if r.Field == "" || r.Value == "" {
			continue
		}
		if val, exists := j[r.Field]; exists {
			if strVal, ok := val.(string); ok {
				// Fill tag
				result, isTemplate := mtp.FillMetaTemplateTag(r.Replacement, j)
				if result == r.Replacement && isTemplate {
					continue
				}
				r.Replacement = result

				// Process
				logging.D(3, "Identified field %q, replacing %q with %q", r.Field, r.Value, r.Replacement)
				j[r.Field] = strings.ReplaceAll(strVal, r.Value, r.Replacement)
				edited = true
			}
		}
	}
	logging.D(5, "After JSON replace: %v", j)
	return edited
}

// replaceJSONPrefix trims defined prefixes from specified fields.
func (rw *JSONFileRW) replaceJSONPrefix(j map[string]any, rPfx []models.MetaReplacePrefix, mtp *parsing.MetaTemplateParser) (edited bool) {
	logging.D(5, "Entering trimJsonPrefix with data: %v", j)

	if len(rPfx) == 0 {
		logging.E("Called trimJsonPrefix without prefixes to trim")
		return false
	}
	for _, rp := range rPfx {
		if rp.Field == "" || rp.Prefix == "" {
			continue
		}
		if val, exists := j[rp.Field]; exists {
			if strVal, ok := val.(string); ok {
				// Fill tag
				result, isTemplate := mtp.FillMetaTemplateTag(rp.Prefix, j)
				if result == rp.Prefix && isTemplate {
					continue
				}
				rp.Prefix = result

				// Process
				if !strings.HasPrefix(strVal, rp.Prefix) {
					logging.D(3, "Metafield %q does not contain prefix %q, not making replacement", strVal, rp.Prefix)
					continue
				}
				logging.D(3, "Identified field %q, trimming %q", rp.Field, rp.Prefix)
				j[rp.Field] = rp.Replacement + strings.TrimPrefix(strVal, rp.Prefix)
				edited = true
			}
		}
	}
	logging.D(5, "After prefix trim: %v", j)
	return edited
}

// replaceJSONSuffix trims defined suffixes from specified fields.
func (rw *JSONFileRW) replaceJSONSuffix(j map[string]any, rSfx []models.MetaReplaceSuffix, mtp *parsing.MetaTemplateParser) (edited bool) {
	logging.D(5, "Entering trimJsonSuffix with data: %v", j)

	if len(rSfx) == 0 {
		logging.E("Called trimJsonSuffix without prefixes to trim")
		return false
	}
	for _, rs := range rSfx {
		if rs.Field == "" || rs.Suffix == "" {
			continue
		}
		if val, exists := j[rs.Field]; exists {
			if strVal, ok := val.(string); ok {
				// Fill tag
				result, isTemplate := mtp.FillMetaTemplateTag(rs.Suffix, j)
				if result == rs.Suffix && isTemplate {
					continue
				}
				rs.Suffix = result

				// Process
				if !strings.HasSuffix(strVal, rs.Suffix) {
					logging.D(3, "Metafield %q does not contain suffix %q, not making replacement", strVal, rs.Suffix)
					continue
				}
				logging.D(3, "Identified field %q, trimming %q", rs.Field, rs.Suffix)
				j[rs.Field] = strings.TrimSuffix(strVal, rs.Suffix) + rs.Replacement
				edited = true
			}
		}
	}
	logging.D(5, "After suffix trim: %v", j)
	return edited
}

// jsonAppend appends to the fields in the JSON data.
func (rw *JSONFileRW) jsonAppend(j map[string]any, file string, apnd []models.MetaAppend, mtp *parsing.MetaTemplateParser) (edited bool) {
	logging.D(5, "Entering jsonAppend with data: %v", j)

	if len(apnd) == 0 {
		logging.E("No new suffixes to append for file %q", file)
		return false // No replacements to apply
	}
	for _, a := range apnd {
		if a.Field == "" || a.Append == "" {
			continue
		}
		if value, exists := j[a.Field]; exists {
			if strVal, ok := value.(string); ok {
				// Fill tag
				result, isTemplate := mtp.FillMetaTemplateTag(a.Append, j)
				if result == a.Append && isTemplate {
					continue
				}
				a.Append = result

				// Process
				logging.D(3, "Identified input JSON field '%v', appending '%v'", a.Field, a.Append)
				strVal += a.Append
				j[a.Field] = strVal
				edited = true
			}
		}
	}
	logging.D(5, "After JSON suffix append: %v", j)
	return edited
}

// jsonPrefix applies prefixes to the fields in the JSON data.
func (rw *JSONFileRW) jsonPrefix(j map[string]any, file string, pfx []models.MetaPrefix, mtp *parsing.MetaTemplateParser) (edited bool) {
	logging.D(5, "Entering jsonPrefix with data: %v", j)

	if len(pfx) == 0 {
		logging.E("No new prefix replacements found for file %q", file)
		return false // No replacements to apply
	}
	for _, p := range pfx {
		if p.Field == "" || p.Prefix == "" {
			continue
		}
		if value, found := j[p.Field]; found {
			if strVal, ok := value.(string); ok {
				// Fill tag
				result, isTemplate := mtp.FillMetaTemplateTag(p.Prefix, j)
				if result == p.Prefix && isTemplate {
					continue
				}
				p.Prefix = result

				// Process
				logging.D(3, "Identified input JSON field '%v', adding prefix '%v'", p.Field, p.Prefix)
				strVal = p.Prefix + strVal
				j[p.Field] = strVal
				edited = true

			}
		}
	}
	logging.D(5, "After adding prefixes: %v", j)
	return edited
}

// setJSONField can insert a new field which does not yet exist into the metadata file.
func (rw *JSONFileRW) setJSONField(j map[string]any, file string, ow bool, newField []models.MetaSetField, mtp *parsing.MetaTemplateParser) (edited bool, err error) {
	if len(newField) == 0 {
		logging.E("No new field additions found for file %q", file)
		return false, nil
	}
	var (
		metaOW,
		metaPS bool
	)

	if !abstractions.IsSet(keys.MOverwrite) && !abstractions.IsSet(keys.MPreserve) {
		logging.I("Model is set to overwrite")
		metaOW = ow
	} else {
		metaOW = abstractions.GetBool(keys.MOverwrite)
		metaPS = abstractions.GetBool(keys.MPreserve)
		logging.I("Meta OW: %v Meta Preserve: %v", metaOW, metaPS)
	}

	logging.D(3, "Retrieved additions for new field data: %v", newField)
	processedFields := make(map[string]bool, len(newField))

	newAddition := false
	for _, n := range newField {
		if n.Field == "" || n.Value == "" {
			continue
		}
		n.Value, _ = mtp.FillMetaTemplateTag(n.Value, j)

		// If field doesn't exist at all, add it
		if _, exists := j[n.Field]; !exists {
			j[n.Field] = n.Value
			processedFields[n.Field] = true
			newAddition = true
			continue
		}

		// Field already exists, check with user
		if !metaOW {

			// Check for cancellation
			select {
			case <-rw.ctx.Done():
				return false, fmt.Errorf("operation canceled: %w", rw.ctx.Err())
			default:
			}

			if _, alreadyProcessed := processedFields[n.Field]; alreadyProcessed {
				continue
			}

			if existingValue, exists := j[n.Field]; exists {
				if !metaPS {
					promptMsg := fmt.Sprintf(
						"Field %q already exists with value '%v' in file '%v'. Overwrite? (y/n) to proceed, (Y/N) to apply to whole queue",
						n.Field, existingValue, file,
					)

					reply, err := prompt.MetaReplace(rw.ctx, promptMsg, metaOW, metaPS)
					if err != nil {
						logging.E("Failed to retrieve reply from user prompt: %v", err)
					}

					switch reply {
					case "Y":
						logging.D(2, "Received meta overwrite reply as 'Y' for %s in %s, falling through to 'y'", existingValue, file)
						abstractions.Set(keys.MOverwrite, true)
						metaOW = true
						fallthrough

					case "y":
						logging.D(2, "Received meta overwrite reply as 'y' for %s in %s", existingValue, file)
						n.Field = strings.TrimSpace(n.Field)
						logging.D(3, "Changed field from %q → %q\n", j[n.Field], n.Field)

						j[n.Field] = n.Value
						processedFields[n.Field] = true
						newAddition = true

					case "N":
						logging.D(2, "Received meta overwrite reply as 'N' for %s in %s, falling through to 'n'", existingValue, file)
						abstractions.Set(keys.MPreserve, true)
						metaPS = true
						fallthrough

					case "n":
						logging.D(2, "Received meta overwrite reply as 'n' for %s in %s", existingValue, file)
						logging.P("Skipping field %q\n", n.Field)
						processedFields[n.Field] = true
					}
				}

				switch {
				case metaOW: // EXISTS and FieldOverwrite is set
					j[n.Field] = n.Value
					processedFields[n.Field] = true
					newAddition = true

				case metaPS: // EXISTS and FieldPreserve is set
					continue
				}
			}
		} else {
			// Field does not exist or overwrite is true
			j[n.Field] = n.Value
			processedFields[n.Field] = true
			newAddition = true
		}
	}
	return newAddition, nil
}

// jsonFieldAddDateTag sets date tags in designated meta fields.
func (rw *JSONFileRW) jsonFieldAddDateTag(j map[string]any, addDateTag map[string]models.MetaDateTag, fd *models.FileData) (edited bool, err error) {
	if len(addDateTag) == 0 {
		logging.D(3, "No date tag operations to perform")
		return false, nil
	}
	if fd == nil {
		return false, fmt.Errorf("jsonFieldDateTag called with null FileData model")
	}
	logging.D(2, "Adding metadata date tags for %q...", fd.MetaFilePath)

	// Add date tags
	for fld, d := range addDateTag {
		val, exists := j[fld]
		if !exists {
			logging.D(3, "Field %q not found in metadata", fld)
			continue
		}
		strVal, ok := val.(string)
		if !ok {
			logging.D(3, "Field %q is not a string value, type: %T", fld, val)
			continue
		}

		// Generate the date tag
		tag, err := dates.MakeDateTag(j, fd, d.Format)
		if err != nil {
			return false, fmt.Errorf("failed to generate date tag for field %q: %w", fld, err)
		}
		if tag == "" {
			logging.D(3, "Generated empty date tag for field %q, skipping", fld)
			continue
		}

		// Check if tag already exists
		if strings.Contains(strVal, tag) {
			logging.I("Tag %q already exists in field %q", tag, strVal)
			continue
		}

		// Apply the tag based on location
		var result string
		switch d.Loc {
		case enums.DateTagLocPrefix:
			result = fmt.Sprintf("%s %s", tag, strVal)
		case enums.DateTagLocSuffix:
			result = fmt.Sprintf("%s %s", strVal, tag)
		default:
			return false, fmt.Errorf("invalid date tag location enum: %v", d.Loc)
		}

		result = rw.cleanFieldValue(result)
		j[fld] = result
		logging.I("Added date tag %q to field %q (location: %v)", tag, fld, d.Loc)
		edited = true
	}
	return edited, nil
}

// jsonFieldDeleteDateTag sets date tags in designated meta fields.
func (rw *JSONFileRW) jsonFieldDeleteDateTag(j map[string]any, deleteDateTag map[string]models.MetaDeleteDateTag, fd *models.FileData) (edited bool, err error) {
	if len(deleteDateTag) == 0 {
		logging.D(3, "No delete date tag operations to perform")
		return false, nil
	}
	if fd == nil {
		return false, fmt.Errorf("jsonFieldDateTag called with null FileData model")
	}
	logging.D(2, "Deleting metadata date tags for %q...", fd.OriginalVideoPath)

	// Delete date tags:
	for fld, d := range deleteDateTag {
		val, exists := j[fld]
		if !exists {
			logging.D(3, "Field %q not found in metadata", fld)
			continue
		}

		strVal, ok := val.(string)
		if !ok {
			logging.D(3, "Field %q is not a string value, type: %T", fld, val)
			continue
		}

		before := strVal
		deletedTags, result := dates.StripDateTags(strVal, d.Loc)
		result = rw.cleanFieldValue(result)

		j[fld] = result

		if j[fld] != before {
			logging.I("Deleted date tags %v at from field %q (operation: %v)", deletedTags, fld, d)
			edited = true
		}
	}
	return edited, nil
}

// copyToField copies values from one meta field to another.
func (rw *JSONFileRW) copyToField(j map[string]any, copyTo []models.CopyToField) (edited bool) {
	logging.D(5, "Entering jsonPrefix with data: %v", j)

	if len(copyTo) == 0 {
		logging.E("No new copy operations found")
		return false
	}
	for _, c := range copyTo {
		if c.Field == "" || c.Dest == "" {
			continue
		}
		if value, found := j[c.Field]; found {

			if val, ok := value.(string); ok {
				logging.I("Identified input JSON field '%v', copying to field '%v'", c.Field, c.Dest)
				j[c.Dest] = val
				edited = true

			}
		}
	}
	logging.D(5, "After making copy operation changes: %v", j)
	return edited
}

// pasteFromField copies values from one meta field to another.
func (rw *JSONFileRW) pasteFromField(j map[string]any, paste []models.PasteFromField) (edited bool) {
	logging.D(5, "Entering jsonPrefix with data: %v", j)

	if len(paste) == 0 {
		logging.E("No new paste operations found")
		return false
	}
	for _, p := range paste {
		if p.Field == "" || p.Origin == "" {
			continue
		}
		if value, found := j[p.Origin]; found {

			if val, ok := value.(string); ok {
				logging.I("Identified input JSON field '%v', pasting to field '%v'", p.Origin, p.Field)
				j[p.Field] = val
				edited = true
			}
		}
	}
	logging.D(5, "After making paste operation changes: %v", j)
	return edited
}

// Map buffer.
var metaMapPool = sync.Pool{
	New: func() any {
		return make(map[string]any, 81) // 81 objects in tested JSON file received from yt-dlp
	},
}

// JSON pool buffer.
var jsonBufferPool = sync.Pool{
	New: func() any {
		return bytes.NewBuffer(make([]byte, 0, 4096)) // i.e. 4KiB
	},
}

// writeJSONToFile is a private metadata writing helper function.
func (rw *JSONFileRW) writeJSONToFile(file *os.File, j map[string]any) error {
	if file == nil {
		return errors.New("file passed in nil")
	}
	if j == nil {
		return errors.New("JSON metadata passed in nil")
	}

	// Get buffer from pool
	buf := jsonBufferPool.Get().(*bytes.Buffer)
	buf.Reset()
	defer jsonBufferPool.Put(buf)

	// Create encoder each time (cheap operation)
	encoder := json.NewEncoder(buf)
	encoder.SetIndent("", "  ")

	// Marshal data
	if err := encoder.Encode(j); err != nil {
		return fmt.Errorf("marshal error: %w", err)
	}

	// Begin file ops
	rw.mu.Lock()
	defer rw.mu.Unlock()

	currentPos, err := file.Seek(0, io.SeekCurrent)
	if err != nil {
		return fmt.Errorf("failed to get current position: %w", err)
	}

	success := false
	defer func() {
		if !success {
			if _, seekErr := file.Seek(currentPos, io.SeekStart); seekErr != nil {
				logging.E("Failed to seek file %q: %v", file.Name(), seekErr)
			}
		}
	}()

	// Seek file start
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return fmt.Errorf("failed to seek to beginning of file: %w", err)
	}

	// File ops
	if err := file.Truncate(0); err != nil {
		return fmt.Errorf("failed to truncate file: %w", err)
	}

	if _, err := buf.WriteTo(file); err != nil {
		return fmt.Errorf("failed to write to file: %w", err)
	}

	// Ensure changes are persisted
	if err := file.Sync(); err != nil {
		return fmt.Errorf("failed to sync file: %w", err)
	}

	success = true
	return nil
}

// copyMeta creates a deep copy of the metadata map under lock.
func (rw *JSONFileRW) copyMeta() map[string]any {
	rw.mu.Lock()
	defer rw.mu.Unlock()
	if rw.Meta == nil {
		return make(map[string]any)
	}
	cloned := maps.Clone(rw.Meta)
	if cloned == nil {
		return make(map[string]any)
	}
	return cloned
}

// updateMeta safely updates the metadata map under write lock.
func (rw *JSONFileRW) updateMeta(newMeta map[string]any) {
	if newMeta == nil {
		newMeta = metaMapPool.Get().(map[string]any)
	}

	rw.mu.Lock()
	oldMeta := rw.Meta
	rw.Meta = newMeta
	rw.mu.Unlock()

	if oldMeta != nil {
		clear(oldMeta)
		metaMapPool.Put(oldMeta)
	}
}

// cleanFieldValue trims leading/trailing whitespaces after deletions.
func (rw *JSONFileRW) cleanFieldValue(value string) string {
	cleaned := strings.TrimSpace(value)
	cleaned = strings.Join(strings.Fields(cleaned), " ")
	return cleaned
}
