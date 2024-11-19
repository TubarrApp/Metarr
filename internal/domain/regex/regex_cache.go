package regex

import (
	"metarr/internal/domain/consts"
	"metarr/internal/models"
	"regexp"
)

var (
	AnsiEscape   *regexp.Regexp
	ExtraSpaces  *regexp.Regexp
	InvalidChars *regexp.Regexp
	SpecialChars *regexp.Regexp

	ContractionMapSpaced      map[string]models.ContractionPattern
	ContractionMapUnderscored map[string]models.ContractionPattern
	ContractionMapAll         map[string]models.ContractionPattern
)

// ContractionMapAllCompile compiles the regex pattern for spaced AND underscored contractions and returns
// a model containing the regex and the replacement
func ContractionMapAllCompile() map[string]models.ContractionPattern {
	if ContractionMapAll == nil {
		ContractionMapAll = make(map[string]models.ContractionPattern, len(consts.ContractionsSpaced)+len(consts.ContractionsUnderscored))

		// Spaced map
		for contraction, replacement := range consts.ContractionsSpaced {

			ContractionMapAll[contraction] = models.ContractionPattern{
				Regexp:      regexp.MustCompile(`\b` + regexp.QuoteMeta(contraction) + `\b`),
				Replacement: replacement,
			}
		}

		// Underscored map
		for contraction, replacement := range consts.ContractionsUnderscored {

			ContractionMapAll[contraction] = models.ContractionPattern{
				Regexp:      regexp.MustCompile(`\b` + regexp.QuoteMeta(contraction) + `\b`),
				Replacement: replacement,
			}
		}
	}

	return ContractionMapAll
}

// ContractionMapSpacesCompile compiles the regex pattern for spaced contractions and returns
// a model containing the regex and the replacement
func ContractionMapSpacesCompile() map[string]models.ContractionPattern {
	if ContractionMapSpaced == nil {
		ContractionMapSpaced = make(map[string]models.ContractionPattern, len(consts.ContractionsSpaced))

		for contraction, replacement := range consts.ContractionsSpaced {

			ContractionMapSpaced[contraction] = models.ContractionPattern{
				Regexp:      regexp.MustCompile(`\b` + regexp.QuoteMeta(contraction) + `\b`),
				Replacement: replacement,
			}
		}
	}
	return ContractionMapSpaced
}

// ContractionMapUnderscoresCompile compiles the regex pattern for underscored contractions and returns
// a model containing the regex and the replacement
func ContractionMapUnderscoresCompile() map[string]models.ContractionPattern {
	if ContractionMapUnderscored == nil {
		ContractionMapUnderscored = make(map[string]models.ContractionPattern, len(consts.ContractionsUnderscored))

		for contraction, replacement := range consts.ContractionsUnderscored {

			ContractionMapUnderscored[contraction] = models.ContractionPattern{
				Regexp:      regexp.MustCompile(`\b` + regexp.QuoteMeta(contraction) + `\b`),
				Replacement: replacement,
			}
		}
	}
	return ContractionMapUnderscored
}

// AnsiEscapeCompile compiles regex for ANSI escape codes
func AnsiEscapeCompile() *regexp.Regexp {
	if AnsiEscape == nil {
		AnsiEscape = regexp.MustCompile(`\x1b\[[0-9;]*m`)
	}
	return AnsiEscape
}

// ExtraSpacesCompile compiles regex for extra spaces
func ExtraSpacesCompile() *regexp.Regexp {
	if ExtraSpaces == nil {
		ExtraSpaces = regexp.MustCompile(`\s+`)
	}
	return ExtraSpaces
}

// InvalidCharsCompile compiles regex for invalid characters
func InvalidCharsCompile() *regexp.Regexp {
	if InvalidChars == nil {
		InvalidChars = regexp.MustCompile(`[<>:"/\\|?*\x00-\x1F]`)
	}
	return InvalidChars
}

// SpecialCharsCompile compiles regex for special characters
func SpecialCharsCompile() *regexp.Regexp {
	if SpecialChars == nil {
		SpecialChars = regexp.MustCompile(`[^\w\s-]`)
	}
	return SpecialChars
}
