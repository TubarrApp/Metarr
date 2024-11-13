package metadata

import (
	"metarr/internal/dates"
	consts "metarr/internal/domain/constants"
	enums "metarr/internal/domain/enums"
	"metarr/internal/models"
	browser "metarr/internal/utils/browser"
	logging "metarr/internal/utils/logging"
	print "metarr/internal/utils/print"
	"strings"
)

// fillTimestamps grabs timestamp metadata from JSON
func FillTimestamps(fd *models.FileData, data map[string]interface{}) bool {

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

	if ok := unpackJSON("date", fieldMap, data); !ok {
		logging.E(1, "Failed to unpack date JSON, no dates currently exist in file?")
	}

	printMap := make(map[string]string, len(fieldMap))

	var gotDate bool
	for k, v := range data {
		val, ok := v.(string)
		if !ok {
			continue
		}

		fieldPtr, exists := fieldMap[k]
		if !exists {
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

		*fieldPtr = finalVal
		printMap[k] = finalVal
		gotDate = true
	}

	if fillEmptyTimestamps(t) {
		gotDate = true
	}

	var err error
	switch {
	case gotDate:

		logging.D(3, "Got a relevant date, proceeding...")

		if logging.Level > -1 {
			print.PrintGrabbedFields("time and date", &printMap)
		}

		if t.FormattedDate == "" {
			dates.FormatAllDates(fd)
		} else if t.StringDate, err = dates.ParseNumDate(t.FormattedDate); err != nil {
			logging.E(0, err.Error())
		}

		if _, err := fd.JSONFileRW.WriteJSON(fieldMap); err != nil {
			logging.E(0, "Failed to write into JSON file '%s': %v", fd.JSONFilePath, err)
		}

		return true

	case w.WebpageURL == "":

		logging.I("Page URL not found in metadata, so cannot scrape for missing date in '%s'", fd.JSONFilePath)
		if logging.Level > -1 {
			print.PrintGrabbedFields("time and date", &printMap)
		}
		return false
	}

	scrapedDate := browser.ScrapeMeta(w, enums.WEBCLASS_DATE)
	logging.D(1, "Scraped date: %s", scrapedDate)

	logging.D(3, "Passed web scrape attempt for date.")

	var date string
	if scrapedDate != "" {
		date, err = dates.ParseWordDate(scrapedDate)
		if err != nil || date == "" {
			logging.E(0, "Failed to parse date '%s': %v", scrapedDate, err)
			return false
		}
		if t.ReleaseDate == "" {
			t.ReleaseDate = date
		}
		if t.Date == "" {
			t.Date = date
		}
		if t.Creation_Time == "" {
			t.Creation_Time = date + "T00:00:00Z"
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

		printMap[consts.JReleaseDate] = t.ReleaseDate
		printMap[consts.JDate] = t.Date
		printMap[consts.JYear] = t.Year

		if logging.Level > -1 {
			print.PrintGrabbedFields("time and date", &printMap)
		}

		if t.FormattedDate == "" {
			dates.FormatAllDates(fd)
		}
		if _, err := fd.JSONFileRW.WriteJSON(fieldMap); err != nil {
			logging.E(0, "Failed to write new metadata (%s) into JSON file '%s': %v", date, fd.JSONFilePath, err)
		}
		return true
	}
	return false
}

// fillEmptyTimestamps attempts to infer missing timestamps
func fillEmptyTimestamps(t *models.MetadataDates) bool {

	gotDate := false

	// Infer from originally available date
	if t.Originally_Available_At != "" && len(t.Originally_Available_At) >= 6 {

		gotDate = true
		if t.Creation_Time == "" {
			if formatted, ok := dates.YyyyMmDd(t.Originally_Available_At); ok {
				if !strings.ContainsRune(formatted, 'T') {
					t.Creation_Time = formatted + "T00:00:00Z"
					t.FormattedDate = formatted
				} else {
					t.Creation_Time = formatted
					t.FormattedDate, _, _ = strings.Cut(formatted, "T")
				}
			} else {
				if formatted, ok := dates.YyyyMmDd(t.Originally_Available_At); ok {
					if !strings.ContainsRune(formatted, 'T') {
						t.Creation_Time = formatted + "T00:00:00Z"
						t.FormattedDate = formatted
					} else {
						t.Creation_Time = formatted
						t.FormattedDate, _, _ = strings.Cut(formatted, "T")
					}
				} else {
					t.Creation_Time = t.Originally_Available_At + "T00:00:00Z"
				}
			}
		}
	}

	// Infer from release date
	if t.ReleaseDate != "" && len(t.ReleaseDate) >= 6 {
		gotDate = true
		if t.Creation_Time == "" {
			if formatted, ok := dates.YyyyMmDd(t.ReleaseDate); ok {
				t.Creation_Time = formatted + "T00:00:00Z"
				if t.FormattedDate == "" {
					t.FormattedDate = formatted
				}
			} else {
				t.Creation_Time = t.ReleaseDate + "T00:00:00Z"
			}
		}
		if t.Originally_Available_At == "" {
			if formatted, ok := dates.YyyyMmDd(t.ReleaseDate); ok {
				t.Originally_Available_At = formatted
				if t.FormattedDate == "" {
					t.FormattedDate = formatted
				}
			} else {
				t.Originally_Available_At = t.ReleaseDate
			}
		}
	}
	// Infer from date
	if t.Date != "" && len(t.Date) >= 6 {
		gotDate = true
		if formatted, ok := dates.YyyyMmDd(t.ReleaseDate); ok {
			t.Creation_Time = formatted + "T00:00:00Z"
			if t.FormattedDate == "" {
				t.FormattedDate = formatted
			}
		} else {
			t.Creation_Time = t.Date + "T00:00:00Z"
		}
		if t.Originally_Available_At == "" {
			if formatted, ok := dates.YyyyMmDd(t.ReleaseDate); ok {
				t.Originally_Available_At = formatted
				if t.FormattedDate == "" {
					t.FormattedDate = formatted
				}
			} else {
				t.Originally_Available_At = t.Date
			}
		}
	}

	// Infer from upload date
	if t.UploadDate != "" && len(t.UploadDate) >= 6 {
		if formatted, ok := dates.YyyyMmDd(t.UploadDate); ok {
			t.Creation_Time = formatted + "T00:00:00Z"
			if t.FormattedDate == "" {
				t.FormattedDate = formatted
			}
		} else {
			t.Creation_Time = t.UploadDate + "T00:00:00Z"
		}
		if t.Originally_Available_At == "" {
			t.Originally_Available_At = t.UploadDate
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
			t.Creation_Time = t.Originally_Available_At + "T00:00:00Z"
			logging.D(1, "Changed creation time to %s", t.Originally_Available_At)

		case strings.HasPrefix(t.ReleaseDate, t.Year):
			t.Creation_Time = t.ReleaseDate + "T00:00:00Z"
			logging.D(1, "Changed creation time to %s", t.ReleaseDate)

		case strings.HasPrefix(t.Date, t.Year):
			t.Creation_Time = t.Date + "T00:00:00Z"
			logging.D(1, "Changed creation time to %s", t.Date)

		case strings.HasPrefix(t.FormattedDate, t.Year):
			t.Creation_Time = t.FormattedDate + "T00:00:00Z"
			logging.D(1, "Changed creation time to %s", t.FormattedDate)

		default:
			logging.D(1, "Could not find a match, directly altering t.Creation_Time for year (month and day may therefore be wrong)")
			t.Creation_Time = t.Year + t.Creation_Time[4:]
			logging.D(1, "Changed creation time's year only. Got '%s'", t.Creation_Time)
		}
	}
	return gotDate
}
