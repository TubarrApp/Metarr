package fieldsnfo

import (
	"cmp"
	"fmt"
	"metarr/internal/dates"
	"metarr/internal/domain/enums"
	"metarr/internal/domain/logger"
	"metarr/internal/models"
	"metarr/internal/utils/printout"

	"github.com/TubarrApp/gocommon/logging"
	"github.com/TubarrApp/gocommon/shareddates"
	"github.com/TubarrApp/gocommon/sharedtags"
)

// FillNFOTimestamps fills empty date fields from existing date metafields.
func FillNFOTimestamps(fd *models.FileData) (filled bool) {
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
		if rtn, ok := shareddates.YmdFromMeta(n.Premiered, logger.Pl); ok && rtn != "" {
			if t.FormattedDate == "" {
				t.FormattedDate = rtn
			}
		}
		printMap[sharedtags.NPremiereDate] = n.Premiered
		gotRelevantDate = true
	}
	if releaseDate := cmp.Or(n.ReleaseDate, n.Released); releaseDate != "" {
		if rtn, ok := shareddates.YmdFromMeta(releaseDate, logger.Pl); ok && rtn != "" {
			if t.FormattedDate == "" {
				t.FormattedDate = rtn
			}
		}
		printMap[sharedtags.NAired] = releaseDate
		gotRelevantDate = true
	}
	if n.Year != "" {
		t.Year = n.Year
		printMap[sharedtags.NYear] = n.Year
	}

	// Fields the NFO holds already carry the override; fill those it cannot hold.
	if v, ok := fd.MetaOps.SetOverrides[enums.OverrideMetaDates]; ok {
		fillEmpty(v, &t.FormattedDate, &t.UploadDate, &t.OriginallyAvailableAt)
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

	// Take the year from the date field most likely to be right for the video.
	if year := t.VideoYear(""); year != "" {
		t.Year = year
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
		if t.StringDate, err = shareddates.ParseNumDate(t.FormattedDate, logger.Pl); err != nil {
			logger.Pl.E("Error parsing date %q: %v", t.FormattedDate, err)
		}
	}
	return true
}
