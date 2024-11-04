package utils

import (
	enums "Metarr/internal/domain/enums"
	presetModels "Metarr/internal/utils/browser/presets/models"
	logging "Metarr/internal/utils/logging"
	"strconv"
	"strings"
	"time"
)

// BitchuteComRules holds rules for scraping bitchute.com
var BitchuteComRules = map[enums.WebClassTags][]presetModels.SelectorRule{
	enums.WEBCLASS_CREDITS: {

		{Selector: "q-item__label ellipsis text-subtitle1 ellipsis", Process: strings.TrimSpace},
	},
	enums.WEBCLASS_DATE: {
		{
			Selector: "span[data-v-3c3cf957]",
			Attr:     "data-v-3c3cf957",
			Process:  BitchuteComParseDate,
		},
	},
	enums.WEBCLASS_DESCRIPTION: {

		{Selector: `meta[name="description"]`, Attr: "content", Process: strings.TrimSpace},
		{Selector: `meta[property="og:description"]`, Attr: "content", Process: strings.TrimSpace},
		{
			Selector: `meta[itemprop="name"]`,
			Attr:     "content",
			Process:  strings.TrimSpace,
		},
	},
	enums.WEBCLASS_TITLE: {
		{
			Selector: `meta[itemprop="name"]`,
			Attr:     "content",
			Process:  strings.TrimSpace,
		},
	},
}

// BitchuteComParseDate attempts to parse dates like "9 hours ago" (etc.)
func BitchuteComParseDate(date string) string {
	date = strings.TrimSpace(date)

	dateSplit := strings.Split(date, " ")

	var (
		unit  string
		digit int
		err   error
	)

	if len(dateSplit) >= 3 {
		digit, err = strconv.Atoi(dateSplit[0])
		if err != nil {
			logging.PrintE(0, "Failed to convert string to digits: %v", err)
		}
		unit = strings.TrimSuffix(strings.ToLower(dateSplit[1]), "s") // handles both "hour" and "hours"

		var duration time.Duration
		now := time.Now()

		switch unit {
		case "second":
			duration = time.Duration(digit) * time.Second
			return now.Add(-duration).Format(time.RFC3339)
		case "minute":
			duration = time.Duration(digit) * time.Minute
			return now.Add(-duration).Format(time.RFC3339)
		case "hour":
			duration = time.Duration(digit) * time.Hour
			return now.Add(-duration).Format(time.RFC3339)
		case "day":
			duration = time.Duration(digit) * time.Hour * 24
			return now.Add(-duration).Format(time.RFC3339)
		case "week":
			duration = time.Duration(digit) * time.Hour * 24 * 7
			return now.Add(-duration).Format(time.RFC3339)
		case "month":
			return now.AddDate(0, -digit, 0).Format(time.RFC3339)
		case "year":
			return now.AddDate(-digit, 0, 0).Format(time.RFC3339)
		default:
			logging.PrintE(0, "Unknown time unit: %s", unit)
			return ""
		}
	}
	logging.PrintE(0, "Wrong date length passed in")
	return ""
}
