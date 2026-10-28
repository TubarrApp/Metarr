package dates

import (
	"fmt"
	"metarr/internal/domain/logger"
	"metarr/internal/domain/regex"
	"metarr/internal/models"
	"strings"

	"github.com/TubarrApp/gocommon/shareddates"
	"github.com/TubarrApp/gocommon/sharedenums"
	"github.com/TubarrApp/gocommon/sharedtags"
)

// MakeDateTag attempts to create the date tag for files using metafile data.
func MakeDateTag(metadata map[string]any, fd *models.FileData, dateFmt sharedenums.DateFormat) (string, error) {
	if dateFmt == sharedenums.DateFmtSkip {
		logger.Pl.D(1, "Skip set, not making file date tag for %q", fd.OriginalVideoPath)
		return "", nil
	}

	date, found := "", false
	if fd.MDates.FormattedDate == "" {
		if date, found = extractDateFromMetadata(metadata); !found {
			logger.Pl.E("No dates found in JSON file")
			return "", nil
		}
	} else {
		date = fd.MDates.FormattedDate
	}

	// Extract and validate year, month, day.
	year, month, day, err := shareddates.ParseDateComponents(date, dateFmt, logger.Pl)
	if err != nil {
		return "", fmt.Errorf("failed to parse date components: %w", err)
	}

	// Turn into hyphenated string (e.g. 'yyyy-mm-dd').
	dateStr := shareddates.FormatDateString(year, month, day, dateFmt)
	if dateStr == "" {
		logger.Pl.E("Failed to create date string")
		return "", nil
	}

	// Produce [yyyy-mm-dd] type tag.
	dateTag := "[" + dateStr + "]"
	logger.Pl.I("Made date tag %s from file %q", dateTag, fd.MetaFilePath)
	return dateTag, nil
}

// FormatAllDates formats timestamps into a hyphenated form.
func FormatAllDates(fd *models.FileData) (result string) {
	var (
		err      error
		madeDate bool
	)
	d := fd.MDates

	fields := []string{
		d.OriginallyAvailableAt,
		d.ReleaseDate,
		d.Date,
		d.UploadDate,
		d.CreationTime,
	}

	for _, field := range fields {
		if field == "" {
			continue
		}

		logger.Pl.D(2, "Attempting to format %+v", field)

		if result, madeDate, err = shareddates.YmdFromMeta(field, logger.Pl); madeDate {
			if err != nil {
				logger.Pl.E(err.Error())
			}

			d.FormattedDate = result
			logger.Pl.D(2, "Got formatted date %q", result)

			if d.StringDate, err = shareddates.ParseNumDate(d.FormattedDate, logger.Pl); err != nil {
				logger.Pl.E("Failed to parse date %q: %v", d.FormattedDate, err)
			}
			logger.Pl.D(2, "Got string date %q", d.StringDate)
			return result
		}
	}
	logger.Pl.E("Failed to format dates")
	return ""
}

// StripDateTags removes [yy-mm-dd] or [yyyy-mm-dd] tags from a field string.
func StripDateTags(val string, loc sharedenums.DateTagLocation) (dateStrs []string, result string) {
	val = strings.TrimSpace(val)

	switch loc {
	case sharedenums.DateTagLocPrefix:
		openTag := strings.IndexRune(val, '[')
		closeTag := strings.IndexRune(val, ']')

		if openTag == 0 && closeTag > openTag {
			dateStr := val[openTag+1 : closeTag]
			if regex.DateTagCompile().MatchString(dateStr) {
				return []string{dateStr}, strings.TrimLeft(val[closeTag+1:], " ")
			}
		}
	case sharedenums.DateTagLocSuffix:
		openTag := strings.LastIndex(val, "[")
		closeTag := strings.LastIndex(val, "]")

		if openTag >= 0 && closeTag > openTag && closeTag == len(val)-1 {
			dateStr := val[openTag+1 : closeTag]
			if regex.DateTagCompile().MatchString(dateStr) {
				return []string{dateStr}, strings.TrimSpace(val[:openTag])
			}
		}
	case sharedenums.DateTagLocAll:
		return stripAllDateTags(val)
	}
	return nil, val
}

// AddDateTag puts tag at the start or end of val, replacing a date tag already there.
//
// Reports false if val already has tag there, or for any other location.
func AddDateTag(val, tag string, loc sharedenums.DateTagLocation) (result string, changed bool) {
	switch loc {
	case sharedenums.DateTagLocPrefix:
		if strings.HasPrefix(val, tag) {
			return val, false
		}
		_, rest := StripDateTags(val, loc)
		return tag + " " + rest, true
	case sharedenums.DateTagLocSuffix:
		if strings.HasSuffix(val, tag) {
			return val, false
		}
		_, rest := StripDateTags(val, loc)
		return rest + " " + tag, true
	}
	return val, false
}

// stripAllDateTags removes all date tags and returns them.
func stripAllDateTags(val string) (tags []string, cleaned string) {
	logger.Pl.D(3, "stripAllDateTags input: %q", val)

	pattern := regex.DateTagWithBracketsCompile()
	logger.Pl.D(3, "Pattern: %v", pattern.String())

	// Find all tags.
	tags = pattern.FindAllString(val, -1)
	logger.Pl.D(3, "Found tags: %v (count: %d)", tags, len(tags))

	// Remove all tags.
	cleaned = pattern.ReplaceAllString(val, "")
	logger.Pl.D(3, "After removal: %q", cleaned)

	// Clean up extra spaces.
	cleaned = strings.TrimSpace(cleaned)
	cleaned = regex.DoubleSpacesCompile().ReplaceAllString(cleaned, " ")
	logger.Pl.D(3, "Final cleaned: %q", cleaned)

	return tags, cleaned
}

// extractDateFromMetadata attempts to find a date in the metadata using predefined fields.
func extractDateFromMetadata(metadata map[string]any) (string, bool) {
	for _, field := range sharedtags.DateKeys {
		if value, found := metadata[field]; found {
			if strVal, ok := value.(string); ok && strVal != "" && len(strVal) > 4 {
				if date, _, found := strings.Cut(strVal, "T"); found {
					return date, true
				}
				return strVal, true
			}
		}
	}
	return "", false
}
