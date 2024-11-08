package metadata

import (
	"fmt"
	"metarr/internal/config"
	consts "metarr/internal/domain/constants"
	enums "metarr/internal/domain/enums"
	keys "metarr/internal/domain/keys"
	logging "metarr/internal/utils/logging"
	"path/filepath"
	"strconv"
	"strings"
)

// MakeDateTag attempts to create the date tag for files using metafile data
func MakeDateTag(metadata map[string]interface{}, fileName string) (string, error) {
	dateFmt, ok := config.Get(keys.FileDateFmt).(enums.FilenameDateFormat)
	if !ok {
		return "", fmt.Errorf("invalid date format configuration")
	}

	date, found := extractDateFromMetadata(metadata)
	if !found {
		logging.E(0, "No dates found in JSON file")
		return "", nil
	}

	year, month, day, err := parseDateComponents(date, dateFmt)
	if err != nil {
		return "", fmt.Errorf("failed to parse date components: %w", err)
	}

	dateStr, err := formatDateString(year, month, day, dateFmt)
	if dateStr == "" || err != nil {
		logging.E(0, "Failed to create date string")
		return "[]", nil
	}

	dateTag := "[" + dateStr + "]"
	if checkTagExists(dateTag, filepath.Base(fileName)) {
		logging.D(2, "Tag '%s' already detected in name, skipping...", dateTag)
		return "[]", nil
	}

	logging.S(0, "Made date tag '%s' from file '%v'", dateTag, filepath.Base(fileName))
	return dateTag, nil
}

// extractDateFromMetadata attempts to find a date in the metadata using predefined fields
func extractDateFromMetadata(metadata map[string]interface{}) (string, bool) {
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

// parseDateComponents extracts and validates year, month, and day from the date string
func parseDateComponents(date string, dateFmt enums.FilenameDateFormat) (year, month, day string, err error) {
	date = strings.ReplaceAll(date, "-", "")
	date = strings.TrimSpace(date)

	year, month, day, err = getYearMonthDay(date, dateFmt)
	if err != nil {
		return "", "", "", err
	}

	return validateDateComponents(year, month, day)
}

// formatDateString formats the date as a hyphenated string
func formatDateString(year, month, day string, dateFmt enums.FilenameDateFormat) (string, error) {
	var parts [3]string

	switch dateFmt {
	case enums.FILEDATE_YYYY_MM_DD, enums.FILEDATE_YY_MM_DD:
		parts = [3]string{year, month, day}
	case enums.FILEDATE_YYYY_DD_MM, enums.FILEDATE_YY_DD_MM:
		parts = [3]string{year, day, month}
	case enums.FILEDATE_DD_MM_YYYY, enums.FILEDATE_DD_MM_YY:
		parts = [3]string{day, month, year}
	case enums.FILEDATE_MM_DD_YYYY, enums.FILEDATE_MM_DD_YY:
		parts = [3]string{month, day, year}
	}

	result := joinNonEmpty(parts)
	if result == "" {
		return "", fmt.Errorf("no valid date components found")
	}
	return result, nil
}

// joinNonEmpty joins non-empty strings from an array with hyphens
func joinNonEmpty(parts [3]string) string {
	nonEmpty := make([]string, 0, len(parts))
	for _, p := range parts {
		if p != "" {
			nonEmpty = append(nonEmpty, p)
		}
	}
	if len(nonEmpty) == 0 {
		return ""
	}
	return strings.Join(nonEmpty, "-")
}

// getYear returns the year digits from the date string
func getYearMonthDay(d string, dateFmt enums.FilenameDateFormat) (year, month, day string, err error) {
	d = strings.ReplaceAll(d, "-", "")
	d = strings.TrimSpace(d)

	if len(d) >= 8 {
		switch dateFmt {
		case enums.FILEDATE_DD_MM_YY, enums.FILEDATE_MM_DD_YY, enums.FILEDATE_YY_DD_MM, enums.FILEDATE_YY_MM_DD:
			year = d[2:4]
		default:
			year = d[:4]
		}
		month = d[4:6]
		day = d[6:8]

		return year, month, day, nil
	}
	if len(d) >= 6 {
		year = d[:2]
		month = d[2:4]
		day = d[4:6]

		return year, month, day, nil
	}
	if len(d) == 4 { // Guess year or month-day

		i, err := strconv.Atoi(d[:2])
		if err != nil {
			return "", "", "", fmt.Errorf("invalid date string '%s' threw error: %w", d, err)
		}
		j, err := strconv.Atoi(d[2:4])
		if err != nil {
			return "", "", "", fmt.Errorf("invalid date string '%s' threw error: %w", d, err)
		}

		if (i == 20 || i == 19) && j > 12 { // First guess year
			logging.I("Guessing date string '%s' as year", d)
			switch dateFmt {
			case enums.FILEDATE_DD_MM_YY, enums.FILEDATE_MM_DD_YY, enums.FILEDATE_YY_DD_MM, enums.FILEDATE_YY_MM_DD:
				return d[2:4], "", "", nil
			default:
				return d[:4], "", "", nil
			}
		} else { // Second guess, month-date
			if ddmm, mmdd := maybeDayMonth(i, j); ddmm || mmdd {
				if ddmm {
					logging.I("Guessing date string '%s' as day-month")
					day = d[:2]
					month = d[2:4]

				} else if mmdd {
					logging.I("Guessing date string '%s' as month-day")
					day = d[2:4]
					month = d[:2]
				}
				return "", month, day, nil
			} else if i == 20 || i == 19 { // Final guess year
				logging.I("Guessing date string '%s' as year after failed day-month check", d)
				switch dateFmt {
				case enums.FILEDATE_DD_MM_YY, enums.FILEDATE_MM_DD_YY, enums.FILEDATE_YY_DD_MM, enums.FILEDATE_YY_MM_DD:
					return d[2:4], "", "", nil
				default:
					return d[:4], "", "", nil
				}
			}
		}
	}

	return "", "", "", fmt.Errorf("failed to parse year, month, and day from '%s'", d)
}

// validateDateComponents attempts to fix faulty date arrangements
func validateDateComponents(year, month, day string) (string, string, string, error) {

	if isValidMonth(month) && isValidDay(day, month, year) {
		return year, month, day, nil
	}

	// Attempt swapping day and month
	if isValidMonth(day) && isValidDay(month, day, year) {
		return year, day, month, nil
	}

	// Fail check:
	return "", "", "", fmt.Errorf("invalid date components: year=%s, month=%s, day=%s", year, month, day)
}

// isValidMonth checks if the month inputted is a valid month
func isValidMonth(month string) bool {
	m, err := strconv.Atoi(month)
	if err != nil {
		return false
	}
	return m >= 1 && m <= 12
}

// isValidDay checks if the day inputted is a valid day
func isValidDay(day, month, year string) bool {
	d, err := strconv.Atoi(day)
	if err != nil {
		return false
	}

	m, err := strconv.Atoi(month)
	if err != nil {
		return false
	}

	y, err := strconv.Atoi(year)
	if err != nil {
		return false
	}

	if d < 1 || d > 31 {
		return false
	}

	// Months with 30 days
	if m == 4 || m == 6 || m == 9 || m == 11 {
		return d <= 30
	}

	// February
	if m == 2 {
		// Leap year check
		isLeap := y%4 == 0 && (y%100 != 0 || y%400 == 0)
		if isLeap {
			return d <= 29
		}
		return d <= 28
	}

	return true
}

// maybeDayMonth guesses if the input is a DD-MM or MM-DD format
func maybeDayMonth(i, j int) (ddmm, mmdd bool) {
	if i == 0 || i >= 31 || j == 0 || j >= 31 {
		return false, false
	}

	switch {
	case i <= 31 && j <= 12:
		return ddmm, false
	case j <= 31 && i <= 12:
		return false, mmdd
	default:
		return false, false
	}
}
