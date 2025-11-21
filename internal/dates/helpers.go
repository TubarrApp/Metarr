package dates

import (
	"fmt"
	"metarr/internal/domain/enums"
	"metarr/internal/domain/logger"
	"metarr/internal/domain/regex"
	"strconv"
	"strings"

	"github.com/TubarrApp/gocommon/sharedtags"
)

// ParseDateComponents extracts and validates year, month, and day from the date string.
func ParseDateComponents(date string, dateFmt enums.DateFormat) (year, month, day string, err error) {
	date = strings.ReplaceAll(date, "-", "")
	date = strings.TrimSpace(date)

	year, month, day, err = getYearMonthDay(date, dateFmt)
	if err != nil {
		return "", "", "", err
	}

	return validateDateComponents(year, month, day)
}

// dayStringSwitch appends a numerical day with the appropriate suffix (e.g. '1st', '2nd', '3rd').
func dayStringSwitch(day string) string {
	var b strings.Builder
	b.Grow(len(day) + 2)

	num, err := strconv.Atoi(day)
	if err != nil {
		logger.Pl.E("Failed to convert date string to number")
		return day
	}

	b.WriteString(day)

	if num > 10 && num < 20 {
		b.WriteString("th")
		return b.String()
	}

	switch num % 10 {
	case 1:
		b.WriteString("st")
	case 2:
		b.WriteString("nd")
	case 3:
		b.WriteString("rd")
	default:
		b.WriteString("th")
	}

	return b.String()
}

// monthStringSwitch converts a numerical month to a word.
func monthStringSwitch(month string) string {
	var monthStr string
	switch month {
	case "01":
		monthStr = "Jan"
	case "02":
		monthStr = "Feb"
	case "03":
		monthStr = "Mar"
	case "04":
		monthStr = "Apr"
	case "05":
		monthStr = "May"
	case "06":
		monthStr = "Jun"
	case "07":
		monthStr = "Jul"
	case "08":
		monthStr = "Aug"
	case "09":
		monthStr = "Sep"
	case "10":
		monthStr = "Oct"
	case "11":
		monthStr = "Nov"
	case "12":
		monthStr = "Dec"
	default:
		logger.Pl.E("Failed to make month string from month number %q", month)
		monthStr = "Jan"
	}
	return monthStr
}

// joinNonEmpty joins non-empty strings from an array with hyphens.
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

// getYearMonthDay returns the year, month, and day digits from the date string.
func getYearMonthDay(d string, dateFmt enums.DateFormat) (year, month, day string, err error) {
	d = strings.ReplaceAll(d, "-", "")
	d = strings.TrimSpace(d)

	if len(d) >= 8 {
		switch dateFmt {
		case enums.DateDdMmYy, enums.DateMmDdYy, enums.DateYyDdMm, enums.DateYyMmDd:
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
	if len(d) == 4 { // Guess year or month-day.

		i, err := strconv.Atoi(d[:2])
		if err != nil {
			return "", "", "", fmt.Errorf("invalid date string %q threw error: %w", d, err)
		}
		j, err := strconv.Atoi(d[2:4])
		if err != nil {
			return "", "", "", fmt.Errorf("invalid date string %q threw error: %w", d, err)
		}

		if (i == 20 || i == 19) && j > 12 { // First guess year.
			logger.Pl.I("Guessing date string %q as year", d)
			switch dateFmt {
			case enums.DateDdMmYy, enums.DateMmDdYy, enums.DateYyDdMm, enums.DateYyMmDd:
				return d[2:4], "", "", nil
			default:
				return d[:4], "", "", nil
			}
		} else { // Second guess, month-date.
			if ddmm, mmdd := maybeDayMonth(i, j); ddmm || mmdd {
				if ddmm {
					logger.Pl.I("Guessing date string %q as day-month", d)
					day = d[:2]
					month = d[2:4]

				} else if mmdd {
					logger.Pl.I("Guessing date string %q as month-day", d)
					day = d[2:4]
					month = d[:2]
				}
				return "", month, day, nil
			} else if i == 20 || i == 19 { // Final guess year.
				logger.Pl.I("Guessing date string %q as year after failed day-month check", d)
				switch dateFmt {
				case enums.DateDdMmYy, enums.DateMmDdYy, enums.DateYyDdMm, enums.DateYyMmDd:
					return d[2:4], "", "", nil
				default:
					return d[:4], "", "", nil
				}
			}
		}
	}
	return "", "", "", fmt.Errorf("failed to parse year, month, and day from %q", d)
}

// validateDateComponents attempts to fix faulty date arrangements.
func validateDateComponents(year, month, day string) (y, m, d string, err error) {
	if isValidMonth(month) && isValidDay(day, month, year) {
		return year, month, day, nil
	}

	// Attempt swapping day and month.
	if isValidMonth(day) && isValidDay(month, day, year) {
		return year, month, day, nil
	}

	// Fail check:
	return "", "", "", fmt.Errorf("invalid date components: year=%s, month=%s, day=%s", year, month, day)
}

// isValidMonth checks if the month inputted is a valid month.
func isValidMonth(month string) bool {
	m, err := strconv.Atoi(month)
	if err != nil {
		return false
	}
	return m >= 1 && m <= 12
}

// isValidDay checks if the day inputted is a valid day.
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

	// Months with 30 days.
	if m == 4 || m == 6 || m == 9 || m == 11 {
		return d <= 30
	}

	// February.
	if m == 2 {
		// Leap year check.
		isLeap := y%4 == 0 && (y%100 != 0 || y%400 == 0)
		if isLeap {
			return d <= 29
		}
		return d <= 28
	}

	return true
}

// maybeDayMonth guesses if the input is a DD-MM or MM-DD format.
func maybeDayMonth(i, j int) (ddmm, mmdd bool) {
	if i == 0 || i >= 31 || j == 0 || j >= 31 {
		return false, false
	}

	switch {
	case i <= 31 && j <= 12:
		return true, false
	case j <= 31 && i <= 12:
		return false, true
	default:
		return false, false
	}
}

// StripDateTags removes [yy-mm-dd] or [yyyy-mm-dd] tags from a field string.
func StripDateTags(val string, loc enums.DateTagLocation) (dateStrs []string, result string) {
	val = strings.TrimSpace(val)

	switch loc {
	case enums.DateTagLocPrefix:
		openTag := strings.IndexRune(val, '[')
		closeTag := strings.IndexRune(val, ']')

		if openTag == 0 && closeTag > openTag {
			dateStr := val[openTag+1 : closeTag]
			if regex.DateTagCompile().MatchString(dateStr) {
				return []string{dateStr}, strings.TrimLeft(val[closeTag+1:], " ")
			}
		}
	case enums.DateTagLocSuffix:
		openTag := strings.LastIndex(val, "[")
		closeTag := strings.LastIndex(val, "]")

		if openTag >= 0 && closeTag > openTag && closeTag == len(val)-1 {
			dateStr := val[openTag+1 : closeTag]
			if regex.DateTagCompile().MatchString(dateStr) {
				return []string{dateStr}, strings.TrimSpace(val[:openTag])
			}
		}
	case enums.DateTagLocAll:
		return stripAllDateTags(val)
	}
	return nil, val
}

// stripAllDateTags removes all date tags and returns them.
func stripAllDateTags(val string) (tags []string, cleaned string) {
	logger.Pl.D(3, "stripAllDateTags input: %q", val)

	pattern := regex.DateTagWithBracketsCompile()
	logger.Pl.D(3, "Pattern: %v", pattern.String())

	// Find all tags.
	tags = pattern.FindAllString(val, -1)
	logger.Pl.D(3, "Found tags: %v (count: %d)", tags, len(tags))

	// Remove all tags.
	cleaned = pattern.ReplaceAllString(val, "")
	logger.Pl.D(3, "After removal: %q", cleaned)

	// Clean up extra spaces.
	cleaned = strings.TrimSpace(cleaned)
	cleaned = regex.DoubleSpacesCompile().ReplaceAllString(cleaned, " ")
	logger.Pl.D(3, "Final cleaned: %q", cleaned)

	return tags, cleaned
}

// extractDateFromMetadata attempts to find a date in the metadata using predefined fields.
func extractDateFromMetadata(metadata map[string]any) (string, bool) {
	preferredDateFields := []string{
		sharedtags.JReleaseDate,
		"releasedate",
		"released_on",
		sharedtags.JOriginallyAvailable,
		"originally_available",
		"originallyavailable",
		sharedtags.JDate,
		sharedtags.JUploadDate,
		"uploaddate",
		"uploaded_on",
		sharedtags.JCreationTime, // Last resort, may give false positives.
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
