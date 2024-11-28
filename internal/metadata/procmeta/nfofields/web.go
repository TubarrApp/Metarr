package nfofields

import (
	"metarr/internal/domain/consts"
	"metarr/internal/models"
	"metarr/internal/utils/logging"
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
	printMap := make(map[string]string, len(fieldMap))

	defer func() {
		if logging.Level > 0 && len(printMap) > 0 {
			printout.PrintGrabbedFields("web info", printMap)
		}
	}()

	if nw.URL != "" {
		if w.WebpageURL == "" {
			w.WebpageURL = nw.URL
			printMap[consts.NURL] = w.WebpageURL
		}
	}

	return true
}
