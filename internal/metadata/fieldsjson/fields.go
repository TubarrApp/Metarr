// Package fieldsjson is used for handling JSON metafields.
package fieldsjson

import (
	"metarr/internal/domain/logger"
	"metarr/internal/metadata/metawriters"
	"metarr/internal/models"

	"github.com/TubarrApp/gocommon/logging"
)

// FillJSONFields is the fills metafields before writing the data.
func FillJSONFields(fd *models.FileData, json map[string]any, jsonRW *metawriters.JSONFileRW) (map[string]any, bool) {
	allFilled := true
	if meta, ok := fillTitles(fd, json, jsonRW); !ok {
		logger.Pl.I("No title metadata found")
		allFilled = false
	} else if meta != nil {
		json = meta
	}

	if meta, ok := fillCredits(fd, json, jsonRW); !ok {
		logger.Pl.I("No credits metadata found")
		allFilled = false
	} else if meta != nil {
		json = meta
	}

	if meta, ok := fillDescriptions(fd, json, jsonRW); !ok {
		logger.Pl.I("No description metadata found")
		allFilled = false
	} else if meta != nil {
		json = meta
	}
	return json, allFilled
}

// unpackJSON decodes JSON for metafields.
func unpackJSON(fmap map[string]*string, json map[string]any) bool {
	filled := false
	pmap := make(map[string]string, len(fmap))

	// Match decoded JSON to field map
	for k, ptr := range fmap {
		if ptr == nil {
			logger.Pl.E("fieldMap entry pointer unexpectedly nil")
			continue
		}

		v, exists := json[k]
		if !exists {
			continue
		}

		val, ok := v.(string)
		if !ok {
			continue
		}

		if *ptr == "" {
			*ptr = val
			if logging.Level > 1 {
				pmap[k] = val
			}
			filled = true
		}
	}

	return filled
}
