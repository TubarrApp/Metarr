package regex

import (
	consts "Metarr/internal/domain/constants"
	"Metarr/internal/models"
	"regexp"
)

var (
	AnsiEscape                *regexp.Regexp
	ExtraSpaces               *regexp.Regexp
	InvalidChars              *regexp.Regexp
	SpecialChars              *regexp.Regexp
	ContractionMapSpaced      map[string]*models.ContractionPattern
	ContractionMapUnderscored map[string]*models.ContractionPattern
)

func ContractionMapSpacesCompile() map[string]*models.ContractionPattern {
	if ContractionMapSpaced == nil {
		ContractionMapSpaced = make(map[string]*models.ContractionPattern, len(consts.ContractionsSpaced))

		for contraction, replacement := range consts.ContractionsSpaced {

			ContractionMapSpaced[contraction] = &models.ContractionPattern{
				Regexp:      regexp.MustCompile(`\b` + regexp.QuoteMeta(contraction) + `\b`),
				Replacement: replacement,
			}
			ContractionMapSpaced[contraction].Replacement = replacement
		}
	}
	return ContractionMapSpaced
}

func ContractionMapUnderscoresCompile() map[string]*models.ContractionPattern {
	if ContractionMapUnderscored == nil {
		ContractionMapUnderscored = make(map[string]*models.ContractionPattern, len(consts.ContractionsUnderscored))

		for contraction, replacement := range consts.ContractionsUnderscored {

			ContractionMapUnderscored[contraction] = &models.ContractionPattern{
				Regexp:      regexp.MustCompile(`\b` + regexp.QuoteMeta(contraction) + `\b`),
				Replacement: replacement,
			}
			ContractionMapUnderscored[contraction].Replacement = replacement
		}
	}
	return ContractionMapUnderscored
}

func AnsiEscapeCompile() *regexp.Regexp {
	if AnsiEscape == nil {
		AnsiEscape = regexp.MustCompile(`\x1b\[[0-9;]*m`)
	}
	return AnsiEscape
}

func ExtraSpacesCompile() *regexp.Regexp {
	if ExtraSpaces == nil {
		ExtraSpaces = regexp.MustCompile(`\s+`)
	}
	return ExtraSpaces
}

func InvalidCharsCompile() *regexp.Regexp {
	if InvalidChars == nil {
		regexp.MustCompile(`[<>:"/\\|?*\x00-\x1F]`)
	}
	return InvalidChars
}

func SpecialCharsCompile() *regexp.Regexp {
	if SpecialChars == nil {
		SpecialChars = regexp.MustCompile(`[^\w\s-]`)
	}
	return SpecialChars
}
