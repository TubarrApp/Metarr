package browsepreset

import (
	"metarr/internal/domain/enums"
	"metarr/internal/models"
	"strings"
)

// RumbleComRules holds rules for scraping rumble.com
var RumbleComRules = map[enums.WebClassTags][]models.SelectorRule{
	enums.WebclassCredits: {

		{Selector: ".media-subscribe-and-notify", Attr: "data-title", Process: strings.TrimSpace},
		{Selector: ".media-by--a .media-heading-name", Process: strings.TrimSpace},
	},
	enums.WebclassDate: {
		{Selector: "time", Attr: "datetime", Process: strings.TrimSpace},
		{
			Selector: "script[type='application/ld+json']",
			JSONPath: []string{"uploadDate"},
			Process:  strings.TrimSpace,
		},
	},
	enums.WebclassDescription: {
		{
			Selector: "script[type='application/ld+json']",
			JSONPath: []string{"description"},
			Process:  strings.TrimSpace,
		},
		{Selector: `meta[name="description"]`, Attr: "content", Process: strings.TrimSpace},
		{Selector: `meta[property="og:description"]`, Attr: "content", Process: strings.TrimSpace},
	},
	enums.WebclassTitle: {

		{Selector: "title", Process: strings.TrimSpace},
		{
			Selector: "script[type='application/ld+json']",
			JSONPath: []string{"name"},
			Process:  strings.TrimSpace,
		},
	},
}
