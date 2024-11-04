package metadata

import (
	"Metarr/internal/config"
	keys "Metarr/internal/domain/keys"
	"Metarr/internal/domain/regex"
	logging "Metarr/internal/utils/logging"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// makeFilenameTag creates the metatag string to prefix filenames with
func MakeFilenameTag(metadata map[string]interface{}, file *os.File) string {
	logging.PrintD(3, "Entering makeFilenameTag with data@ %v", metadata)

	tagArray := config.GetStringSlice(keys.MFilenamePfx)
	tag := "["

	for field, value := range metadata {
		for i, data := range tagArray {

			if field == data {
				tag += fmt.Sprintf(value.(string))
				logging.PrintD(3, "Added metafield %v data %v to prefix tag (Tag so far: %s)", field, data, tag)

				if i != len(tagArray)-1 {
					tag += "_"
				}
			}
		}
	}
	tag += "]"
	tag = strings.TrimSpace(tag)
	tag = strings.ToValidUTF8(tag, "")

	invalidChars := regex.InvalidCharsCompile()
	tag = invalidChars.ReplaceAllString(tag, "")

	logging.PrintD(1, "Made metatag '%s' from file '%s'", tag, file.Name())

	if tag != "[]" {
		if checkTagExists(tag, filepath.Base(file.Name())) {
			logging.PrintD(2, "Tag '%s' already detected in name, skipping...", tag)
			tag = "[]"
		}
	}
	return tag
}

// checkTagExists checks if the constructed tag already exists in the filename
func checkTagExists(tag, filename string) bool {
	logging.PrintD(3, "Checking if tag '%s' exists in filename '%s'", tag, filename)

	return strings.Contains(filename, tag)
}
