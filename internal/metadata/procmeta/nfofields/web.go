package nfofields

import (
	"metarr/internal/domain/consts"
	"metarr/internal/models"
	"metarr/internal/utils/printout"
)

// fillNFODescriptions attempts to fill in title info from NFO
func fillNFOWebData(fd *models.FileData) bool {

	w := fd.MWebData
	nw := fd.NFOData.WebpageInfo

	fieldMap := map[string]*string{
		consts.NURL: &w.WebpageURL,
	}

	// Post-unmarshal clean
	cleanEmptyFields(fieldMap)

	if nw.URL != "" {
		if w.WebpageURL == "" {
			w.WebpageURL = nw.URL
		}
	}

	printout.CreateModelPrintout(fd, fd.NFOFilePath, "Parsing NFO descriptions")
	return true
}
