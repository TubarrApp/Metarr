// Package metatags handles tags for metadata such as creation of the [yy-mm-dd] date tag.
package metatags

import (
	"fmt"
	"metarr/internal/dates"
	"metarr/internal/domain/consts"
	"metarr/internal/domain/enums"
	"metarr/internal/models"
	"metarr/internal/utils/logging"
	"strings"
)

// MakeDateTag attempts to create the date tag for files using metafile data.
func MakeDateTag(metadata map[string]any, fd *models.FileData, dateFmt enums.DateFormat) (string, error) {

	if dateFmt == enums.DateFmtSkip {
		logging.D(1, "Skip set, not making file date tag for %q", fd.OriginalVideoBaseName)
		return "", nil
	}

	var (
		date  string
		found bool
	)

	if fd.MDates.FormattedDate == "" {
		if date, found = extractDateFromMetadata(metadata); !found {
			logging.E("No dates found in JSON file")
			return "", nil
		}
	} else {
		date = fd.MDates.FormattedDate
	}

	year, month, day, err := dates.ParseDateComponents(date, dateFmt)
	if err != nil {
		return "", fmt.Errorf("failed to parse date components: %w", err)
	}

	dateStr, err := dates.FormatDateString(year, month, day, dateFmt)
	if dateStr == "" || err != nil {
		logging.E("Failed to create date string")
		return "", nil
	}

	dateTag := fmt.Sprintf("[%s]", dateStr)
	logging.S("Made date tag %q from file '%v'", dateTag, fd.FinalVideoPath)
	return dateTag, nil
}

// extractDateFromMetadata attempts to find a date in the metadata using predefined fields
func extractDateFromMetadata(metadata map[string]any) (string, bool) {
	preferredDateFields := []string{
		consts.JReleaseDate,
		"releasedate",
		"released_on",
		consts.JOriginallyAvailable,
		"originally_available",
		"originallyavailable",
		consts.JDate,
		consts.JUploadDate,
		"uploaddate",
		"uploaded_on",
		consts.JCreationTime, // Last resort, may give false positives
		"created_at",
	}

	for _, field := range preferredDateFields {
		if value, found := metadata[field]; found {
			if strVal, ok := value.(string); ok && strVal != "" && len(strVal) > 4 {
				if date, _, found := strings.Cut(strVal, "T"); found {
					return date, true
				}
				return strVal, true
			}
		}
	}
	return "", false
}
