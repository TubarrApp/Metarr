package utils

import (
	enums "Metarr/internal/domain/enums"
	"strings"
)

// PLACEHOLDER
var odyseeComRules = map[enums.WebClassTags][]selectorRule{
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
	enums.WEBCLASS_CREDITS: {
		{
			selector: "script[type='application/ld+json']",
			jsonPath: []string{"author", "name"},
			process:  strings.TrimSpace},
	},
}
