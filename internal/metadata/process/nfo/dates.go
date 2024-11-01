package metadata

import (
	consts "Metarr/internal/domain/constants"
	enums "Metarr/internal/domain/enums"
	helpers "Metarr/internal/metadata/process/helpers"
	"Metarr/internal/types"
	logging "Metarr/internal/utils/logging"
	print "Metarr/internal/utils/print"
)

func fillNFOTimestamps(fd *types.FileData) bool {

	t := fd.MDates
	w := fd.MWebData
	n := fd.NFOData

	fieldMap := map[string]*string{
		consts.NAired:        &t.Date,
		consts.NPremiereDate: &t.ReleaseDate,
		consts.NYear:         &t.Year,
	}

	cleanEmptyFields(fieldMap)

	gotRelevantDate := false
	printMap := make(map[string]string, len(fieldMap))

	if n.Premiered != "" {
		if rtn, ok := helpers.YyyyMmDd(n.Premiered); ok && rtn != "" {
			if t.FormattedDate == "" {
				t.FormattedDate = rtn
			}
		}
		printMap[consts.NPremiereDate] = n.Premiered
		gotRelevantDate = true
	}
	if n.ReleaseDate != "" {
		if rtn, ok := helpers.YyyyMmDd(n.ReleaseDate); ok && rtn != "" {
			if t.FormattedDate == "" {
				t.FormattedDate = rtn
			}
		}
		printMap[consts.NAired] = n.Premiered
		gotRelevantDate = true
	}
	if n.Year != "" {
		t.Year = n.Year
		printMap[consts.NYear] = n.Year
	}

	if t.FormattedDate != "" {
		if t.Date == "" {
			t.Date = t.FormattedDate
		}
		if t.ReleaseDate == "" {
			t.ReleaseDate = t.FormattedDate
		}
		if t.Creation_Time == "" {
			t.Creation_Time = t.FormattedDate + "T00:00:00Z"
		}
		gotRelevantDate = true
	}

	switch {
	case gotRelevantDate:

		var err error

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

	case w.WebpageURL == "":

		logging.PrintI("Page URL not found in metadata, so cannot scrape for missing date in '%s'", fd.JSONFilePath)
		print.PrintGrabbedFields("time and date", &printMap)
		return false
	}

	scrapedDate := helpers.ScrapeMeta(w, enums.WEBCLASS_DATE)
	logging.PrintD(1, "Scraped date: %s", scrapedDate)

	logging.PrintD(3, "Passed web scrape attempt for date.")

	var date string
	var err error
	if scrapedDate != "" {
		date, err = helpers.ParseStringDate(scrapedDate)
		if err != nil || date == "" {
			logging.PrintE(0, "Failed to parse date '%s': %v", scrapedDate, err)
			return false
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

			printMap[consts.NPremiereDate] = t.ReleaseDate
			printMap[consts.NAired] = t.Date
			printMap[consts.NYear] = t.Year

			print.PrintGrabbedFields("time and date", &printMap)

			if t.FormattedDate == "" {
				helpers.FormatAllDates(fd)
			}
		}
	}

	return true
}
