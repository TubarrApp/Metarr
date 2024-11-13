package metadata

import (
	"metarr/internal/models"
	logging "metarr/internal/utils/logging"
	print "metarr/internal/utils/print"
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
func unpackJSON(ftype string, fmap map[string]*string, json map[string]interface{}) bool {

	filled := false
	pmap := make(map[string]string, len(fmap))

	// Match decoded JSON to field map
	for k, v := range json {
		if val, ok := v.(string); ok {
			logging.D(3, "Checking field '%s' with value '%s'", k, val)
			if field, exists := fmap[k]; exists && field != nil && *field == "" {

				*field = val
				filled = true

				if pmap[k] == "" {
					pmap[k] = val
				}
			}
		}
	}
	if logging.Level > -1 {
		print.PrintGrabbedFields(ftype, &pmap)
	}

	return filled
}
