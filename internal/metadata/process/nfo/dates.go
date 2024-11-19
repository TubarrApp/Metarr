package metadata

import (
	"fmt"
	"metarr/internal/dates"
	"metarr/internal/domain/consts"
	"metarr/internal/domain/enums"
	"metarr/internal/models"
	browser "metarr/internal/utils/browser"
	"metarr/internal/utils/logging"
	print "metarr/internal/utils/print"
)

func fillNFOTimestamps(fd *models.FileData) bool {

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

	defer func() {
		print.PrintGrabbedFields("time and date", printMap)
	}()

	if n.Premiered != "" {
		if rtn, ok := dates.YyyyMmDd(n.Premiered); ok && rtn != "" {
			if t.FormattedDate == "" {
				t.FormattedDate = rtn
			}
		}
		printMap[consts.NPremiereDate] = n.Premiered
		gotRelevantDate = true
	}
	if n.ReleaseDate != "" {
		if rtn, ok := dates.YyyyMmDd(n.ReleaseDate); ok && rtn != "" {
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
			t.Creation_Time = fmt.Sprintf("%sT00:00:00Z", t.FormattedDate)
		}
		gotRelevantDate = true
	}

	switch {
	case gotRelevantDate:

		var err error

		logging.D(3, "Got a relevant date, proceeding...")
		if t.FormattedDate == "" {
			dates.FormatAllDates(fd)
		} else {
			t.StringDate, err = dates.ParseNumDate(t.FormattedDate)
			if err != nil {
				logging.E(0, err.Error())
			}
		}

	case w.WebpageURL == "":

		logging.I("Page URL not found in metadata, so cannot scrape for missing date in %q", fd.JSONFilePath)
		return false
	}

	scrapedDate := browser.ScrapeMeta(w, enums.WEBCLASS_DATE)
	logging.D(1, "Scraped date: %s", scrapedDate)

	logging.D(3, "Passed web scrape attempt for date.")

	var (
		date string
		err  error
	)
	if scrapedDate != "" {
		date, err = dates.ParseWordDate(scrapedDate)
		if err != nil || date == "" {
			logging.E(0, "Failed to parse date %q: %v", scrapedDate, err)
			return false
		} else {
			if t.ReleaseDate == "" {
				t.ReleaseDate = date
			}
			if t.Date == "" {
				t.Date = date
			}
			if t.Creation_Time == "" {
				t.Creation_Time = fmt.Sprintf("%sT00:00:00Z", date)
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

			if t.FormattedDate == "" {
				dates.FormatAllDates(fd)
			}
		}
	}

	return true
}
