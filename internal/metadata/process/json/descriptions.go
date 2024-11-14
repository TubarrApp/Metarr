package metadata

import (
	"fmt"
	"metarr/internal/cfg"
	consts "metarr/internal/domain/constants"
	enums "metarr/internal/domain/enums"
	keys "metarr/internal/domain/keys"
	"metarr/internal/models"
	browser "metarr/internal/utils/browser"
	logging "metarr/internal/utils/logging"
	print "metarr/internal/utils/print"
	"strings"
)

// fillDescriptions grabs description data from JSON
func fillDescriptions(fd *models.FileData, data map[string]interface{}) (map[string]interface{}, bool) {

	d := fd.MTitleDesc
	w := fd.MWebData
	t := fd.MDates

	fieldMap := map[string]*string{ // Order by importance
		consts.JLongDescription:  &d.LongDescription,
		consts.JLong_Description: &d.Long_Description,
		consts.JDescription:      &d.Description,
		consts.JSynopsis:         &d.Synopsis,
		consts.JSummary:          &d.Summary,
		consts.JComment:          &d.Comment,
	}
	filled := unpackJSON(fieldMap, data)

	datePfx := cfg.GetBool(keys.MDescDatePfx)
	dateSfx := cfg.GetBool(keys.MDescDateSfx)

	if (datePfx || dateSfx) && t.StringDate != "" {
		for _, ptr := range fieldMap {
			if ptr == nil {
				logging.E(0, "Unexpected nil pointer in descriptions fieldMap")
				continue
			}

			if !datePfx && !dateSfx {
				logging.D(1, "Unknown issue appending date to description. Condition should be impossible? (reached: %s)", *ptr)
				continue
			}

			if datePfx && !strings.HasPrefix(*ptr, t.StringDate) {
				*ptr = fmt.Sprintf("%s\n\n%s", t.StringDate, *ptr) // Prefix string date
			}

			if dateSfx && !strings.HasSuffix(*ptr, t.StringDate) {
				*ptr = fmt.Sprintf("%s\n\n%s", *ptr, t.StringDate) // Suffix string date
			}
		}
	}

	printMap := make(map[string]string, len(fieldMap))
	defer func() {
		if len(printMap) > 0 && logging.Level > 1 {
			print.PrintGrabbedFields("descriptions", printMap)
		}
	}()

	// Attempt to fill empty description fields by inference
	for k, ptr := range fieldMap {
		if ptr == nil {
			logging.E(0, "Unexpected nil pointer in descriptions fieldMap")
			continue
		}

		if *ptr == "" {
			if ok := fillEmptyDescriptions(ptr, d); ok {
				filled = true
				if logging.Level > 1 {
					printMap[k] = *ptr
				}
			}
		} else {
			filled = true
			if logging.Level > 1 {
				printMap[k] = *ptr
			}
		}
	}

	if filled {

		rtn, err := fd.JSONFileRW.WriteJSON(fieldMap)
		if err != nil {
			logging.E(0, "Failed to write into JSON file '%s': %v", fd.JSONFilePath, err)
		}

		if len(rtn) == 0 {
			logging.E(0, "Length of return value is 0, returning original data from descriptions functions")
			return data, true
		}

		data = rtn
		return data, true
	}

	if w.WebpageURL == "" {
		logging.I("Page URL not found in data, so cannot scrape for missing description in '%s'", fd.JSONFilePath)
		return data, false
	}

	description := browser.ScrapeMeta(w, enums.WEBCLASS_DESCRIPTION)

	// Infer remaining fields from description
	if description != "" {

		for _, ptr := range fieldMap {
			if ptr == nil {
				logging.E(0, "Unexpected nil in descriptions fieldMap")
				continue
			}

			if *ptr == "" {
				*ptr = description
			}
		}

		// Insert new scraped fields into file
		rtn, err := fd.JSONFileRW.WriteJSON(fieldMap)
		if err != nil {
			logging.E(0, "Failed to insert new data (%s) into JSON file '%s': %v", description, fd.JSONFilePath, err)
		} else if rtn != nil {

			data = rtn
			return data, true
		}

		logging.D(1, "No descriptions were grabbed from scrape, returning original data map")
		return data, false
	} else {
		return data, false
	}
}

// fillEmptyDescriptions fills empty description fields by inference
func fillEmptyDescriptions(s *string, d *models.MetadataTitlesDescs) bool {

	// Nil check and empty value check should be done in caller
	filled := false
	switch {
	case d.LongDescription != "":
		*s = d.LongDescription
		filled = true

	case d.Long_Description != "":
		*s = d.Long_Description
		filled = true

	case d.Description != "":
		*s = d.Description
		filled = true

	case d.Synopsis != "":
		*s = d.Synopsis
		filled = true

	case d.Summary != "":
		*s = d.Summary
		filled = true

	case d.Comment != "":
		*s = d.Comment
		filled = true
	}

	return filled
}
