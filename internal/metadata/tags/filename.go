package metadata

import (
	"metarr/internal/config"
	keys "metarr/internal/domain/keys"
	"metarr/internal/domain/regex"
	logging "metarr/internal/utils/logging"
	"os"
	"path/filepath"
	"strings"
)

// makeFilenameTag creates the metatag string to prefix filenames with
func MakeFilenameTag(metadata map[string]interface{}, file *os.File) string {
	logging.D(5, "Entering makeFilenameTag with data %v", metadata)

	tagFields := config.GetStringSlice(keys.MFilenamePfx)
	if len(tagFields) == 0 {
		return "[]"
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

	logging.D(1, "Made metatag '%s' from file '%s'", tag, file.Name())

	if tag != "[]" {
		if checkTagExists(tag, filepath.Base(file.Name())) {
			logging.D(2, "Tag '%s' already detected in name, skipping...", tag)
			tag = "[]"
		}
	}
	return tag
}

// checkTagExists checks if the constructed tag already exists in the filename
func checkTagExists(tag, filename string) bool {
	logging.D(3, "Checking if tag '%s' exists in filename '%s'", tag, filename)

	return strings.Contains(filename, tag)
}
