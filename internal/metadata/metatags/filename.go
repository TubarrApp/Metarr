package metatags

import (
	"metarr/internal/cfg"
	"metarr/internal/domain/keys"
	"metarr/internal/domain/regex"
	"metarr/internal/utils/logging"
	"os"
	"path/filepath"
	"strings"
)

// MakeFilenameTag creates the metatag string to prefix filenames with.
func MakeFilenameTag(metadata map[string]any, file *os.File) string {
	logging.D(5, "Entering makeFilenameTag with data %v", metadata)

	tagFields := cfg.GetStringSlice(keys.MFilenamePfx)
	if len(tagFields) == 0 {
		return ""
	}

	var b strings.Builder
	b.Grow(len(metadata) + len("[2006-02-01]"))
	b.WriteString("[")

	written := false
	for _, field := range tagFields {

		if value, exists := metadata[field]; exists {
			if strVal, ok := value.(string); ok && strVal != "" {

				if written {
					b.WriteString("_")
				}

				b.WriteString(strVal)
				written = true

				logging.D(3, "Added metafield %v to prefix tag (Tag so far: %s)", field, b.String())
			}
		}
	}

	b.WriteString("]")

	tag := b.String()
	tag = strings.TrimSpace(tag)
	tag = strings.ToValidUTF8(tag, "")

	invalidChars := regex.InvalidCharsCompile()
	tag = invalidChars.ReplaceAllString(tag, "")

	logging.D(1, "Made metatag %q from file %q", tag, file.Name())

	if tag != "[]" {
		if strings.Contains(filepath.Base(file.Name()), tag) {
			logging.D(2, "Tag %q already detected in name, skipping...", tag)
		} else {
			return tag
		}
	}
	return ""
}
