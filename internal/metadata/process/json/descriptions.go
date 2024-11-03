package metadata

import (
	"Metarr/internal/config"
	consts "Metarr/internal/domain/constants"
	enums "Metarr/internal/domain/enums"
	keys "Metarr/internal/domain/keys"
	helpers "Metarr/internal/metadata/process/helpers"
	"Metarr/internal/types"
	logging "Metarr/internal/utils/logging"
	"strings"
)

// fillDescriptions grabs description data from JSON
func fillDescriptions(fd *types.FileData, data map[string]interface{}) (map[string]interface{}, bool) {
	var (
		d = fd.MTitleDesc
		w = fd.MWebData
		t = fd.MDates
	)

	fieldMap := map[string]*string{ // Order by importance
		consts.JLongDescription:  &d.LongDescription,
		consts.JLong_Description: &d.Long_Description,
		consts.JDescription:      &d.Description,
		consts.JSynopsis:         &d.Synopsis,
		consts.JSummary:          &d.Summary,
		consts.JComment:          &d.Comment,
	}

	filled := unpackJSON("descriptions", fieldMap, data)

	datePfx := config.GetBool(keys.MDescDatePfx)
	dateSfx := config.GetBool(keys.MDescDateSfx)

	if (datePfx || dateSfx) && t.StringDate != "" {

		for _, value := range fieldMap {
			if value != nil {
				switch {
				case datePfx:
					if !strings.HasPrefix(*value, t.StringDate) {
						*value = t.StringDate + "\n\n" + *value // Prefix string date
					}
					continue
				case dateSfx:
					if !strings.HasSuffix(*value, t.StringDate) {
						*value = *value + "\n\n" + t.StringDate // Suffix string date
					}
					continue
				default:
					logging.PrintD(1, "Unknown issue appending date to description. Condition should be impossible? (reached: %s)", *value)
					continue
				}
			}
		}
	}

	// Attempt to fill empty description fields by inference
	for _, value := range fieldMap {
		if ok := fillEmptyDescriptions(value, d); ok {
			filled = true
		}
	}

	// Check if any values are present
	if !filled {
		for _, val := range fieldMap {
			if val != nil {
				if *val == "" {
					continue
				} else {
					filled = true
				}
			}
		}
	}

	switch {
	case filled:
		rtn, err := fd.JSONFileRW.WriteMetadata(fieldMap)
		switch {
		case err != nil:
			logging.PrintE(0, "Failed to write into JSON file '%s': %v", fd.JSONFilePath, err)
			return data, true
		case rtn != nil:
			data = rtn
			return data, true
		}

	case w.WebpageURL == "":
		logging.PrintI("Page URL not found in data, so cannot scrape for missing description in '%s'", fd.JSONFilePath)
		return data, false
	}

	description := helpers.ScrapeMeta(w, enums.WEBCLASS_DESCRIPTION)

	// Infer remaining fields from description
	if description != "" {
		for _, value := range fieldMap {
			if *value == "" {
				*value = description
			}
		}

		// Insert new scraped fields into file
		rtn, err := fd.JSONFileRW.WriteMetadata(fieldMap)
		if err != nil {
			logging.PrintE(0, "Failed to insert new data (%s) into JSON file '%s': %v", description, fd.JSONFilePath, err)
		} else if rtn != nil {
			data = rtn
		}
		return data, true
	} else {
		return data, false
	}
}

// fillEmptyDescriptions fills empty description fields by inference
func fillEmptyDescriptions(want *string, d *types.MetadataTitlesDescs) bool {

	filled := false
	if want == nil {
		logging.PrintE(0, "Sent in string null, returning...")
		return false
	}
	if *want == "" {
		switch {
		case d.LongDescription != "":
			*want = d.LongDescription
			filled = true

		case d.Long_Description != "":
			*want = d.Long_Description
			filled = true

		case d.Description != "":
			*want = d.Description
			filled = true

		case d.Synopsis != "":
			*want = d.Synopsis
			filled = true

		case d.Summary != "":
			*want = d.Summary
			filled = true

		case d.Comment != "":
			*want = d.Comment
			filled = true
		}
	}
	return filled
}
