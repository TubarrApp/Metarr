package metadata

import (
	"metarr/internal/models"
	logging "metarr/internal/utils/logging"
)

// Primary function to fill out meta fields before writing
func FillMetaFields(fd *models.FileData, data map[string]interface{}) (map[string]interface{}, bool) {

	allFilled := true

	if meta, ok := fillTitles(fd, data); !ok {
		logging.I("No title metadata found")
		allFilled = false
	} else if meta != nil {
		data = meta
	}

	if meta, ok := fillCredits(fd, data); !ok {
		logging.I("No credits metadata found")
		allFilled = false
	} else if meta != nil {
		data = meta
	}

	if meta, ok := fillDescriptions(fd, data); !ok {
		logging.I("No description metadata found")
		allFilled = false
	} else if meta != nil {
		data = meta
	}
	return data, allFilled
}

// unpackJSON decodes JSON for metafields
func unpackJSON(fmap map[string]*string, json map[string]interface{}) bool {

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
			pmap[k] = val
			filled = true
		}
	}

	return filled
}
