package metadata

import (
	"metarr/internal/models"
	logging "metarr/internal/utils/logging"
	print "metarr/internal/utils/print"
)

// Primary function to fill out meta fields before writing
func FillMetaFields(fd *models.FileData, data map[string]interface{}) (map[string]interface{}, bool) {

	var (
		ok   bool
		meta map[string]interface{}
	)
	allFilled := true

	if len(fd.MWebData.TryURLs) == 0 {
		if !FillWebpageDetails(fd, data) {
			logging.I("No URL metadata found")
			allFilled = false
		}
	}

	if !fillTitles(fd, data) {
		logging.I("No title metadata found")
		allFilled = false
	}

	if meta, ok = fillCredits(fd, data); !ok {
		logging.I("No credits metadata found")
		allFilled = false
	} else if meta != nil {
		data = meta
	}

	if meta, ok = fillDescriptions(fd, data); !ok {
		logging.I("No description metadata found")
		allFilled = false
	} else if meta != nil {
		data = meta
	}
	return data, allFilled
}

// unpackJSON decodes JSON for metafields
func unpackJSON(fieldType string, fieldMap map[string]*string, metadata map[string]interface{}) bool {

	dataFilled := false
	printMap := make(map[string]string, len(fieldMap))

	// Iterate through the decoded JSON to match fields against
	// the passed in map of fields to fill
	for key, value := range metadata {
		if strVal, ok := value.(string); ok {
			if field, exists := fieldMap[key]; exists && field != nil && *field == "" {

				*field = strVal
				dataFilled = true

				if printMap[key] == "" {
					printMap[key] = strVal
				}
			}
		}
	}
	print.PrintGrabbedFields(fieldType, &printMap)

	return dataFilled
}
