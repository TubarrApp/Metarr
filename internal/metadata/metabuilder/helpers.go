package metabuilder

import (
	"encoding/xml"
	"strings"

	"metarr/internal/domain/logger"
)

// indentNFO indents a string by the specified number of levels.
func indentNFO(s string, indentLevel int) (o string) {
	for range indentLevel {
		o += "  "
	}
	return o + s
}

// escapeNFO escapes a value for safe inclusion in NFO (XML) character data.
func escapeNFO(s string) string {
	var b strings.Builder
	if err := xml.EscapeText(&b, []byte(s)); err != nil {
		logger.Pl.E("Failed to escape value %q for NFO output: %v", s, err)
		return ""
	}
	return b.String()
}

// makeUniqueArray removes duplicates and empty entries from a slice of strings,
// preserving the order of first occurrences.
func makeUniqueArray(input []string, extra ...string) []string {
	// Create a new slice to hold unique values.
	o := make([]string, 0, len(input)+len(extra))

	// Create a map to track seen values.
	// Iterate the slices separately to avoid appending into the caller's backing array.
	seen := make(map[string]struct{}, len(input)+len(extra))
	for _, list := range [][]string{input, extra} {
		for _, item := range list {
			item = strings.TrimSpace(item)
			if item == "" {
				continue
			}
			if _, ok := seen[item]; !ok {
				seen[item] = struct{}{}
				o = append(o, item)
			}
		}
	}

	// Return the slice with duplicates removed.
	return o
}
