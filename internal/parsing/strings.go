package parsing

import "strings"

// EscapedSplit allows users to escape separator characters without messing up 'strings.Split' logic.
func EscapedSplit(s string, desiredSeparator rune) []string {
	var parts []string
	var buf strings.Builder
	escaped := false

	for _, r := range s {
		switch {
		case escaped:
			// Always take the next character literally.
			buf.WriteRune(r)
			escaped = false
		case r == '\\':
			// Escape next character.
			escaped = true
		case r == desiredSeparator:
			// Separator.
			parts = append(parts, buf.String())
			buf.Reset()
		default:
			buf.WriteRune(r)
		}
	}
	if escaped {
		// Trailing '\' treated as literal backslash.
		buf.WriteRune('\\')
	}

	// Add last segment.
	parts = append(parts, buf.String())
	return parts
}

// UnescapeSplit reverts string elements back to unescaped versions.
func UnescapeSplit(s string, separatorUsed string) string {
	return strings.ReplaceAll(s, `\`+separatorUsed, separatorUsed)
}
