package fieldsjson

import (
	"metarr/internal/domain/consts"
	"metarr/internal/domain/logger"
	"metarr/internal/metadata/metawriters"
	"metarr/internal/models"
	"metarr/internal/utils/printout"

	"github.com/TubarrApp/gocommon/logging"
	"github.com/TubarrApp/gocommon/sharedtags"
)

// fillDescriptions grabs description data from JSON.
func fillDescriptions(fd *models.FileData, data map[string]any, jsonRW *metawriters.JSONFileRW) (map[string]any, bool) {
	d := fd.MTitleDesc

	fieldMap := map[string]*string{ // Order by importance.
		sharedtags.JLongDesc:           &d.LongDescription,
		sharedtags.JLongUnderscoreDesc: &d.LongUnderscoreDescription,
		sharedtags.JDescription:        &d.Description,
		sharedtags.JSynopsis:           &d.Synopsis,
		sharedtags.JSummary:            &d.Summary,
		sharedtags.JComment:            &d.Comment,
	}
	filled := unpackJSON(fieldMap, data)

	printMap := make(map[string]string, len(fieldMap))
	if logging.Level() > 1 {
		defer func() {
			if len(printMap) > 0 {
				printout.PrintGrabbedFields("descriptions", printMap)
			}
		}()
	}

	// Attempt to fill empty description fields by inference.
	for k, ptr := range fieldMap {
		if ptr == nil {
			logger.Pl.E("Unexpected nil pointer in descriptions fieldMap")
			continue
		}

		if *ptr == "" {
			if fillEmptyDescriptions(ptr, fd.MTitleDesc) {
				filled = true
			}
			if logging.Level() > 1 {
				printMap[k] = *ptr
			}
		} else {
			filled = true
			if logging.Level() > 1 {
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
	return data, filled
}

// fillEmptyDescriptions fills empty description fields by inference.
func fillEmptyDescriptions(s *string, d *models.MetadataTitlesDescs) bool {
	if s == nil || d == nil {
		logger.Pl.E("%s entered description string nil or MetadataTitlesDescs nil", consts.LogTagDevError)
		return false
	}

	// Nil check and empty value check.
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
