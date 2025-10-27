package jsonrw

import (
	"errors"
	"fmt"
	"metarr/internal/abstractions"
	"metarr/internal/dates"
	"metarr/internal/domain/enums"
	"metarr/internal/domain/keys"
	"metarr/internal/metadata/metatags"
	"metarr/internal/models"
	"metarr/internal/utils/logging"
	"metarr/internal/utils/prompt"
	"strings"
)

// replaceJSON makes user defined JSON replacements
func (rw *JSONFileRW) replaceJSON(j map[string]any, rplce []models.MetaReplace) bool {
	logging.D(5, "Entering replaceJson with data: %v", j)

	if len(rplce) == 0 {
		logging.E("Called replaceJson without replacements")
		return false
	}

	edited := false
	for _, r := range rplce {
		if r.Field == "" || r.Value == "" {
			continue
		}

		if val, exists := j[r.Field]; exists {
			if strVal, ok := val.(string); ok {
				logging.D(3, "Identified field %q, replacing %q with %q", r.Field, r.Value, r.Replacement)
				j[r.Field] = strings.ReplaceAll(strVal, r.Value, r.Replacement)
				edited = true
			}
		}
	}
	logging.D(5, "After JSON replace: %v", j)
	return edited
}

// replaceJSONPrefix trims defined prefixes from specified fields
func (rw *JSONFileRW) replaceJSONPrefix(j map[string]any, tPfx []models.MetaReplacePrefix) bool {
	logging.D(5, "Entering trimJsonPrefix with data: %v", j)

	if len(tPfx) == 0 {
		logging.E("Called trimJsonPrefix without prefixes to trim")
		return false
	}

	edited := false
	for _, p := range tPfx {
		logging.I("PREFIX: %v, REPLACEMENT: %v", p.Prefix, p.Replacement)
		if p.Field == "" || p.Prefix == "" {
			continue
		}

		if val, exists := j[p.Field]; exists {

			if strVal, ok := val.(string); ok {
				logging.D(3, "Identified field %q, trimming %q", p.Field, p.Prefix)
				j[p.Field] = p.Replacement + strings.TrimPrefix(strVal, p.Prefix)
				edited = true
			}
		}
	}
	logging.D(5, "After prefix trim: %v", j)
	return edited
}

// replaceJSONSuffix trims defined suffixes from specified fields
func (rw *JSONFileRW) replaceJSONSuffix(j map[string]any, tSfx []models.MetaReplaceSuffix) bool {
	logging.D(5, "Entering trimJsonSuffix with data: %v", j)

	if len(tSfx) == 0 {
		logging.E("Called trimJsonSuffix without prefixes to trim")
		return false
	}

	edited := false
	for _, s := range tSfx {
		if s.Field == "" || s.Suffix == "" {
			continue
		}

		if val, exists := j[s.Field]; exists {

			if strVal, ok := val.(string); ok {
				logging.D(3, "Identified field %q, trimming %q", s.Field, s.Suffix)
				j[s.Field] = strings.TrimSuffix(strVal, s.Suffix) + s.Replacement
				edited = true
			}
		}
	}
	logging.D(5, "After suffix trim: %v", j)
	return edited
}

// jsonAppend appends to the fields in the JSON data
func (rw *JSONFileRW) jsonAppend(j map[string]any, file string, apnd []models.MetaAppend) bool {
	logging.D(5, "Entering jsonAppend with data: %v", j)

	if len(apnd) == 0 {
		logging.E("No new suffixes to append for file %q", file)
		return false // No replacements to apply
	}

	edited := false
	for _, a := range apnd {
		if a.Field == "" || a.Append == "" {
			continue
		}

		if value, exists := j[a.Field]; exists {

			if strVal, ok := value.(string); ok {

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

// jsonPrefix applies prefixes to the fields in the JSON data
func (rw *JSONFileRW) jsonPrefix(j map[string]any, file string, pfx []models.MetaPrefix) bool {
	logging.D(5, "Entering jsonPrefix with data: %v", j)

	if len(pfx) == 0 {
		logging.E("No new prefix replacements found for file %q", file)
		return false // No replacements to apply
	}

	edited := false
	for _, p := range pfx {
		if p.Field == "" || p.Prefix == "" {
			continue
		}

		if value, found := j[p.Field]; found {

			if strVal, ok := value.(string); ok {
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

// setJSONField can insert a new field which does not yet exist into the metadata file
func (rw *JSONFileRW) setJSONField(j map[string]any, file string, ow bool, newField []models.MetaSetField) (bool, error) {
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
				logging.I("Operation canceled for field: %s", n.Field)
				return false, errors.New("operation canceled")
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
func (rw *JSONFileRW) jsonFieldAddDateTag(j map[string]any, addDateTag map[string]models.MetaDateTag, fd *models.FileData) (bool, error) {
	if len(addDateTag) == 0 {
		logging.D(3, "No date tag operations to perform")
		return false, nil
	}
	if fd == nil {
		return false, fmt.Errorf("jsonFieldDateTag called with null FileData model")
	}
	logging.D(2, "Adding metadata date tags for %q...", fd.OriginalVideoBaseName)

	edited := false

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
		tag, err := metatags.MakeDateTag(j, fd, d.Format)
		if err != nil || tag == "" {
			return false, fmt.Errorf("failed to generate date tag for field %q: %w", fld, err)
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
func (rw *JSONFileRW) jsonFieldDeleteDateTag(j map[string]any, deleteDateTag map[string]models.MetaDeleteDateTag, fd *models.FileData) (bool, error) {
	if len(deleteDateTag) == 0 {
		logging.D(3, "No delete date tag operations to perform")
		return false, nil
	}
	if fd == nil {
		return false, fmt.Errorf("jsonFieldDateTag called with null FileData model")
	}
	logging.D(2, "Deleting metadata date tags for %q...", fd.OriginalVideoBaseName)

	edited := false

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

// copyToField copies values from one meta field to another
func (rw *JSONFileRW) copyToField(j map[string]any, copyTo []models.CopyToField) bool {
	logging.D(5, "Entering jsonPrefix with data: %v", j)

	if len(copyTo) == 0 {
		logging.E("No new copy operations found")
		return false
	}

	edited := false
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

// pasteFromField copies values from one meta field to another
func (rw *JSONFileRW) pasteFromField(j map[string]any, paste []models.PasteFromField) bool {
	logging.D(5, "Entering jsonPrefix with data: %v", j)

	if len(paste) == 0 {
		logging.E("No new paste operations found")
		return false
	}

	edited := false
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
