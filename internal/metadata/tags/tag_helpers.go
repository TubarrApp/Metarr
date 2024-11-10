package metadata

import (
	logging "metarr/internal/utils/logging"
	"strings"
)

// TagAlreadyExists checks if the constructed tag already exists in the string
func TagAlreadyExists(tag, s string) bool {
	logging.D(3, "Checking if tag '%s' exists in '%s'", tag, s)
	return strings.Contains(s, tag)
}
