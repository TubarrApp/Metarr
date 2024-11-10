package metadata

import (
	"context"
	"fmt"
	"metarr/internal/config"
	enums "metarr/internal/domain/enums"
	keys "metarr/internal/domain/keys"
	tags "metarr/internal/metadata/tags"
	"metarr/internal/models"
	logging "metarr/internal/utils/logging"
	prompt "metarr/internal/utils/prompt"
	"strings"
)

// replaceJson makes user defined JSON replacements
func (rw *JSONFileRW) replaceJson(data map[string]interface{}, replace []*models.MetaReplace) (bool, error) {

	logging.D(5, "Entering replaceJson with data: %v", data)

	if len(replace) == 0 {
		logging.E(0, "Called replaceJson without replacements")
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
	logging.D(5, "After JSON replace: %v", data)
	return edited, nil
}

// trimJsonPrefix trims defined prefixes from specified fields
func (rw *JSONFileRW) trimJsonPrefix(data map[string]interface{}, trimPfx []*models.MetaTrimPrefix) (bool, error) {

	logging.D(5, "Entering trimJsonPrefix with data: %v", data)

	if len(trimPfx) == 0 {
		logging.E(0, "Called trimJsonPrefix without prefixes to trim")
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

// trimJsonSuffix trims defined suffixes from specified fields
func (rw *JSONFileRW) trimJsonSuffix(data map[string]interface{}, trimSfx []*models.MetaTrimSuffix) (bool, error) {

	logging.D(5, "Entering trimJsonSuffix with data: %v", data)

	if len(trimSfx) == 0 {
		logging.E(0, "Called trimJsonSuffix without prefixes to trim")
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

// jsonAppend appends to the fields in the JSON data
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
	logging.D(5, "After JSON suffix append: %v", data)

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

// setJsonField can insert a new field which does not yet exist into the metadata file
func (rw *JSONFileRW) setJsonField(data map[string]interface{}, modelOW bool, new []*models.MetaNewField) (bool, error) {

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
func (rw *JSONFileRW) jsonFieldDateTag(data map[string]interface{}, dateTagMap map[string]*models.MetaDateTag, fd *models.FileData, op enums.MetaDateTaggingType) (bool, error) {

	logging.D(2, "Making metadata date tag for '%s'...", fd.OriginalVideoBaseName)

	if len(dateTagMap) == 0 {
		logging.D(3, "No date tag operations to perform")
		return false, nil
	}
	if fd == nil {
		return false, fmt.Errorf("jsonFieldDateTag called with null FileData model")
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
		tag, err := tags.MakeDateTag(data, fd, dateTag.Format)
		if err != nil || tag == "" {
			return false, fmt.Errorf("failed to generate date tag for field '%s': %w", field, err)
		}

		if strings.Contains(tag, strVal) {
			logging.I("Tag '%s' already exists in field '%s'", tag, strVal)
			return false, nil
		}

		// Apply the tag based on location
		switch dateTag.Loc {
		case enums.DATE_TAG_LOC_PFX:

			switch op {
			case enums.DATE_TAG_DEL_OP:
				before := strVal
				data[field] = cleanFieldValue(strings.TrimPrefix(strVal, tag))
				if data[field] != before {
					logging.I("Deleted date tag '%s' prefix from field '%s'", tag, field)
					edited = true
				} else {
					logging.E(0, "Failed to strip date tag from '%s'", before)
				}

			case enums.DATE_TAG_ADD_OP:
				data[field] = tag + " " + strVal
				logging.I("Added date tag '%s' as prefix to field '%s'", tag, field)
				edited = true
			}

		case enums.DATE_TAG_LOC_SFX:

			switch op {
			case enums.DATE_TAG_DEL_OP:
				before := strVal
				data[field] = cleanFieldValue(strings.TrimSuffix(strVal, tag))
				if data[field] != before {
					logging.I("Deleted date tag '%s' suffix from field '%s'", tag, field)
					edited = true
				} else {
					logging.E(0, "Failed to strip date tag from '%s'", before)
				}

			case enums.DATE_TAG_ADD_OP:
				data[field] = strVal + " " + tag
				logging.I("Added date tag '%s' as suffix to field '%s'", tag, field)
				edited = true
			}

		default:
			return false, fmt.Errorf("invalid date tag location enum: %v", dateTag.Loc)
		}
	}

	return edited, nil
}
