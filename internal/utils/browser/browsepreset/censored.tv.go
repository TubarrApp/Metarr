// Package browsepreset holds preset tags and data useful for web scraping operations.
package browsepreset

import (
	"metarr/internal/domain/enums"
	"metarr/internal/models"
	"metarr/internal/utils/logging"
	"strings"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

var CensoredTvRules = map[enums.WebClassTags][]models.SelectorRule{
	enums.WebclassDate: {
		{Selector: ".main-episode-player-container p.text-muted.text-right.text-date.mb-0", Process: strings.TrimSpace},
		{Selector: ".text-date", Process: strings.TrimSpace},
	},
	enums.WebclassDescription: {
		{Selector: ".p-3 check-for-urls", Process: strings.TrimSpace},
		{Selector: `meta[name="description"]`, Attr: "content", Process: strings.TrimSpace},
	},
	enums.WebclassTitle: {
		{Selector: ".p-3 h4", Process: strings.TrimSpace},
		{Selector: "[title]", Attr: "title", Process: strings.TrimSpace},
	},
}

// CensoredTvChannelName gets the channel name from the URL string
func CensoredTvChannelName(url string) string {
	if url == "" {
		logging.E(0, "url passed in empty")
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
		logging.E(0, "failed to fill channel name from url, out of bounds?")
	}
	channel = strings.ReplaceAll(channel, "-", " ")

	caser := cases.Title(language.English)
	channel = caser.String(channel)

	if strings.EqualFold(channel, "atheism is unstoppable") {
		channel = "Atheism-is-Unstoppable"
	}
	return channel
}
