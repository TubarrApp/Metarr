package utils

import (
	enums "Metarr/internal/domain/enums"
	"Metarr/internal/models"
	"strings"
)

// OdyseeComRules holds rules for scraping odysee.com
var OdyseeComRules = map[enums.WebClassTags][]models.SelectorRule{
	enums.WEBCLASS_CREDITS: {
		{
			Selector: "script[type='application/ld+json']",
			JsonPath: []string{"author", "name"},
			Process:  strings.TrimSpace,
		},
	},
	enums.WEBCLASS_DATE: {
		{
			Selector: "script[type='application/ld+json']",
			JsonPath: []string{"uploadDate"},
			Process:  strings.TrimSpace,
		},
		{Selector: `meta[property="og:video:release_date"]`, Attr: "content", Process: strings.TrimSpace},
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
