package dates

import (
	"errors"
	"fmt"
	enums "metarr/internal/domain/enums"
	"metarr/internal/models"
	logging "metarr/internal/utils/logging"
	"strings"

	"github.com/araddon/dateparse"
)

// ParseWordDate parses and formats the inputted word date (e.g. Jan 2nd)
func ParseWordDate(dateString string) (string, error) {

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
		return "", fmt.Errorf("unable to parse date %q to word date", dateNum)
	}
	time := t.Format("01022006")
	if time == "" {
		time = t.Format("010206")
	}

	var day, month, year, dateStr string

	if len(time) < 6 {
		return dateNum, fmt.Errorf("unable to parse date, date %q is too short", time)
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

	dateStr = fmt.Sprintf("%s %s, %s", month, day, year)
	logging.S(1, "Made string form date: %q", dateStr)

	return dateStr, nil
}

// YyyyMmDd converts inputted date strings into the user's defined format
func YyyyMmDd(date string) (string, bool) {

	var t string = ""
	if tIdx := strings.Index(date, "T"); tIdx != -1 {
		t = date[tIdx:]
	}

	date = strings.ReplaceAll(date, "-", "")

	if len(date) >= 8 {
		formatted := fmt.Sprintf("%s-%s-%s%s", date[:4], date[4:6], date[6:8], t)
		logging.S(2, "Made date %s", formatted)
		return formatted, true

	} else if len(date) >= 6 {
		formatted := fmt.Sprintf("%s-%s-%s%s", date[:2], date[2:4], date[4:6], t)
		logging.S(2, "Made date %s", formatted)
		return formatted, true
	}
	logging.D(3, "Returning empty or short date element (%s) without formatting", date)
	return date, false
}

// formatDateString formats the date as a hyphenated string
func FormatDateString(year, month, day string, dateFmt enums.DateFormat) (string, error) {
	var parts [3]string

	switch dateFmt {
	case enums.DATEFMT_YYYY_MM_DD, enums.DATEFMT_YY_MM_DD:
		parts = [3]string{year, month, day}
	case enums.DATEFMT_YYYY_DD_MM, enums.DATEFMT_YY_DD_MM:
		parts = [3]string{year, day, month}
	case enums.DATEFMT_DD_MM_YYYY, enums.DATEFMT_DD_MM_YY:
		parts = [3]string{day, month, year}
	case enums.DATEFMT_MM_DD_YYYY, enums.DATEFMT_MM_DD_YY:
		parts = [3]string{month, day, year}
	}

	result := joinNonEmpty(parts)
	if result == "" {
		return "", errors.New("no valid date components found")
	}
	return result, nil
}

// FormatAllDates formats timestamps into a hyphenated form
func FormatAllDates(fd *models.FileData) string {
	var (
		result string
		err    error
		ok     bool
	)

	d := fd.MDates

	fields := []string{
		d.Originally_Available_At,
		d.ReleaseDate,
		d.Date,
		d.UploadDate,
		d.Creation_Time,
	}

	for _, field := range fields {
		if field != "" {
			logging.D(2, "Attempting to format %+v", field)

			if result, ok = YyyyMmDd(field); ok {
				d.FormattedDate = result
				logging.D(2, "Got formatted date %q", result)

				if d.StringDate, err = ParseNumDate(d.FormattedDate); err != nil {
					logging.E(0, err.Error())
				}
				logging.D(2, "Got string date %q", d.StringDate)
				return result
			}
		}
	}
	logging.E(0, "Failed to format dates")
	return ""
}
