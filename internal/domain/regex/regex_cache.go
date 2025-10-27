// Package regex handles and caches regex directives.
package regex

import (
	"metarr/internal/domain/consts"
	"metarr/internal/models"
	"regexp"
	"sync"
)

// Regex cache.
var (
	AnsiEscape                *regexp.Regexp
	DateTagDetect             *regexp.Regexp
	DateTagWithBrackets       *regexp.Regexp
	DoubleSpaces              *regexp.Regexp
	ExtraSpaces               *regexp.Regexp
	InvalidChars              *regexp.Regexp
	SpecialChars              *regexp.Regexp
	ContractionMapSpaced      map[string]models.ContractionPattern
	ContractionMapUnderscored map[string]models.ContractionPattern
	ContractionMapAll         map[string]models.ContractionPattern

	// Initialize sync.Once for each compilation
	ansiEscapeOnce              sync.Once
	dateTagDetectOnce           sync.Once
	dateTagWithBracketsOnce     sync.Once
	doubleSpacesOnce            sync.Once
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

// DateTagCompile compiles regex to detect date structures (used in filename date tag stripping)
func DateTagCompile() *regexp.Regexp {
	dateTagDetectOnce.Do(func() {
		contractionMu.Lock()
		defer contractionMu.Unlock()

		DateTagDetect = regexp.MustCompile(`^\d{2,4}-\d{2}-\d{2}$`)
	})
	return DateTagDetect
}

// DateTagWithBracketsCompile compiles regex to find [date] tags anywhere in string
func DateTagWithBracketsCompile() *regexp.Regexp {
	dateTagWithBracketsOnce.Do(func() {
		contractionMu.Lock()
		defer contractionMu.Unlock()
		DateTagWithBrackets = regexp.MustCompile(`\[\d{2,4}-\d{2}-\d{2}\]`)
	})
	return DateTagWithBrackets
}

// DoubleSpacesCompils compiles regex to detect double spaces
func DoubleSpacesCompile() *regexp.Regexp {
	doubleSpacesOnce.Do(func() {
		contractionMu.Lock()
		defer contractionMu.Unlock()
		DoubleSpaces = regexp.MustCompile(`\s+`)
	})
	return DoubleSpaces
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
