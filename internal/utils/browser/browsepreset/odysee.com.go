package browsepreset

import (
	"metarr/internal/domain/enums"
	"metarr/internal/models"
	"strings"
)

// OdyseeComRules holds rules for scraping odysee.com
var OdyseeComRules = map[enums.WebClassTags][]models.SelectorRule{
	enums.WebclassCredits: {
		{
			Selector: "script[type='application/ld+json']",
			JSONPath: []string{"author", "name"},
			Process:  strings.TrimSpace,
		},
	},
	enums.WebclassDate: {
		{
			Selector: "script[type='application/ld+json']",
			JSONPath: []string{"uploadDate"},
			Process:  strings.TrimSpace,
		},
		{Selector: `meta[property="og:video:release_date"]`, Attr: "content", Process: strings.TrimSpace},
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
