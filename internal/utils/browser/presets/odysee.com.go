package utils

import (
	enums "Metarr/internal/domain/enums"
	presetModels "Metarr/internal/utils/browser/presets/models"
	"strings"
)

// PLACEHOLDER
var OdyseeComRules = map[enums.WebClassTags][]presetModels.SelectorRule{
	enums.WEBCLASS_CREDITS: {
		{
			Selector: "script[type='application/ld+json']",
			JsonPath: []string{"author", "name"},
			Process:  strings.TrimSpace},
	},
}
