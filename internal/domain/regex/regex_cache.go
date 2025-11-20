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
	BracketedNumber           *regexp.Regexp
	DateTagDetect             *regexp.Regexp
	DateTagWithBrackets       *regexp.Regexp
	DoubleSpaces              *regexp.Regexp
	ExtraSpaces               *regexp.Regexp
	InvalidChars              *regexp.Regexp
	SpecialChars              *regexp.Regexp
	ContractionMapSpaced      map[string]models.ContractionPattern
	ContractionMapUnderscored map[string]models.ContractionPattern
	ContractionMapAll         map[string]models.ContractionPattern

	// Initialize sync.Once for each compilation.
	ansiEscapeOnce          sync.Once
	bracketedNumberOnce     sync.Once
	dateTagDetectOnce       sync.Once
	dateTagWithBracketsOnce sync.Once
	doubleSpacesOnce        sync.Once
	extraSpacesOnce         sync.Once
	invalidCharsOnce        sync.Once
	specialCharsOnce        sync.Once
	compileMu               sync.RWMutex
)

// ContractionMapAllCompile compiles a map of all contraction regex patterns.
func ContractionMapAllCompile() map[string]models.ContractionPattern {
	compileMu.Lock()
	defer compileMu.Unlock()

	// Clear old compiled patterns.
	for k := range ContractionMapAll {
		delete(ContractionMapAll, k)
	}

	totalLen := len(consts.ContractionsSpaced) + len(consts.ContractionsUnderscored)
	ContractionMapAll = make(map[string]models.ContractionPattern, totalLen)

	// Spaced map.
	for contraction, replacement := range consts.ContractionsSpaced {
		ContractionMapAll[contraction] = models.ContractionPattern{
			Regexp:      regexp.MustCompile(`\b` + regexp.QuoteMeta(contraction) + `\b`),
			Replacement: replacement,
		}
	}

	// Underscored map.
	for contraction, replacement := range consts.ContractionsUnderscored {
		ContractionMapAll[contraction] = models.ContractionPattern{
			Regexp:      regexp.MustCompile(`\b` + regexp.QuoteMeta(contraction) + `\b`),
			Replacement: replacement,
		}
	}

	return ContractionMapAll
}

// ContractionMapSpacesCompile compiles the regex pattern for spaced contractions and returns
// a model containing the regex and the replacement.
func ContractionMapSpacesCompile() map[string]models.ContractionPattern {
	compileMu.Lock()
	defer compileMu.Unlock()

	// Clear old compiled patterns.
	for k := range ContractionMapSpaced {
		delete(ContractionMapSpaced, k)
	}

	ContractionMapSpaced = make(map[string]models.ContractionPattern, len(consts.ContractionsSpaced))
	for contraction, replacement := range consts.ContractionsSpaced {
		ContractionMapSpaced[contraction] = models.ContractionPattern{
			Regexp:      regexp.MustCompile(`\b` + regexp.QuoteMeta(contraction) + `\b`),
			Replacement: replacement,
		}
	}
	return ContractionMapSpaced
}

// ContractionMapUnderscoresCompile compiles the regex pattern for underscored contractions and returns
// a model containing the regex and the replacement.
func ContractionMapUnderscoresCompile() map[string]models.ContractionPattern {
	compileMu.Lock()
	defer compileMu.Unlock()

	// Clear old compiled patterns.
	for k := range ContractionMapUnderscored {
		delete(ContractionMapUnderscored, k)
	}

	ContractionMapUnderscored = make(map[string]models.ContractionPattern, len(consts.ContractionsUnderscored))
	for contraction, replacement := range consts.ContractionsUnderscored {
		ContractionMapUnderscored[contraction] = models.ContractionPattern{
			Regexp:      regexp.MustCompile(`\b` + regexp.QuoteMeta(contraction) + `\b`),
			Replacement: replacement,
		}
	}
	return ContractionMapUnderscored
}

// AnsiEscapeCompile compiles regex for ANSI escape codes.
func AnsiEscapeCompile() *regexp.Regexp {
	ansiEscapeOnce.Do(func() {
		AnsiEscape = regexp.MustCompile(`\x1b\[[0-9;]*m`)
	})
	return AnsiEscape
}

// BracketedNumberCompile compiles regex for ANSI escape codes.
func BracketedNumberCompile() *regexp.Regexp {
	bracketedNumberOnce.Do(func() {
		BracketedNumber = regexp.MustCompile(`\s*\(\d+\)$`)
	})
	return BracketedNumber
}

// DateTagCompile compiles regex to detect date structures (used in filename date tag stripping).
func DateTagCompile() *regexp.Regexp {
	dateTagDetectOnce.Do(func() {
		DateTagDetect = regexp.MustCompile(`^\d{2,4}-\d{2}-\d{2}$`)
	})
	return DateTagDetect
}

// DateTagWithBracketsCompile compiles regex to find [date] tags anywhere in string.
func DateTagWithBracketsCompile() *regexp.Regexp {
	dateTagWithBracketsOnce.Do(func() {
		DateTagWithBrackets = regexp.MustCompile(`\[\d{2,4}-\d{2}-\d{2}\]`)
	})
	return DateTagWithBrackets
}

// DoubleSpacesCompile compiles regex to detect double spaces.
func DoubleSpacesCompile() *regexp.Regexp {
	doubleSpacesOnce.Do(func() {
		DoubleSpaces = regexp.MustCompile(`\s+`)
	})
	return DoubleSpaces
}

// ExtraSpacesCompile compiles regex for extra spaces.
func ExtraSpacesCompile() *regexp.Regexp {
	extraSpacesOnce.Do(func() {
		ExtraSpaces = regexp.MustCompile(`\s+`)
	})
	return ExtraSpaces
}

// InvalidCharsCompile compiles regex for invalid characters.
func InvalidCharsCompile() *regexp.Regexp {
	invalidCharsOnce.Do(func() {
		InvalidChars = regexp.MustCompile(`[<>:"/\\|?*\x00-\x1F]`)
	})
	return InvalidChars
}

// SpecialCharsCompile compiles regex for special characters.
func SpecialCharsCompile() *regexp.Regexp {
	specialCharsOnce.Do(func() {
		SpecialChars = regexp.MustCompile(`[^\w\s-]`)
	})
	return SpecialChars
}
