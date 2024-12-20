// Package regex handles and caches regex directives.
package regex

import (
	"metarr/internal/domain/consts"
	"metarr/internal/models"
	"regexp"
	"sync"
)

var (
	AnsiEscape                *regexp.Regexp
	ExtraSpaces               *regexp.Regexp
	InvalidChars              *regexp.Regexp
	SpecialChars              *regexp.Regexp
	ContractionMapSpaced      map[string]models.ContractionPattern
	ContractionMapUnderscored map[string]models.ContractionPattern
	ContractionMapAll         map[string]models.ContractionPattern

	// Initialize sync.Once for each compilation
	ansiEscapeOnce              sync.Once
	extraSpacesOnce             sync.Once
	invalidCharsOnce            sync.Once
	specialCharsOnce            sync.Once
	contractionsSpacedOnce      sync.Once
	contractionsUnderscoredOnce sync.Once
	contractionsAllOnce         sync.Once
	contractionMu               sync.RWMutex
)

// ContractionMapAllCompile compiles the regex pattern for spaced AND underscored contractions and returns
// a model containing the regex and the replacement
func ContractionMapAllCompile() map[string]models.ContractionPattern {
	contractionsAllOnce.Do(func() {
		contractionMu.Lock()
		defer contractionMu.Unlock()

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
	})
	return ContractionMapAll
}

// ContractionMapSpacesCompile compiles the regex pattern for spaced contractions and returns
// a model containing the regex and the replacement
func ContractionMapSpacesCompile() map[string]models.ContractionPattern {
	contractionsSpacedOnce.Do(func() {
		contractionMu.Lock()
		defer contractionMu.Unlock()

		ContractionMapSpaced = make(map[string]models.ContractionPattern, len(consts.ContractionsSpaced))
		for contraction, replacement := range consts.ContractionsSpaced {
			ContractionMapSpaced[contraction] = models.ContractionPattern{
				Regexp:      regexp.MustCompile(`\b` + regexp.QuoteMeta(contraction) + `\b`),
				Replacement: replacement,
			}
		}
	})
	return ContractionMapSpaced
}

// ContractionMapUnderscoresCompile compiles the regex pattern for underscored contractions and returns
// a model containing the regex and the replacement
func ContractionMapUnderscoresCompile() map[string]models.ContractionPattern {
	contractionsUnderscoredOnce.Do(func() {
		contractionMu.Lock()
		defer contractionMu.Unlock()

		ContractionMapUnderscored = make(map[string]models.ContractionPattern, len(consts.ContractionsUnderscored))
		for contraction, replacement := range consts.ContractionsUnderscored {
			ContractionMapUnderscored[contraction] = models.ContractionPattern{
				Regexp:      regexp.MustCompile(`\b` + regexp.QuoteMeta(contraction) + `\b`),
				Replacement: replacement,
			}
		}
	})
	return ContractionMapUnderscored
}

// AnsiEscapeCompile compiles regex for ANSI escape codes
func AnsiEscapeCompile() *regexp.Regexp {
	ansiEscapeOnce.Do(func() {
		contractionMu.Lock()
		defer contractionMu.Unlock()

		AnsiEscape = regexp.MustCompile(`\x1b\[[0-9;]*m`)
	})
	return AnsiEscape
}

// ExtraSpacesCompile compiles regex for extra spaces
func ExtraSpacesCompile() *regexp.Regexp {
	extraSpacesOnce.Do(func() {
		contractionMu.Lock()
		defer contractionMu.Unlock()

		ExtraSpaces = regexp.MustCompile(`\s+`)
	})
	return ExtraSpaces
}

// InvalidCharsCompile compiles regex for invalid characters
func InvalidCharsCompile() *regexp.Regexp {
	invalidCharsOnce.Do(func() {
		contractionMu.Lock()
		defer contractionMu.Unlock()

		InvalidChars = regexp.MustCompile(`[<>:"/\\|?*\x00-\x1F]`)
	})
	return InvalidChars
}

// SpecialCharsCompile compiles regex for special characters
func SpecialCharsCompile() *regexp.Regexp {
	specialCharsOnce.Do(func() {
		contractionMu.Lock()
		defer contractionMu.Unlock()

		SpecialChars = regexp.MustCompile(`[^\w\s-]`)
	})
	return SpecialChars
}
