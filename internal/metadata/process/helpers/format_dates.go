package metadata

import (
	"Metarr/internal/types"
	logging "Metarr/internal/utils/logging"
	"fmt"
	"strconv"
	"strings"

	"github.com/araddon/dateparse"
)

// ParseAndFormatDate parses and formats the inputted date string
func ParseStringDate(dateString string) (string, error) {

	t, err := dateparse.ParseAny(dateString)
	if err != nil {
		return "", fmt.Errorf("unable to parse date: %s", dateString)
	}

	return t.Format("2006-01-02"), nil
}

// ParseAndFormatDate parses and formats the inputted date string
func ParseNumDate(dateNum string) (string, error) {

	t, err := dateparse.ParseAny(dateNum)
	if err != nil {
		return "", fmt.Errorf("unable to parse date '%s' to word date", dateNum)
	}
	time := t.Format("01022006")
	if time == "" {
		time = t.Format("010206")
	}

	var day, month, year, dateStr string

	if len(time) < 6 {
		return dateNum, fmt.Errorf("unable to parse date, date '%s' is too short", time)
	}

	if len(time) >= 8 {
		day = time[2:4]
		month = time[:2]
		year = time[4:8]
	} else if len(time) >= 6 {
		day = time[2:4]
		month = time[:2]
		year = time[4:6]
	}

	month = monthStringSwitch(month)
	day = dayStringSwitch(day)

	dateStr = month + " " + day + ", " + year
	logging.PrintS(1, "Made string form date: '%s'", dateStr)

	return dateStr, nil
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
		logging.PrintE(0, "Failed to make month string from month number '%s'", month)
		monthStr = "Jan"
	}
	return monthStr
}

// Affix a numerical day with the appropriate suffix (e.g. '1st', '2nd', '3rd')
func dayStringSwitch(day string) string {
	if thCheck, err := strconv.Atoi(day); err != nil {
		logging.PrintE(0, "Failed to convert date string to number")
		return day
	} else if thCheck > 10 && thCheck < 20 {
		return day + "th"
	}
	switch {
	case strings.HasSuffix(day, "1"):
		return day + "st"
	case strings.HasSuffix(day, "2"):
		return day + "nd"
	case strings.HasSuffix(day, "3"):
		return day + "rd"
	default:
		return day + "th"
	}
}

// YyyyMmDd converts inputted date strings into the user's defined format
func YyyyMmDd(fieldValue string) (string, bool) {

	var t string = ""
	if tIdx := strings.Index(fieldValue, "T"); tIdx != -1 {
		t = fieldValue[tIdx:]
	}

	fieldValue = strings.ReplaceAll(fieldValue, "-", "")

	if len(fieldValue) >= 8 {
		formatted := fmt.Sprintf("%s-%s-%s%s", fieldValue[:4], fieldValue[4:6], fieldValue[6:8], t)
		logging.PrintS(2, "Made date %s", formatted)
		return formatted, true

	} else if len(fieldValue) >= 6 {
		formatted := fmt.Sprintf("%s-%s-%s%s", fieldValue[:2], fieldValue[2:4], fieldValue[4:6], t)
		logging.PrintS(2, "Made date %s", formatted)
		return formatted, true
	}
	logging.PrintD(3, "Returning empty or short date element (%s) without formatting", fieldValue)
	return fieldValue, false
}

// FormatAllDates formats timestamps into a hyphenated form
func FormatAllDates(fd *types.FileData) string {

	var (
		result = ""
		ok     = false
	)

	d := fd.MDates

	if !ok && d.Originally_Available_At != "" {
		logging.PrintD(2, "Attempting to format originally available date: %v", d.Originally_Available_At)
		result, ok = YyyyMmDd(d.Originally_Available_At)
	}
	if !ok && d.ReleaseDate != "" {
		logging.PrintD(2, "Attempting to format release date: %v", d.ReleaseDate)
		result, ok = YyyyMmDd(d.ReleaseDate)
	}
	if !ok && d.Date != "" {
		logging.PrintD(2, "Attempting to format date: %v", d.Date)
		result, ok = YyyyMmDd(d.Date)
	}
	if !ok && d.UploadDate != "" {
		logging.PrintD(2, "Attempting to format upload date: %v", d.UploadDate)
		result, ok = YyyyMmDd(d.UploadDate)
	}
	if !ok && d.Creation_Time != "" {
		logging.PrintD(3, "Attempting to format creation time: %v", d.Creation_Time)
		result, ok = YyyyMmDd(d.Creation_Time)
	}
	if !ok {
		logging.PrintE(0, "Failed to format dates")
		return ""
	} else {
		logging.PrintD(2, "Exiting with formatted date: %v", result)

		d.FormattedDate = result

		logging.PrintD(2, "Got formatted date '%s' and entering parse to string function...", result)

		var err error
		d.StringDate, err = ParseNumDate(d.FormattedDate)
		if err != nil {
			logging.PrintE(0, err.Error())
		}

		return result
	}
}
