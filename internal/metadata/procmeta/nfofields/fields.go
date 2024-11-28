// Package nfofields handles the parsing and filling of NFO metadata fields.
package nfofields

import (
	"metarr/internal/models"
	"metarr/internal/utils/logging"
	"metarr/internal/utils/printout"
	"strings"
)

// FillNFO is the primary entrypoint for filling NFO metadata
// from an open file's read content
func FillNFO(fd *models.FileData) bool {

	var filled bool

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

	if logging.Level > 2 {
		printout.CreateModelPrintout(fd, fd.NFOBaseName, "Fill metadata from NFO for file %q", fd.NFOFilePath)
	}

	return filled
}

// Clean up empty fields from fieldmap
func cleanEmptyFields(fieldMap map[string]*string) {
	for _, value := range fieldMap {
		if strings.TrimSpace(*value) == "" {
			*value = ""
		}
	}
}
