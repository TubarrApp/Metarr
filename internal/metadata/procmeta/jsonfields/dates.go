package jsonfields

import (
	"metarr/internal/dates"
	"metarr/internal/domain/consts"
	"metarr/internal/domain/enums"
	"metarr/internal/models"
	"metarr/internal/utils/browser"
	"metarr/internal/utils/logging"
	"metarr/internal/utils/printout"
	"strings"
)

// FillTimestamps grabs timestamp metadata from JSON.
func FillTimestamps(fd *models.FileData, json map[string]any) bool {
	t := fd.MDates
	w := fd.MWebData

	fieldMap := map[string]*string{ // Order by importance
		consts.JReleaseDate:         &t.ReleaseDate,
		consts.JOriginallyAvailable: &t.OriginallyAvailableAt,
		consts.JDate:                &t.Date,
		consts.JUploadDate:          &t.UploadDate,
		consts.JReleaseYear:         &t.Year,
		consts.JYear:                &t.Year,
		consts.JCreationTime:        &t.CreationTime,
	}

	if ok := unpackJSON(fieldMap, json); !ok {
		logging.E("Failed to unpack date JSON, no dates currently exist in file?")
	}

	printMap := make(map[string]string, len(fieldMap))
	if logging.Level > 1 {
		defer func() {
			if len(printMap) > 0 {
				printout.PrintGrabbedFields("dates", printMap)
			}
		}()
	}

	var gotDate bool
	for k, ptr := range fieldMap {
		if ptr == nil {
			logging.E("Unexpected nil pointer in fieldMap")
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
			if formatted, ok := dates.YmdFromMeta(val); ok {
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
			logging.E("Failed to parse date %q: %v", t.FormattedDate, err)
		}

		if _, err := fd.JSONFileRW.WriteJSON(fieldMap); err != nil {
			logging.E("Failed to write into JSON file %q: %v", fd.JSONFilePath, err)
		}

		return true

	case w.WebpageURL == "":
		logging.I("Page URL not found in metadata, so cannot scrape for missing date in %q", fd.JSONFilePath)
		return false
	}

	scrapedDate := browser.ScrapeMeta(w, enums.WebclassDate)
	logging.D(1, "Scraped date: %s", scrapedDate)

	var date string

	if scrapedDate != "" {
		date, err = dates.ParseWordDate(scrapedDate)
		if err != nil || date == "" {
			logging.E("Failed to parse date %q: %v", scrapedDate, err)
			return false
		}
		if t.ReleaseDate == "" {
			t.ReleaseDate = date
		}
		if t.Date == "" {
			t.Date = date
		}
		if !strings.ContainsRune(t.CreationTime, 'T') {
			t.CreationTime = formatTimeStamp(date, &b)
		}
		if t.UploadDate == "" {
			t.UploadDate = date
		}
		if t.OriginallyAvailableAt == "" {
			t.OriginallyAvailableAt = date
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
			logging.E("Failed to write new metadata (%s) into JSON file %q: %v", date, fd.JSONFilePath, err)
		}
		return true
	}
	return false
}

// fillEmptyTimestamps attempts to infer missing timestamps.
func fillEmptyTimestamps(t *models.MetadataDates, b *strings.Builder) bool {

	gotDate := false

	// Infer from originally available date
	if t.OriginallyAvailableAt != "" && len(t.OriginallyAvailableAt) >= 6 {
		gotDate = true

		if !strings.ContainsRune(t.CreationTime, 'T') {
			processDateField(t.OriginallyAvailableAt, &t.CreationTime, t)
			t.CreationTime = formatTimeStamp(t.CreationTime, b)
		}
	}

	// Infer from release date
	if t.ReleaseDate != "" && len(t.ReleaseDate) >= 6 {
		gotDate = true

		if !strings.ContainsRune(t.CreationTime, 'T') {
			processDateField(t.ReleaseDate, &t.CreationTime, t)
			t.CreationTime = formatTimeStamp(t.CreationTime, b)
		}

		if t.OriginallyAvailableAt == "" {
			processDateField(t.ReleaseDate, &t.OriginallyAvailableAt, t)
		}
	}

	// Infer from date
	if t.Date != "" && len(t.Date) >= 6 {
		gotDate = true

		if !strings.ContainsRune(t.CreationTime, 'T') {
			processDateField(t.Date, &t.CreationTime, t)
			t.CreationTime = formatTimeStamp(t.CreationTime, b)
		}

		if t.OriginallyAvailableAt == "" {
			processDateField(t.Date, &t.OriginallyAvailableAt, t)
		}
	}

	// Infer from upload date
	if t.UploadDate != "" && len(t.UploadDate) >= 6 {

		if !strings.ContainsRune(t.CreationTime, 'T') {
			processDateField(t.UploadDate, &t.CreationTime, t)
			t.CreationTime = formatTimeStamp(t.CreationTime, b)
		}

		if t.OriginallyAvailableAt == "" {
			processDateField(t.UploadDate, &t.OriginallyAvailableAt, t)
		}
	}

	// Fill empty date
	if t.Date == "" {
		switch {
		case t.ReleaseDate != "":
			t.Date = t.ReleaseDate
			t.OriginallyAvailableAt = t.ReleaseDate

		case t.UploadDate != "":
			t.Date = t.UploadDate
			t.OriginallyAvailableAt = t.UploadDate

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
	if len(t.Year) == 4 && !strings.HasPrefix(t.CreationTime, t.Year) && len(t.CreationTime) >= 4 {

		logging.D(1, "Creation time does not match year tag, seeing if other dates are available...")

		switch {
		case strings.HasPrefix(t.OriginallyAvailableAt, t.Year):
			if !strings.ContainsRune(t.CreationTime, 'T') {
				t.CreationTime = formatTimeStamp(t.OriginallyAvailableAt, b)
			}

			logging.D(1, "Set creation time to %s", t.OriginallyAvailableAt)

		case strings.HasPrefix(t.ReleaseDate, t.Year):
			if !strings.ContainsRune(t.CreationTime, 'T') {
				t.CreationTime = formatTimeStamp(t.ReleaseDate, b)
			}

			logging.D(1, "Set creation time to %s", t.ReleaseDate)

		case strings.HasPrefix(t.Date, t.Year):
			if !strings.ContainsRune(t.CreationTime, 'T') {
				t.CreationTime = formatTimeStamp(t.Date, b)
			}

			logging.D(1, "Set creation time to %s", t.Date)

		case strings.HasPrefix(t.FormattedDate, t.Year):
			if !strings.ContainsRune(t.CreationTime, 'T') {
				t.CreationTime = formatTimeStamp(t.FormattedDate, b)
			}

			logging.D(1, "Set creation time to %s", t.FormattedDate)

		default:
			logging.D(1, "Could not find a match, directly altering t.CreationTime for year (month and day may therefore be wrong)")
			t.CreationTime = t.Year + t.CreationTime[4:]
			logging.D(1, "Set creation time's year only. Got %q", t.CreationTime)
		}
	}
	return gotDate
}

// formatTimeStamp takes an input date and appends the T time string.
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

// processDateField takes in a filled date, and fills the target with it.
func processDateField(date string, target *string, t *models.MetadataDates) {
	if formatted, ok := dates.YmdFromMeta(date); ok {
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
