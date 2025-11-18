package fieldsjson

import (
	"metarr/internal/domain/consts"
	"metarr/internal/domain/enums"
	"metarr/internal/domain/logger"
	"metarr/internal/metadata/metawriters"
	"metarr/internal/models"
	"metarr/internal/utils/browser"
	"metarr/internal/utils/printout"

	"github.com/TubarrApp/gocommon/logging"
)

// fillDescriptions grabs description data from JSON.
func fillDescriptions(fd *models.FileData, data map[string]any, jsonRW *metawriters.JSONFileRW) (map[string]any, bool) {
	d := fd.MTitleDesc
	w := fd.MWebData

	fieldMap := map[string]*string{ // Order by importance
		consts.JLongDesc:           &d.LongDescription,
		consts.JLongUnderscoreDesc: &d.LongUnderscoreDescription,
		consts.JDescription:        &d.Description,
		consts.JSynopsis:           &d.Synopsis,
		consts.JSummary:            &d.Summary,
		consts.JComment:            &d.Comment,
	}
	filled := unpackJSON(fieldMap, data)

	printMap := make(map[string]string, len(fieldMap))
	if logging.Level > 1 {
		defer func() {
			if len(printMap) > 0 {
				printout.PrintGrabbedFields("descriptions", printMap)
			}
		}()
	}

	// Attempt to fill empty description fields by inference
	for k, ptr := range fieldMap {
		if ptr == nil {
			logger.Pl.E("Unexpected nil pointer in descriptions fieldMap")
			continue
		}

		if *ptr == "" {
			if fillEmptyDescriptions(ptr, fd.MTitleDesc) {
				filled = true
			}
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
		rtn, err := jsonRW.WriteJSON(fieldMap)
		if err != nil {
			logger.Pl.E("Failed to write into JSON file %q: %v", fd.MetaFilePath, err)
		}

		if len(rtn) == 0 {
			logger.Pl.E("Length of return value is 0, returning original data from descriptions functions")
			return data, true
		}
		data = rtn
		return data, true
	}

	// Attempt to scrape description from the webpage
	if w.WebpageURL != "" {
		description := browser.ScrapeMeta(w, enums.WebclassDescription)
		if description != "" {
			for _, ptr := range fieldMap {
				if ptr == nil {
					logger.Pl.E("Unexpected nil pointer in descriptions fieldMap")
					continue
				}
				if *ptr == "" {
					*ptr = description
					filled = true
				}
			}
			rtn, err := jsonRW.WriteJSON(fieldMap)
			if err != nil {
				logger.Pl.E("Failed to insert new data (%s) into JSON file %q: %v", description, fd.MetaFilePath, err)
			} else if rtn != nil {
				data = rtn
				return data, filled
			}
			logger.Pl.D(1, "No descriptions were grabbed from scrape, returning original data map")
		}
	}
	return data, filled
}

// fillEmptyDescriptions fills empty description fields by inference.
func fillEmptyDescriptions(s *string, d *models.MetadataTitlesDescs) bool {
	if s == nil || d == nil {
		logger.Pl.E("%s entered description string nil or MetadataTitlesDescs nil", consts.LogTagDevError)
		return false
	}

	// Nil check and empty value check
	switch {
	case d.LongDescription != "":
		*s = d.LongDescription
		return true

	case d.LongUnderscoreDescription != "":
		*s = d.LongUnderscoreDescription
		return true

	case d.Description != "":
		*s = d.Description
		return true

	case d.Synopsis != "":
		*s = d.Synopsis
		return true

	case d.Summary != "":
		*s = d.Summary
		return true

	case d.Comment != "":
		*s = d.Comment
		return true
	}
	return false
}
