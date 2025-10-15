package jsonfields

import (
	"fmt"
	"metarr/internal/cfg"
	"metarr/internal/domain/consts"
	"metarr/internal/domain/enums"
	"metarr/internal/domain/keys"
	"metarr/internal/models"
	"metarr/internal/utils/browser"
	"metarr/internal/utils/logging"
	"metarr/internal/utils/printout"
	"strings"
)

// fillDescriptions grabs description data from JSON
func fillDescriptions(fd *models.FileData, data map[string]any) (map[string]any, bool) {

	d := fd.MTitleDesc
	w := fd.MWebData
	t := fd.MDates

	fieldMap := map[string]*string{ // Order by importance
		consts.JLongDesc:           &d.LongDescription,
		consts.JLongUnderscoreDesc: &d.LongUnderscoreDescription,
		consts.JDescription:        &d.Description,
		consts.JSynopsis:           &d.Synopsis,
		consts.JSummary:            &d.Summary,
		consts.JComment:            &d.Comment,
	}
	filled := unpackJSON(fieldMap, data)

	datePfx := cfg.GetBool(keys.MDescDatePfx)
	dateSfx := cfg.GetBool(keys.MDescDateSfx)

	if (datePfx || dateSfx) && t.StringDate != "" {
		for _, ptr := range fieldMap {
			if ptr == nil {
				logging.E("Unexpected nil pointer in descriptions fieldMap")
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

	var printMap map[string]string
	if logging.Level > 1 {
		printMap = make(map[string]string, len(fieldMap))
		defer func() {
			if len(printMap) > 0 {
				printout.PrintGrabbedFields("descriptions", printMap)
			}
		}()
	}

	// Fill with by inference
	var fillWith string
	for _, ptr := range fieldMap {
		if ptr == nil {
			continue
		}
		if *ptr != "" {
			fillWith = *ptr
			break
		}
	}

	// Attempt to fill empty description fields by inference
	for k, ptr := range fieldMap {
		if ptr == nil {
			logging.E("Unexpected nil pointer in descriptions fieldMap")
			continue
		}

		if *ptr == "" {
			*ptr = fillWith
			filled = true
			if logging.Level > 1 {
				printMap[k] = *ptr
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
			logging.E("Failed to write into JSON file %q: %v", fd.JSONFilePath, err)
		}

		if len(rtn) == 0 {
			logging.E("Length of return value is 0, returning original data from descriptions functions")
			return data, true
		}

		data = rtn
		return data, true
	}

	if w.WebpageURL == "" {
		logging.I("Page URL not found in data, so cannot scrape for missing description in %q", fd.JSONFilePath)
		return data, false
	}

	description := browser.ScrapeMeta(w, enums.WebclassDescription)

	// Infer remaining fields from description
	if description != "" {

		for _, ptr := range fieldMap {
			if ptr == nil {
				logging.E("Unexpected nil in descriptions fieldMap")
				continue
			}

			if *ptr == "" {
				*ptr = description
			}
		}

		// Insert new scraped fields into file
		rtn, err := fd.JSONFileRW.WriteJSON(fieldMap)
		if err != nil {
			logging.E("Failed to insert new data (%s) into JSON file %q: %v", description, fd.JSONFilePath, err)
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

// // fillEmptyDescriptions fills empty description fields by inference
// func fillEmptyDescriptions(s *string, d *models.MetadataTitlesDescs) bool {

// 	// Nil check and empty value check should be done in caller
// 	filled := false
// 	switch {
// 	case d.LongDescription != "":
// 		*s = d.LongDescription
// 		filled = true

// 	case d.Long_Description != "":
// 		*s = d.Long_Description
// 		filled = true

// 	case d.Description != "":
// 		*s = d.Description
// 		filled = true

// 	case d.Synopsis != "":
// 		*s = d.Synopsis
// 		filled = true

// 	case d.Summary != "":
// 		*s = d.Summary
// 		filled = true

// 	case d.Comment != "":
// 		*s = d.Comment
// 		filled = true
// 	}

// 	return filled
// }
