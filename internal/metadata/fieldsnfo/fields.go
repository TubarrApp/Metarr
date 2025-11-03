// Package fieldsnfo is used to for handling NFO metafields.
package fieldsnfo

import (
	"metarr/internal/models"
	"strings"
)

// FillNFO is the primary entrypoint for filling NFO metadata from an open file's read content.
func FillNFO(fd *models.FileData) (filled bool) {
	if ok := fillNFOTimestamps(fd); ok {
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

	if ok := fillNFOWebData(fd); ok {
		filled = true
	}
	return filled
}

// Clean up empty fields from the field map.
func cleanEmptyFields(fieldMap map[string]*string) {
	for _, value := range fieldMap {
		if strings.TrimSpace(*value) == "" {
			*value = ""
		}
	}
}
