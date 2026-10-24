package fieldsjson

import (
	"metarr/internal/dates"
	"metarr/internal/domain/consts"
	"metarr/internal/domain/logger"
	"metarr/internal/metadata/metawriters"
	"metarr/internal/models"
	"metarr/internal/utils/printout"
	"strings"

	"github.com/TubarrApp/gocommon/logging"
	"github.com/TubarrApp/gocommon/sharedtags"
)

// FillTimestamps grabs timestamp metadata from JSON.
func FillTimestamps(fd *models.FileData, json map[string]any, jsonRW *metawriters.JSONFileRW) bool {
	t := fd.MDates

	fieldMap := map[string]*string{ // Date fields by JSON key.
		sharedtags.JReleaseDate:         &t.ReleaseDate,
		sharedtags.JOriginallyAvailable: &t.OriginallyAvailableAt,
		sharedtags.JDate:                &t.Date,
		sharedtags.JUploadDate:          &t.UploadDate,
		sharedtags.JYear:                &t.Year,
		sharedtags.JCreationTime:        &t.CreationTime,
	}

	if ok := unpackJSON(fieldMap, json); !ok {
		logger.Pl.E("Failed to unpack date JSON, no dates currently exist in file?")
	}

	printMap := make(map[string]string, len(fieldMap))
	if logging.Level() > 1 {
		defer func() {
			if len(printMap) > 0 {
				printout.PrintGrabbedFields("dates", printMap)
			}
		}()
	}

	var gotDate bool
	for k, ptr := range fieldMap {
		if ptr == nil {
			logger.Pl.E("Unexpected nil pointer in fieldMap")
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
		if logging.Level() > 1 {
			printMap[k] = finalVal
		}
		gotDate = true
	}

	// Take the year from the date field most likely to be right for the video.
	if year := t.VideoYear(jsonText(json, sharedtags.JReleaseYear)); year != "" {
		t.Year = year
		gotDate = true
	}

	// Note whether the originally available date came from the file, before one is worked out.
	originallyAvailableInFile := t.OriginallyAvailableAt != ""

	var b strings.Builder
	b.Grow(len(consts.TimeSfx) + 10)

	if fillEmptyTimestamps(t, &b) {
		gotDate = true
	}

	if !gotDate {
		logger.Pl.I("No date found in metadata %q", fd.MetaFilePath)
		return false
	}
	logger.Pl.D(3, "Got a relevant date, proceeding...")

	var err error
	if t.FormattedDate == "" {
		dates.FormatAllDates(fd)
	} else if t.StringDate, err = dates.ParseNumDate(t.FormattedDate); err != nil {
		logger.Pl.E("Failed to parse date %q: %v", t.FormattedDate, err)
	}

	// Only save an originally available date that was in the file, as a worked-out one would pass for a release
	// date on the next run.
	if !originallyAvailableInFile {
		delete(fieldMap, sharedtags.JOriginallyAvailable)
	}

	if _, err := jsonRW.WriteJSON(fieldMap); err != nil {
		logger.Pl.E("Failed to write into JSON file %q: %v", fd.MetaFilePath, err)
	}
	return true
}

// fillEmptyTimestamps attempts to infer missing timestamps.
func fillEmptyTimestamps(t *models.MetadataDates, b *strings.Builder) bool {
	gotDate := false

	// Infer from release date.
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

	// Infer from originally available date.
	if t.OriginallyAvailableAt != "" && len(t.OriginallyAvailableAt) >= 6 {
		gotDate = true

		if !strings.ContainsRune(t.CreationTime, 'T') {
			processDateField(t.OriginallyAvailableAt, &t.CreationTime, t)
			t.CreationTime = formatTimeStamp(t.CreationTime, b)
		}
	}

	// Infer from date.
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

	// Infer from upload date.
	if t.UploadDate != "" && len(t.UploadDate) >= 6 {

		if !strings.ContainsRune(t.CreationTime, 'T') {
			processDateField(t.UploadDate, &t.CreationTime, t)
			t.CreationTime = formatTimeStamp(t.CreationTime, b)
		}

		if t.OriginallyAvailableAt == "" {
			processDateField(t.UploadDate, &t.OriginallyAvailableAt, t)
		}
	}

	// Fill empty date.
	if t.Date == "" {
		switch {
		case t.ReleaseDate != "":
			t.Date = t.ReleaseDate
			if t.OriginallyAvailableAt == "" {
				t.OriginallyAvailableAt = t.ReleaseDate
			}

		case t.UploadDate != "":
			t.Date = t.UploadDate
			if t.OriginallyAvailableAt == "" {
				t.OriginallyAvailableAt = t.UploadDate
			}

		case t.FormattedDate != "":
			t.Date = t.FormattedDate
		}
	}
	// Fill empty year.
	if t.Year == "" {
		switch {
		case t.ReleaseDate != "" && len(t.ReleaseDate) >= 4:
			t.Year = t.ReleaseDate[:4]

		case t.OriginallyAvailableAt != "" && len(t.OriginallyAvailableAt) >= 4:
			t.Year = t.OriginallyAvailableAt[:4]

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

	// Try to fix accidentally using upload date if another date is available.
	if len(t.Year) == 4 && !strings.HasPrefix(t.CreationTime, t.Year) && len(t.CreationTime) >= 4 {

		logger.Pl.D(1, "Creation time does not match year tag, seeing if other dates are available...")

		switch {
		case strings.HasPrefix(t.OriginallyAvailableAt, t.Year):
			if !strings.ContainsRune(t.CreationTime, 'T') {
				t.CreationTime = formatTimeStamp(t.OriginallyAvailableAt, b)
			}

			logger.Pl.D(1, "Set creation time to %s", t.OriginallyAvailableAt)

		case strings.HasPrefix(t.ReleaseDate, t.Year):
			if !strings.ContainsRune(t.CreationTime, 'T') {
				t.CreationTime = formatTimeStamp(t.ReleaseDate, b)
			}

			logger.Pl.D(1, "Set creation time to %s", t.ReleaseDate)

		case strings.HasPrefix(t.Date, t.Year):
			if !strings.ContainsRune(t.CreationTime, 'T') {
				t.CreationTime = formatTimeStamp(t.Date, b)
			}

			logger.Pl.D(1, "Set creation time to %s", t.Date)

		case strings.HasPrefix(t.FormattedDate, t.Year):
			if !strings.ContainsRune(t.CreationTime, 'T') {
				t.CreationTime = formatTimeStamp(t.FormattedDate, b)
			}

			logger.Pl.D(1, "Set creation time to %s", t.FormattedDate)

		default:
			logger.Pl.D(1, "No full date is from %s, keeping the creation time %q", t.Year, t.CreationTime)
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
