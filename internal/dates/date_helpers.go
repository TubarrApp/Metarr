package dates

import (
	"fmt"
	enums "metarr/internal/domain/enums"
	logging "metarr/internal/utils/logging"
	"strconv"
	"strings"
)

// ParseDateComponents extracts and validates year, month, and day from the date string
func ParseDateComponents(date string, dateFmt enums.DateFormat) (year, month, day string, err error) {
	date = strings.ReplaceAll(date, "-", "")
	date = strings.TrimSpace(date)

	year, month, day, err = getYearMonthDay(date, dateFmt)
	if err != nil {
		return "", "", "", err
	}

	return validateDateComponents(year, month, day)
}

// Affix a numerical day with the appropriate suffix (e.g. '1st', '2nd', '3rd')
func dayStringSwitch(day string) string {
	var b strings.Builder
	b.Grow(len(day) + 2)
	b.WriteString(day)

	num, err := strconv.Atoi(day)
	if err != nil {
		logging.E(0, "Failed to convert date string to number")
		return day
	}

	if num > 10 && num < 20 {
		b.WriteString("th")
		return b.String()
	}

	switch num % 10 {
	case 1:
		b.WriteString(day + "st")
	case 2:
		b.WriteString(day + "nd")
	case 3:
		b.WriteString(day + "rd")
	default:
		b.WriteString(day + "th")
	}

	return b.String()
}

// Convert a numerical month to a word
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
		logging.E(0, "Failed to make month string from month number '%s'", month)
		monthStr = "Jan"
	}
	return monthStr
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
func getYearMonthDay(d string, dateFmt enums.DateFormat) (year, month, day string, err error) {
	d = strings.ReplaceAll(d, "-", "")
	d = strings.TrimSpace(d)

	if len(d) >= 8 {
		switch dateFmt {
		case enums.DATEFMT_DD_MM_YY, enums.DATEFMT_MM_DD_YY, enums.DATEFMT_YY_DD_MM, enums.DATEFMT_YY_MM_DD:
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
			case enums.DATEFMT_DD_MM_YY, enums.DATEFMT_MM_DD_YY, enums.DATEFMT_YY_DD_MM, enums.DATEFMT_YY_MM_DD:
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
				case enums.DATEFMT_DD_MM_YY, enums.DATEFMT_MM_DD_YY, enums.DATEFMT_YY_DD_MM, enums.DATEFMT_YY_MM_DD:
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
