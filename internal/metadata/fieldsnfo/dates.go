package fieldsnfo

import (
	"fmt"
	"metarr/internal/dates"
	"metarr/internal/domain/enums"
	"metarr/internal/domain/logger"
	"metarr/internal/models"
	"metarr/internal/utils/browser"
	"metarr/internal/utils/printout"

	"github.com/TubarrApp/gocommon/logging"
	"github.com/TubarrApp/gocommon/sharedtags"
)

// fillNFOTimestamps fills empty date fields from existing date metafields.
func fillNFOTimestamps(fd *models.FileData) (filled bool) {
	t := fd.MDates
	w := fd.MWebData
	n := fd.NFOData

	fieldMap := map[string]*string{
		sharedtags.NAired:        &t.Date,
		sharedtags.NPremiereDate: &t.ReleaseDate,
		sharedtags.NYear:         &t.Year,
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
		printMap[sharedtags.NPremiereDate] = n.Premiered
		gotRelevantDate = true
	}
	if n.ReleaseDate != "" {
		if rtn, ok := dates.YmdFromMeta(n.ReleaseDate); ok && rtn != "" {
			if t.FormattedDate == "" {
				t.FormattedDate = rtn
			}
		}
		printMap[sharedtags.NAired] = n.Premiered
		gotRelevantDate = true
	}
	if n.Year != "" {
		t.Year = n.Year
		printMap[sharedtags.NYear] = n.Year
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

		logger.Pl.D(3, "Got a relevant date, proceeding...")
		if t.FormattedDate == "" {
			dates.FormatAllDates(fd)
		} else {
			t.StringDate, err = dates.ParseNumDate(t.FormattedDate)
			if err != nil {
				logger.Pl.E("Error parsing date %q: %v", t.FormattedDate, err)
			}
		}

	case w.WebpageURL == "":

		logger.Pl.I("Page URL not found in metadata, so cannot scrape for missing date in %q", fd.MetaFilePath)
		return false
	}

	scrapedDate := browser.ScrapeMeta(w, enums.WebclassDate)
	logger.Pl.D(1, "Scraped date: %s", scrapedDate)

	logger.Pl.D(3, "Passed web scrape attempt for date.")

	var (
		date string
		err  error
	)
	if scrapedDate != "" {
		if date, err = dates.ParseWordDate(scrapedDate); err != nil || date == "" {
			logger.Pl.E("Failed to parse date %q: %v", scrapedDate, err)
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

		printMap[sharedtags.NPremiereDate] = t.ReleaseDate
		printMap[sharedtags.NAired] = t.Date
		printMap[sharedtags.NYear] = t.Year

		if t.FormattedDate == "" {
			dates.FormatAllDates(fd)
		}
	}
	return true
}
