package utils

import (
	enums "Metarr/internal/domain/enums"
	presetModels "Metarr/internal/utils/browser/presets/models"
	"strings"
)

// RumbleComRules holds rules for scraping rumble.com
var RumbleComRules = map[enums.WebClassTags][]presetModels.SelectorRule{
	enums.WEBCLASS_CREDITS: {

		{Selector: ".media-subscribe-and-notify", Attr: "data-title", Process: strings.TrimSpace},
		{Selector: ".media-by--a .media-heading-name", Process: strings.TrimSpace},
	},
	enums.WEBCLASS_DATE: {
		{Selector: "time", Attr: "datetime", Process: strings.TrimSpace},
		{
			Selector: "script[type='application/ld+json']",
			JsonPath: []string{"uploadDate"},
			Process:  strings.TrimSpace,
		},
	},
	enums.WEBCLASS_DESCRIPTION: {
		{
			Selector: "script[type='application/ld+json']",
			JsonPath: []string{"description"},
			Process:  strings.TrimSpace,
		},
		{Selector: `meta[name="description"]`, Attr: "content", Process: strings.TrimSpace},
		{Selector: `meta[property="og:description"]`, Attr: "content", Process: strings.TrimSpace},
	},
	enums.WEBCLASS_TITLE: {

		{Selector: "title", Process: strings.TrimSpace},
		{
			Selector: "script[type='application/ld+json']",
			JsonPath: []string{"name"},
			Process:  strings.TrimSpace,
		},
	},
}
