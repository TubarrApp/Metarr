package utils

import (
	enums "Metarr/internal/domain/enums"
	logging "Metarr/internal/utils/logging"
	"strings"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

var censoredTvRules = map[enums.WebClassTags][]selectorRule{
	enums.WEBCLASS_TITLE: {
		{selector: ".p-3 h4", process: strings.TrimSpace},
		{selector: "[title]", attr: "title", process: strings.TrimSpace},
	},
	enums.WEBCLASS_DESCRIPTION: {
		{selector: ".p-3 check-for-urls", process: strings.TrimSpace},
		{selector: `meta[name="description"]`, attr: "content", process: strings.TrimSpace},
	},
	enums.WEBCLASS_DATE: {
		{selector: "p.text-muted.text-right.text-date.mb-0", process: strings.TrimSpace},
		{selector: ".text-date", process: strings.TrimSpace},
	},
}

// censoredTvChannelName gets the channel name from the URL string
func censoredTvChannelName(url string) string {
	if url == "" {
		logging.PrintE(0, "url passed in empty")
		return ""
	}
	urlSplit := strings.Split(url, "/")

	var channel string
	for i, seg := range urlSplit {
		if strings.HasSuffix(seg, "shows") && len(urlSplit) > i+1 {
			channel = urlSplit[i+1]
		}
	}

	if channel == "" {
		logging.PrintE(0, "failed to fill channel name from url, out of bounds?")
	}
	channel = strings.ReplaceAll(channel, "-", " ")

	caser := cases.Title(language.English)
	channel = caser.String(channel)

	if strings.ToLower(channel) == "atheism is unstoppable" {
		channel = "Atheism-is-Unstoppable"
	}
	return channel
}
