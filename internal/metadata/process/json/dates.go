package metadata

import (
	consts "Metarr/internal/domain/constants"
	enums "Metarr/internal/domain/enums"
	helpers "Metarr/internal/metadata/process/helpers"
	"Metarr/internal/models"
	browser "Metarr/internal/utils/browser"
	logging "Metarr/internal/utils/logging"
	print "Metarr/internal/utils/print"
	"strings"
)

// fillTimestamps grabs timestamp metadata from JSON
func fillTimestamps(fd *models.FileData, data map[string]interface{}) (map[string]interface{}, bool) {
	var (
		err             error
		gotRelevantDate bool
	)

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
		logging.PrintE(1, "Failed to unpack date JSON, no dates currently exist in file?")
	}

	printMap := make(map[string]string, len(fieldMap))

	for key, value := range data {
		if strVal, ok := value.(string); ok {
			if _, exists := fieldMap[key]; exists {

				if len(strVal) >= 6 {
					if formatted, ok := helpers.YyyyMmDd(strVal); ok {
						*fieldMap[key] = formatted
						printMap[key] = formatted
						gotRelevantDate = true
						continue

					} else {
						*fieldMap[key] = strVal
						printMap[key] = strVal
						gotRelevantDate = true
						continue
					}
				} else {
					*fieldMap[key] = strVal
					printMap[key] = strVal
					gotRelevantDate = true
					continue
				}
			}
		}
		continue
	}

	if fillEmptyTimestamps(t) {
		gotRelevantDate = true
	}

	switch {
	case gotRelevantDate:

		logging.PrintD(3, "Got a relevant date, proceeding...")
		print.PrintGrabbedFields("time and date", &printMap)
		if t.FormattedDate == "" {
			helpers.FormatAllDates(fd)
		} else {
			t.StringDate, err = helpers.ParseNumDate(t.FormattedDate)
			if err != nil {
				logging.PrintE(0, err.Error())
			}
		}

		rtn, err := fd.JSONFileRW.WriteMetadata(fieldMap)
		if err != nil {
			logging.PrintE(0, "Failed to write into JSON file '%s': %v", fd.JSONFilePath, err)
			return data, true
		} else if rtn != nil {
			data = rtn
			return data, true
		}

	case w.WebpageURL == "":

		logging.PrintI("Page URL not found in metadata, so cannot scrape for missing date in '%s'", fd.JSONFilePath)
		print.PrintGrabbedFields("time and date", &printMap)
		return data, false
	}

	scrapedDate := browser.ScrapeMeta(w, enums.WEBCLASS_DATE)
	logging.PrintD(1, "Scraped date: %s", scrapedDate)

	logging.PrintD(3, "Passed web scrape attempt for date.")

	var date string
	if scrapedDate != "" {
		date, err = helpers.ParseStringDate(scrapedDate)
		if err != nil || date == "" {
			logging.PrintE(0, "Failed to parse date '%s': %v", scrapedDate, err)
			return data, false
		} else {
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

			print.PrintGrabbedFields("time and date", &printMap)

			if t.FormattedDate == "" {
				helpers.FormatAllDates(fd)
			}
			rtn, err := fd.JSONFileRW.WriteMetadata(fieldMap)
			switch {
			case err != nil:
				logging.PrintE(0, "Failed to write new metadata (%s) into JSON file '%s': %v", date, fd.JSONFilePath, err)
				return data, true
			case rtn != nil:
				data = rtn
				return data, true
			}
		}
	}
	return data, false
}

// fillEmptyTimestamps attempts to infer missing timestamps
func fillEmptyTimestamps(t *models.MetadataDates) bool {

	gotRelevantDate := false

	// Infer from originally available date
	if t.Originally_Available_At != "" && len(t.Originally_Available_At) >= 6 {
		gotRelevantDate = true
		if t.Creation_Time == "" {
			if formatted, ok := helpers.YyyyMmDd(t.Originally_Available_At); ok {
				if !strings.ContainsRune(formatted, 'T') {
					t.Creation_Time = formatted + "T00:00:00Z"
					t.FormattedDate = formatted
				} else {
					t.Creation_Time = formatted
					t.FormattedDate, _, _ = strings.Cut(formatted, "T")
				}
			} else {
				if formatted, ok := helpers.YyyyMmDd(t.Originally_Available_At); ok {
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
		gotRelevantDate = true
		if t.Creation_Time == "" {
			if formatted, ok := helpers.YyyyMmDd(t.ReleaseDate); ok {
				t.Creation_Time = formatted + "T00:00:00Z"
				if t.FormattedDate == "" {
					t.FormattedDate = formatted
				}
			} else {
				t.Creation_Time = t.ReleaseDate + "T00:00:00Z"
			}
		}
		if t.Originally_Available_At == "" {
			if formatted, ok := helpers.YyyyMmDd(t.ReleaseDate); ok {
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
		gotRelevantDate = true
		if formatted, ok := helpers.YyyyMmDd(t.ReleaseDate); ok {
			t.Creation_Time = formatted + "T00:00:00Z"
			if t.FormattedDate == "" {
				t.FormattedDate = formatted
			}
		} else {
			t.Creation_Time = t.Date + "T00:00:00Z"
		}
		if t.Originally_Available_At == "" {
			if formatted, ok := helpers.YyyyMmDd(t.ReleaseDate); ok {
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
		if formatted, ok := helpers.YyyyMmDd(t.UploadDate); ok {
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
	return gotRelevantDate
}
