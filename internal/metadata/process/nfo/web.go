package metadata

import (
	consts "Metarr/internal/domain/constants"
	"Metarr/internal/models"
	print "Metarr/internal/utils/print"
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

	print.CreateModelPrintout(fd, fd.NFOFilePath, "Parsing NFO descriptions")
	return true
}
