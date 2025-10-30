package nfofields

import (
	"fmt"
	"metarr/internal/dates"
	"metarr/internal/domain/consts"
	"metarr/internal/domain/enums"
	"metarr/internal/models"
	"metarr/internal/utils/browser"
	"metarr/internal/utils/logging"
	"metarr/internal/utils/printout"
)

// fillNFOTimestamps fills empty date fields from existing date metafields.
func fillNFOTimestamps(fd *models.FileData) (filled bool) {
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
		if logging.Level > 0 && len(printMap) > 0 {
			printout.PrintGrabbedFields("time and date", printMap)
		}
	}()

	if n.Premiered != "" {
		if rtn, ok := dates.YmdFromMeta(n.Premiered); ok && rtn != "" {
			if t.FormattedDate == "" {
				t.FormattedDate = rtn
			}
		}
		printMap[consts.NPremiereDate] = n.Premiered
		gotRelevantDate = true
	}
	if n.ReleaseDate != "" {
		if rtn, ok := dates.YmdFromMeta(n.ReleaseDate); ok && rtn != "" {
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
		if t.CreationTime == "" {
			t.CreationTime = fmt.Sprintf("%sT00:00:00Z", t.FormattedDate)
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
				logging.E("Error parsing date %q: %v", t.FormattedDate, err)
			}
		}

	case w.WebpageURL == "":

		logging.I("Page URL not found in metadata, so cannot scrape for missing date in %q", fd.MetaFilePath)
		return false
	}

	scrapedDate := browser.ScrapeMeta(w, enums.WebclassDate)
	logging.D(1, "Scraped date: %s", scrapedDate)

	logging.D(3, "Passed web scrape attempt for date.")

	var (
		date string
		err  error
	)
	if scrapedDate != "" {
		if date, err = dates.ParseWordDate(scrapedDate); err != nil || date == "" {
			logging.E("Failed to parse date %q: %v", scrapedDate, err)
			return false
		}

		if t.ReleaseDate == "" {
			t.ReleaseDate = date
		}
		if t.Date == "" {
			t.Date = date
		}
		if t.CreationTime == "" {
			t.CreationTime = fmt.Sprintf("%sT00:00:00Z", date)
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

		printMap[consts.NPremiereDate] = t.ReleaseDate
		printMap[consts.NAired] = t.Date
		printMap[consts.NYear] = t.Year

		if t.FormattedDate == "" {
			dates.FormatAllDates(fd)
		}
	}

	return true
}
