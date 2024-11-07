package utils

import (
	enums "metarr/internal/domain/enums"
	"metarr/internal/models"
	logging "metarr/internal/utils/logging"
	"strings"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

var CensoredTvRules = map[enums.WebClassTags][]*models.SelectorRule{
	enums.WEBCLASS_DATE: {
		{Selector: ".main-episode-player-container p.text-muted.text-right.text-date.mb-0", Process: strings.TrimSpace},
		{Selector: ".text-date", Process: strings.TrimSpace},
	},
	enums.WEBCLASS_DESCRIPTION: {
		{Selector: ".p-3 check-for-urls", Process: strings.TrimSpace},
		{Selector: `meta[name="description"]`, Attr: "content", Process: strings.TrimSpace},
	},
	enums.WEBCLASS_TITLE: {
		{Selector: ".p-3 h4", Process: strings.TrimSpace},
		{Selector: "[title]", Attr: "title", Process: strings.TrimSpace},
	},
}

// censoredTvChannelName gets the channel name from the URL string
func CensoredTvChannelName(url string) string {
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
