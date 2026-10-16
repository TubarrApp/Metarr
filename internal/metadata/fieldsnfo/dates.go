package fieldsnfo

import (
	"fmt"
	"metarr/internal/dates"
	"metarr/internal/domain/logger"
	"metarr/internal/models"
	"metarr/internal/utils/printout"

	"github.com/TubarrApp/gocommon/logging"
	"github.com/TubarrApp/gocommon/sharedtags"
)

// fillNFOTimestamps fills empty date fields from existing date metafields.
func fillNFOTimestamps(fd *models.FileData) (filled bool) {
	t := fd.MDates
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
		if logging.Level() > 0 && len(printMap) > 0 {
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

	if !gotRelevantDate {
		logger.Pl.I("No date found in metadata %q", fd.MetaFilePath)
		return false
	}
	logger.Pl.D(3, "Got a relevant date, proceeding...")

	if t.FormattedDate == "" {
		dates.FormatAllDates(fd)
	} else {
		var err error
		if t.StringDate, err = dates.ParseNumDate(t.FormattedDate); err != nil {
			logger.Pl.E("Error parsing date %q: %v", t.FormattedDate, err)
		}
	}
	return true
}
