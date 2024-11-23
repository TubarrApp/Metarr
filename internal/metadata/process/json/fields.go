package metadata

import (
	"metarr/internal/models"
	logging "metarr/internal/utils/logging"
)

// Primary function to fill out meta fields before writing
func FillJSONFields(fd *models.FileData, json map[string]any) (map[string]any, bool) {

	allFilled := true
	if meta, ok := fillTitles(fd, json); !ok {
		logging.I("No title metadata found")
		allFilled = false
	} else if meta != nil {
		json = meta
	}

	if meta, ok := fillCredits(fd, json); !ok {
		logging.I("No credits metadata found")
		allFilled = false
	} else if meta != nil {
		json = meta
	}

	if meta, ok := fillDescriptions(fd, json); !ok {
		logging.I("No description metadata found")
		allFilled = false
	} else if meta != nil {
		json = meta
	}
	return json, allFilled
}

// unpackJSON decodes JSON for metafields
func unpackJSON(fmap map[string]*string, json map[string]any) bool {

	filled := false
	pmap := make(map[string]string, len(fmap))

	// Match decoded JSON to field map
	for k, ptr := range fmap {
		if ptr == nil {
			logging.E(0, "fieldMap entry pointer unexpectedly nil")
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
