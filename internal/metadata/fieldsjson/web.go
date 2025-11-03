package fieldsjson

import (
	"metarr/internal/domain/consts"
	"metarr/internal/models"
	"metarr/internal/utils/logging"
	"metarr/internal/utils/printout"
)

// FillWebpageDetails grabs details necessary to scrape the web for missing metafields.
func FillWebpageDetails(fd *models.FileData, data map[string]any) bool {
	var isFilled bool

	priorityMap := [...]string{consts.JWebpageURL,
		consts.JURL,
		consts.JReferer,
		consts.JWebpageDomain,
		consts.JDomain}

	if fd.MWebData.TryURLs == nil {
		fd.MWebData.TryURLs = make([]string, 0, len(priorityMap))
	}

	printMap := make(map[string]string, len(priorityMap))
	if logging.Level > 1 {
		defer func() {
			if len(printMap) > 0 {
				printout.PrintGrabbedFields("web info", printMap)
			}
		}()
	}

	// Fill model using priorityMap keys
	for _, k := range priorityMap {
		v, exists := data[k]
		if !exists {
			continue
		}

		val, ok := v.(string)
		if !ok {
			continue
		}

		switch k {
		case consts.JWebpageURL:
			if webInfoFill(&fd.MWebData.WebpageURL, val, fd.MWebData) {
				isFilled = true
			}

		case consts.JURL:
			if webInfoFill(&fd.MWebData.VideoURL, val, fd.MWebData) {
				isFilled = true
			}

		case consts.JReferer:
			if webInfoFill(&fd.MWebData.Referer, val, fd.MWebData) {
				isFilled = true
			}

		case consts.JWebpageDomain, consts.JDomain:
			if webInfoFill(&fd.MWebData.Domain, val, fd.MWebData) {
				isFilled = true
			}

		default:
			continue
		}

		if logging.Level > 1 && val != "" {
			printMap[k] = val
		}

	}
	logging.D(2, "Stored URLs for scraping missing fields: %v", fd.MWebData.TryURLs)

	return isFilled
}

// webInfoFill fills web info data into the model.
func webInfoFill(s *string, val string, w *models.MetadataWebData) (filled bool) {
	if s == nil {
		logging.E("String passed in null")
		return false
	}
	logging.D(3, "Got URL: %s", val)
	if *s == "" {
		*s = val
	}

	w.TryURLs = append(w.TryURLs, val)
	return *s != ""
}
