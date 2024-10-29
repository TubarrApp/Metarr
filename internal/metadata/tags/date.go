package metadata

import (
	"Metarr/internal/config"
	consts "Metarr/internal/domain/constants"
	enums "Metarr/internal/domain/enums"
	keys "Metarr/internal/domain/keys"
	logging "Metarr/internal/utils/logging"
	"fmt"
	"path/filepath"
	"strings"
)

// makeDateTag creates the date tag to affix to the filename
func MakeDateTag(metadata map[string]interface{}, fileName string) (string, error) {

	preferredDateFields := []string{
		consts.JReleaseDate,
		"releasedate", // Alias
		"released_on", // Alias
		consts.JOriginallyAvailable,
		"originally_available", // Alias
		"originallyavailable",  // Alias
		consts.JDate,
		consts.JUploadDate,
		"uploaddate",  // Alias
		"uploaded_on", // Alias
		consts.JCreationTime,
		"created_at", // Alias
	}

	dateFmt, ok := config.Get(keys.FileDateFmt).(enums.FilenameDateFormat)
	if !ok {
		return "", fmt.Errorf("grabbed value from Viper in makeDateTag but var type was not correct")
	}

	var gotDate bool = false
	var date string = ""

	for _, field := range preferredDateFields {
		logging.PrintD(3, "Checking for date in field: %s", field)
		if value, found := metadata[field]; found {
			logging.PrintD(3, "Found date in field '%s' with value: %v (type: %T)", field, value, value)

			if strVal, ok := value.(string); ok && strVal != "" && len(strVal) > 4 {
				bef, _, _ := strings.Cut(strVal, "T")
				date = bef
				gotDate = true
				logging.PrintD(3, "Extracted date string: %s", date)
				break
			}
		}
	}

	if !gotDate {
		logging.PrintE(0, "No dates found in JSON file")
		return "", nil
	}

	// Logging dates before and after replacing dashes for parsing
	logging.PrintD(3, "Date before replacing dashes: %s", date)
	date = strings.ReplaceAll(date, "-", "")
	logging.PrintD(3, "Date after replacing dashes: %s (length: %d)", date, len(date))

	logging.PrintD(1, "Entering case check for %v", fileName)

	if len(date) >= 8 {
		logging.PrintD(3, "Input date length >= 8: %d", date)
		switch dateFmt {
		case enums.FILEDATE_YYYY_MM_DD:
			logging.PrintD(3, "Formatting as: yyyy-mm-dd")
			date = date[:4] + "-" + date[4:6] + "-" + date[6:8]
			logging.PrintD(3, "Date as %s", date)

		case enums.FILEDATE_YY_MM_DD:
			logging.PrintD(3, "Formatting as: yy-mm-dd")
			date = date[2:4] + "-" + date[4:6] + "-" + date[6:8]
			logging.PrintD(3, "Date as %s", date)

		case enums.FILEDATE_YYYY_DD_MM:
			logging.PrintD(3, "Formatting as: yyyy-dd-mm")
			date = date[:4] + "-" + date[6:8] + "-" + date[4:6]
			logging.PrintD(3, "Date as %s", date)

		case enums.FILEDATE_YY_DD_MM:
			logging.PrintD(3, "Formatting as: yy-dd-mm")
			date = date[2:4] + "-" + date[6:8] + "-" + date[4:6]
			logging.PrintD(3, "Date as %s", date)

		case enums.FILEDATE_DD_MM_YYYY:
			logging.PrintD(3, "Formatting as: dd-mm-yyyy")
			date = date[6:8] + "-" + date[4:6] + "-" + date[:4]
			logging.PrintD(3, "Date as %s", date)

		case enums.FILEDATE_DD_MM_YY:
			logging.PrintD(3, "Formatting as: dd-mm-yy")
			date = date[6:8] + "-" + date[4:6] + "-" + date[2:4]
			logging.PrintD(3, "Date as %s", date)

		case enums.FILEDATE_MM_DD_YYYY:
			logging.PrintD(3, "Formatting as: mm-dd-yyyy")
			date = date[4:6] + "-" + date[6:8] + "-" + date[:4]
			logging.PrintD(3, "Date as %s", date)

		case enums.FILEDATE_MM_DD_YY:
			logging.PrintD(3, "Formatting as: mm-dd-yy")
			date = date[4:6] + "-" + date[6:8] + "-" + date[2:4]
			logging.PrintD(3, "Date as %s", date)
		}
	} else if len(date) >= 6 {
		logging.PrintD(3, "Input date length >= 6 and < 8: %d", date)
		switch dateFmt {
		case enums.FILEDATE_YYYY_MM_DD, enums.FILEDATE_YY_MM_DD:
			date = date[:2] + "-" + date[2:4] + "-" + date[4:6]

		case enums.FILEDATE_YYYY_DD_MM, enums.FILEDATE_YY_DD_MM:
			date = date[:2] + "-" + date[4:6] + "-" + date[2:4]

		case enums.FILEDATE_DD_MM_YYYY, enums.FILEDATE_DD_MM_YY:
			date = date[4:6] + "-" + date[2:4] + "-" + date[:2]

		case enums.FILEDATE_MM_DD_YYYY, enums.FILEDATE_MM_DD_YY:
			date = date[2:4] + "-" + date[4:6] + "-" + date[:2]
		}
	} else if len(date) >= 4 {
		logging.PrintD(3, "Input date length >= 4 and < 6: %d", date)
		switch dateFmt {
		case enums.FILEDATE_YYYY_MM_DD,
			enums.FILEDATE_YY_MM_DD,
			enums.FILEDATE_MM_DD_YYYY,
			enums.FILEDATE_MM_DD_YY:

			date = date[:2] + "-" + date[2:4]

		case enums.FILEDATE_YYYY_DD_MM,
			enums.FILEDATE_YY_DD_MM,
			enums.FILEDATE_DD_MM_YYYY,
			enums.FILEDATE_DD_MM_YY:

			date = date[2:4] + "-" + date[:2]
		}
	}

	dateTag := "[" + date + "]"

	logging.PrintD(1, "Made date tag '%s' from file '%v'", dateTag, filepath.Base(fileName))

	if dateTag != "[]" {
		if checkTagExists(dateTag, filepath.Base(fileName)) {
			logging.PrintD(2, "Tag '%s' already detected in name, skipping...", dateTag)
			dateTag = "[]"
		}
	} else {
		logging.PrintD(3, "Constructed empty tag, skipping tag exists check for file '%s'", filepath.Base(fileName))
	}

	return dateTag, nil
}
