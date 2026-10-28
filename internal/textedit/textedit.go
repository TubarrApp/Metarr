// Package textedit holds the text edits of filename and metadata operations, so both edit text the same way.
package textedit

import (
	"metarr/internal/dates"
	"strings"
	"unicode"

	"github.com/TubarrApp/gocommon/sharedenums"
)

// AddPrefixes puts prefixes at the start of val, after any date tag there, so they read in the order given.
//
// A prefix already in its place is left as it is, so running the same prefixes again changes nothing.
func AddPrefixes(val string, prefixes []string) (result string, changed bool) {
	i := prefixStart(val)
	for _, p := range prefixes {
		if !strings.HasPrefix(val[i:], p) {
			val = val[:i] + p + val[i:]
			changed = true
		}
		i += len(p)
	}
	return val, changed
}

// AddSuffixes puts suffixes at the end of val, before any date tag there, so they read in the order given.
//
// A suffix already in its place is left as it is, so running the same suffixes again changes nothing.
func AddSuffixes(val string, suffixes []string) (result string, changed bool) {
	j := suffixEnd(val)
	for k := len(suffixes) - 1; k >= 0; k-- {
		s := suffixes[k]
		if strings.HasSuffix(val[:j], s) {
			j -= len(s)
			continue
		}
		val = val[:j] + s + val[j:]
		changed = true
	}
	return val, changed
}

// ReplacePrefix swaps prefix at the start of val for replacement, reporting false if val doesn't start with it.
func ReplacePrefix(val, prefix, replacement string) (result string, changed bool) {
	after, found := strings.CutPrefix(val, prefix)
	if !found {
		return val, false
	}
	return replacement + after, true
}

// ReplaceSuffix swaps suffix at the end of val for replacement, reporting false if val doesn't end with it.
func ReplaceSuffix(val, suffix, replacement string) (result string, changed bool) {
	before, found := strings.CutSuffix(val, suffix)
	if !found {
		return val, false
	}
	return before + replacement, true
}

// Cut keeps the text after the first delim, or with keepBefore set, the text before it, dropping delim either way.
//
// Reports false if val doesn't contain delim.
func Cut(val, delim string, keepBefore bool) (result string, changed bool) {
	before, after, found := strings.Cut(val, delim)
	if !found {
		return val, false
	}
	if keepBefore {
		return before, true
	}
	return after, true
}

// prefixStart returns where val's text begins after a date tag at its start, or 0 if it has none.
func prefixStart(val string) int {
	tags, rest := dates.StripDateTags(val, sharedenums.DateTagLocPrefix)
	if len(tags) == 0 {
		return 0
	}
	return len(strings.TrimRightFunc(val, unicode.IsSpace)) - len(rest)
}

// suffixEnd returns where val's text ends before a date tag at its end, or len(val) if it has none.
func suffixEnd(val string) int {
	tags, rest := dates.StripDateTags(val, sharedenums.DateTagLocSuffix)
	if len(tags) == 0 {
		return len(val)
	}
	return len(val) - len(strings.TrimLeftFunc(val, unicode.IsSpace)) + len(rest)
}
