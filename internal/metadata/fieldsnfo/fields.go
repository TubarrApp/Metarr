// Package fieldsnfo is used to for handling NFO metafields.
package fieldsnfo

import (
	"metarr/internal/models"
	"strings"
)

// FillNFO is the primary entrypoint for filling NFO metadata from an open file's read content.
func FillNFO(fd *models.FileData) (filled bool) {
	if ok := FillNFOTimestamps(fd); ok {
		filled = true
	}

	if ok := fillNFOTitles(fd); ok {
		filled = true
	}

	if ok := fillNFODescriptions(fd); ok {
		filled = true
	}

	if ok := fillNFOCredits(fd); ok {
		filled = true
	}

	if ok := FillWebData(fd); ok {
		filled = true
	}
	return filled
}

// fillEmpty sets each empty field to val.
func fillEmpty(val string, fields ...*string) {
	for _, f := range fields {
		if *f == "" {
			*f = val
		}
	}
}

// Clean up empty fields from the field map.
func cleanEmptyFields(fieldMap map[string]*string) {
	for _, value := range fieldMap {
		if strings.TrimSpace(*value) == "" {
			*value = ""
		}
	}
}
