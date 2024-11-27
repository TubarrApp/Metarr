// Package validation can validate things like file extension correctness.
package validation

import "strings"

// ValidateExtension checks if the output extension is valid
func ValidateExtension(ext string) string {
	ext = strings.TrimSpace(ext)

	// Handle empty or invalid cases
	if ext == "" || ext == "." {
		return ""
	}

	// Ensure proper dot prefix
	if !strings.HasPrefix(ext, ".") {
		ext = "." + ext
	}

	// Verify the extension is not just a lone dot
	if len(ext) <= 1 {
		return ""
	}

	return ext
}
