package metadata

import (
	"metarr/internal/dates"
	"metarr/internal/domain/consts"
	"metarr/internal/domain/enums"
	"metarr/internal/models"
	browser "metarr/internal/utils/browser"
	"metarr/internal/utils/logging"
	print "metarr/internal/utils/print"
	"strings"
)

// fillTimestamps grabs timestamp metadata from JSON
func FillTimestamps(fd *models.FileData, json map[string]interface{}) bool {

	t := fd.MDates
	w := fd.MWebData

	fieldMap := map[string]*string{ // Order by importance
		consts.JReleaseDate:         &t.ReleaseDate,
		consts.JOriginallyAvailable: &t.Originally_Available_At,
		consts.JDate:                &t.Date,
		consts.JUploadDate:          &t.UploadDate,
		consts.JReleaseYear:         &t.Year,
		consts.JYear:                &t.Year,
		consts.JCreationTime:        &t.Creation_Time,
	}

	if ok := unpackJSON(fieldMap, json); !ok {
		logging.E(1, "Failed to unpack date JSON, no dates currently exist in file?")
	}

	var printMap map[string]string
	if logging.Level > 1 {
		printMap = make(map[string]string, len(fieldMap))
		defer func() {
			if len(printMap) > 0 {
				print.PrintGrabbedFields("dates", printMap)
			}
		}()
	}

	var gotDate bool
	for k, ptr := range fieldMap {
		if ptr == nil {
			logging.E(0, "Unexpected nil pointer in fieldMap")
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

		var finalVal string
		if len(val) >= 6 {
			if formatted, ok := dates.YyyyMmDd(val); ok {
				finalVal = formatted
			} else {
				finalVal = val
			}
		} else {
			finalVal = val
		}

		*ptr = finalVal
		if logging.Level > 1 {
			printMap[k] = finalVal
		}
		gotDate = true
	}

	var b strings.Builder
	b.Grow(len(consts.TimeSfx) + 10)

	if fillEmptyTimestamps(t, &b) {
		gotDate = true
	}

	var err error
	switch {
	case gotDate:
		logging.D(3, "Got a relevant date, proceeding...")

		if t.FormattedDate == "" {
			dates.FormatAllDates(fd)
		} else if t.StringDate, err = dates.ParseNumDate(t.FormattedDate); err != nil {
			logging.E(0, err.Error())
		}

		if _, err := fd.JSONFileRW.WriteJSON(fieldMap); err != nil {
			logging.E(0, "Failed to write into JSON file %q: %v", fd.JSONFilePath, err)
		}

		return true

	case w.WebpageURL == "":

		logging.I("Page URL not found in metadata, so cannot scrape for missing date in %q", fd.JSONFilePath)
		return false
	}

	scrapedDate := browser.ScrapeMeta(w, enums.WEBCLASS_DATE)
	logging.D(1, "Scraped date: %s", scrapedDate)

	var date string

	if scrapedDate != "" {
		date, err = dates.ParseWordDate(scrapedDate)
		if err != nil || date == "" {
			logging.E(0, "Failed to parse date %q: %v", scrapedDate, err)
			return false
		}
		if t.ReleaseDate == "" {
			t.ReleaseDate = date
		}
		if t.Date == "" {
			t.Date = date
		}
		if !strings.ContainsRune(t.Creation_Time, 'T') {
			t.Creation_Time = formatTimeStamp(date, &b)
		}
		if t.UploadDate == "" {
			t.UploadDate = date
		}
		if t.Originally_Available_At == "" {
			t.Originally_Available_At = date
		}
		if t.FormattedDate == "" {
			t.FormattedDate = date
		}
		if len(date) >= 4 {
			t.Year = date[:4]
		}

		if logging.Level > 1 {
			printMap[consts.JReleaseDate] = t.ReleaseDate
			printMap[consts.JDate] = t.Date
			printMap[consts.JYear] = t.Year
		}

		if t.FormattedDate == "" {
			dates.FormatAllDates(fd)
		}
		if _, err := fd.JSONFileRW.WriteJSON(fieldMap); err != nil {
			logging.E(0, "Failed to write new metadata (%s) into JSON file %q: %v", date, fd.JSONFilePath, err)
		}
		return true
	}
	return false
}

// fillEmptyTimestamps attempts to infer missing timestamps
func fillEmptyTimestamps(t *models.MetadataDates, b *strings.Builder) bool {

	gotDate := false

	// Infer from originally available date
	if t.Originally_Available_At != "" && len(t.Originally_Available_At) >= 6 {
		gotDate = true

		if !strings.ContainsRune(t.Creation_Time, 'T') {
			processDateField(t.Originally_Available_At, &t.Creation_Time, t)
			t.Creation_Time = formatTimeStamp(t.Creation_Time, b)
		}
	}

	// Infer from release date
	if t.ReleaseDate != "" && len(t.ReleaseDate) >= 6 {
		gotDate = true

		if !strings.ContainsRune(t.Creation_Time, 'T') {
			processDateField(t.ReleaseDate, &t.Creation_Time, t)
			t.Creation_Time = formatTimeStamp(t.Creation_Time, b)
		}

		if t.Originally_Available_At == "" {
			processDateField(t.ReleaseDate, &t.Originally_Available_At, t)
		}
	}

	// Infer from date
	if t.Date != "" && len(t.Date) >= 6 {
		gotDate = true

		if !strings.ContainsRune(t.Creation_Time, 'T') {
			processDateField(t.Date, &t.Creation_Time, t)
			t.Creation_Time = formatTimeStamp(t.Creation_Time, b)
		}

		if t.Originally_Available_At == "" {
			processDateField(t.Date, &t.Originally_Available_At, t)
		}
	}

	// Infer from upload date
	if t.UploadDate != "" && len(t.UploadDate) >= 6 {

		if !strings.ContainsRune(t.Creation_Time, 'T') {
			processDateField(t.UploadDate, &t.Creation_Time, t)
			t.Creation_Time = formatTimeStamp(t.Creation_Time, b)
		}

		if t.Originally_Available_At == "" {
			processDateField(t.UploadDate, &t.Originally_Available_At, t)
		}
	}

	// Fill empty date
	if t.Date == "" {
		switch {
		case t.ReleaseDate != "":
			t.Date = t.ReleaseDate
			t.Originally_Available_At = t.ReleaseDate

		case t.UploadDate != "":
			t.Date = t.UploadDate
			t.Originally_Available_At = t.UploadDate

		case t.FormattedDate != "":
			t.Date = t.FormattedDate
		}
	}
	// Fill empty year
	if t.Year == "" {
		switch {
		case t.Date != "" && len(t.Date) >= 4:
			t.Year = t.Date[:4]

		case t.UploadDate != "" && len(t.UploadDate) >= 4:
			t.Year = t.UploadDate[:4]

		case t.FormattedDate != "" && len(t.FormattedDate) >= 4:
			t.Year = t.FormattedDate[:4]
		}
	}
	if len(t.Year) > 4 {
		t.Year = t.Year[:4]
	}

	// Try to fix accidentally using upload date if another date is available
	if len(t.Year) == 4 && !strings.HasPrefix(t.Creation_Time, t.Year) && len(t.Creation_Time) >= 4 {

		logging.D(1, "Creation time does not match year tag, seeing if other dates are available...")

		switch {
		case strings.HasPrefix(t.Originally_Available_At, t.Year):
			if !strings.ContainsRune(t.Creation_Time, 'T') {
				t.Creation_Time = formatTimeStamp(t.Originally_Available_At, b)
			}

			logging.D(1, "Set creation time to %s", t.Originally_Available_At)

		case strings.HasPrefix(t.ReleaseDate, t.Year):
			if !strings.ContainsRune(t.Creation_Time, 'T') {
				t.Creation_Time = formatTimeStamp(t.ReleaseDate, b)
			}

			logging.D(1, "Set creation time to %s", t.ReleaseDate)

		case strings.HasPrefix(t.Date, t.Year):
			if !strings.ContainsRune(t.Creation_Time, 'T') {
				t.Creation_Time = formatTimeStamp(t.Date, b)
			}

			logging.D(1, "Set creation time to %s", t.Date)

		case strings.HasPrefix(t.FormattedDate, t.Year):
			if !strings.ContainsRune(t.Creation_Time, 'T') {
				t.Creation_Time = formatTimeStamp(t.FormattedDate, b)
			}

			logging.D(1, "Set creation time to %s", t.FormattedDate)

		default:
			logging.D(1, "Could not find a match, directly altering t.Creation_Time for year (month and day may therefore be wrong)")
			t.Creation_Time = t.Year + t.Creation_Time[4:]
			logging.D(1, "Set creation time's year only. Got %q", t.Creation_Time)
		}
	}
	return gotDate
}

// formatTimeStamp takes an input date and appends the T time string
func formatTimeStamp(date string, b *strings.Builder) string {
	if b == nil {
		b = &strings.Builder{}
		b.Grow(len(consts.TimeSfx) + 10)
	}

	b.Reset()
	b.WriteString(date)
	b.WriteString(consts.TimeSfx)
	return b.String()
}

// processDateField takes in a filled date, and fills the target with it
func processDateField(date string, target *string, t *models.MetadataDates) {
	if formatted, ok := dates.YyyyMmDd(date); ok {
		if !strings.ContainsRune(formatted, 'T') {
			*target = formatted
			t.FormattedDate = formatted
		} else {
			*target = formatted
			t.FormattedDate, _, _ = strings.Cut(formatted, "T")
		}
	} else {
		*target = date
	}
}
