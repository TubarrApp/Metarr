package jsonrw

import (
	"context"
	"errors"
	"fmt"
	"metarr/internal/cfg"
	"metarr/internal/domain/enums"
	"metarr/internal/domain/keys"
	"metarr/internal/metadata/metatags"
	"metarr/internal/models"
	"metarr/internal/utils/logging"
	"metarr/internal/utils/prompt"
	"strings"
)

// replaceJSON makes user defined JSON replacements
func replaceJSON(j map[string]any, rplce []models.MetaReplace) (bool, error) {

	logging.D(5, "Entering replaceJson with data: %v", j)

	if len(rplce) == 0 {
		logging.E(0, "Called replaceJson without replacements")
		return false, nil
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
	return edited, nil
}

// trimJSONPrefix trims defined prefixes from specified fields
func trimJSONPrefix(j map[string]any, tPfx []models.MetaTrimPrefix) (bool, error) {

	logging.D(5, "Entering trimJsonPrefix with data: %v", j)

	if len(tPfx) == 0 {
		logging.E(0, "Called trimJsonPrefix without prefixes to trim")
		return false, nil
	}

	edited := false
	for _, p := range tPfx {
		if p.Field == "" || p.Prefix == "" {
			continue
		}

		if val, exists := j[p.Field]; exists {

			if strVal, ok := val.(string); ok {
				logging.D(3, "Identified field %q, trimming %q", p.Field, p.Prefix)
				j[p.Field] = strings.TrimPrefix(strVal, p.Prefix)
				edited = true
			}
		}
	}
	logging.D(5, "After prefix trim: %v", j)
	return edited, nil
}

// trimJSONSuffix trims defined suffixes from specified fields
func trimJSONSuffix(j map[string]any, tSfx []models.MetaTrimSuffix) (bool, error) {

	logging.D(5, "Entering trimJsonSuffix with data: %v", j)

	if len(tSfx) == 0 {
		logging.E(0, "Called trimJsonSuffix without prefixes to trim")
		return false, nil
	}

	edited := false
	for _, s := range tSfx {
		if s.Field == "" || s.Suffix == "" {
			continue
		}

		if val, exists := j[s.Field]; exists {

			if strVal, ok := val.(string); ok {
				logging.D(3, "Identified field %q, trimming %q", s.Field, s.Suffix)
				j[s.Field] = strings.TrimSuffix(strVal, s.Suffix)
				edited = true
			}
		}
	}
	logging.D(5, "After suffix trim: %v", j)
	return edited, nil
}

// jsonAppend appends to the fields in the JSON data
func jsonAppend(j map[string]any, file string, apnd []models.MetaAppend) (bool, error) {

	logging.D(5, "Entering jsonAppend with data: %v", j)

	if len(apnd) == 0 {
		logging.E(0, "No new suffixes to append for file %q", file)
		return false, nil // No replacements to apply
	}

	edited := false
	for _, a := range apnd {
		if a.Field == "" || a.Suffix == "" {
			continue
		}

		if value, exists := j[a.Field]; exists {

			if strVal, ok := value.(string); ok {

				logging.D(3, "Identified input JSON field '%v', appending '%v'", a.Field, a.Suffix)
				strVal += a.Suffix
				j[a.Field] = strVal
				edited = true
			}
		}
	}
	logging.D(5, "After JSON suffix append: %v", j)

	return edited, nil
}

// jsonPrefix applies prefixes to the fields in the JSON data
func jsonPrefix(j map[string]any, file string, pfx []models.MetaPrefix) (bool, error) {

	logging.D(5, "Entering jsonPrefix with data: %v", j)

	if len(pfx) == 0 {
		logging.E(0, "No new prefix replacements found for file %q", file)
		return false, nil // No replacements to apply
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

	return edited, nil
}

// setJSONField can insert a new field which does not yet exist into the metadata file
func setJSONField(j map[string]any, file string, ow bool, newField []models.MetaNewField) (bool, error) {
	if len(newField) == 0 {
		logging.E(0, "No new field additions found for file %q", file)
		return false, nil
	}

	var (
		metaOW,
		metaPS bool
	)

	if !cfg.IsSet(keys.MOverwrite) && !cfg.IsSet(keys.MPreserve) {
		logging.I("Model is set to overwrite")
		metaOW = ow
	} else {
		metaOW = cfg.GetBool(keys.MOverwrite)
		metaPS = cfg.GetBool(keys.MPreserve)
		logging.I("Meta OW: %v Meta Preserve: %v", metaOW, metaPS)
	}

	logging.D(3, "Retrieved additions for new field data: %v", newField)
	processedFields := make(map[string]bool, len(newField))

	newAddition := false
	ctx := context.Background()
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
			case <-ctx.Done():
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

					reply, err := prompt.PromptMetaReplace(promptMsg, metaOW, metaPS)
					if err != nil {
						logging.E(0, "Failed to retrieve reply from user prompt: %v", err)
					}

					switch reply {
					case "Y":
						logging.D(2, "Received meta overwrite reply as 'Y' for %s in %s, falling through to 'y'", existingValue, file)
						cfg.Set(keys.MOverwrite, true)
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
						cfg.Set(keys.MPreserve, true)
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
	logging.D(3, "JSON after transformations: %v", j)

	return newAddition, nil
}

// jsonFieldDateTag sets date tags in designated meta fields
func jsonFieldDateTag(j map[string]any, dtm map[string]models.MetaDateTag, fd *models.FileData, op enums.MetaDateTaggingType) (bool, error) {

	logging.D(2, "Making metadata date tag for %q...", fd.OriginalVideoBaseName)

	if len(dtm) == 0 {
		logging.D(3, "No date tag operations to perform")
		return false, nil
	}
	if fd == nil {
		return false, fmt.Errorf("jsonFieldDateTag called with null FileData model")
	}

	edited := false
	for fld, d := range dtm {
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

		if strings.Contains(strVal, tag) {
			logging.I("Tag %q already exists in field %q", tag, strVal)
			return false, nil
		}

		// Apply the tag based on location
		switch d.Loc {
		case enums.DatetagLocPrefix:

			switch op {
			case enums.DatetagDelOp:
				before := strVal
				result := strings.TrimPrefix(strVal, tag)
				result = cleanFieldValue(result)

				j[fld] = result

				if j[fld] != before {
					logging.I("Deleted date tag %q prefix from field %q", tag, fld)
					edited = true
				} else {
					logging.E(0, "Failed to strip date tag from %q", before)
				}

			case enums.DatetagAddOp:

				j[fld] = fmt.Sprintf("%s %s", tag, strVal)
				logging.I("Added date tag %q as prefix to field %q", tag, fld)
				edited = true
			}

		case enums.DatetagLocSuffix:

			switch op {
			case enums.DatetagDelOp:

				before := strVal
				result := strings.TrimPrefix(strVal, tag)
				result = cleanFieldValue(result)
				j[fld] = result

				if j[fld] != before {
					logging.I("Deleted date tag %q suffix from field %q", tag, fld)
					edited = true
				} else {
					logging.E(0, "Failed to strip date tag from %q", before)
				}

			case enums.DatetagAddOp:

				j[fld] = fmt.Sprintf("%s %s", strVal, tag)
				logging.I("Added date tag %q as suffix to field %q", tag, fld)
				edited = true
			}

		default:
			return false, fmt.Errorf("invalid date tag location enum: %v", d.Loc)
		}
	}
	return edited, nil
}

// copyToField copies values from one meta field to another
func copyToField(j map[string]any, copyTo []models.CopyToField) (bool, error) {

	logging.D(5, "Entering jsonPrefix with data: %v", j)

	if len(copyTo) == 0 {
		logging.E(0, "No new copy operations found")
		return false, nil
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

	return edited, nil
}

// pasteFromField copies values from one meta field to another
func pasteFromField(j map[string]any, paste []models.PasteFromField) (bool, error) {

	logging.D(5, "Entering jsonPrefix with data: %v", j)

	if len(paste) == 0 {
		logging.E(0, "No new paste operations found")
		return false, nil
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

	return edited, nil
}
