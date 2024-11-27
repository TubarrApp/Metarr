package jsonfields

import (
	"metarr/internal/domain/consts"
	"metarr/internal/models"
	"metarr/internal/utils/logging"
	"metarr/internal/utils/printout"
)

// FillWebpageDetails grabs details necessary to scrape the web for missing metafields.
func FillWebpageDetails(fd *models.FileData, data map[string]any) bool {

	var isFilled bool
	w := fd.MWebData

	priorityMap := [...]string{consts.JWebpageURL,
		consts.JURL,
		consts.JReferer,
		consts.JWebpageDomain,
		consts.JDomain}

	if w.TryURLs == nil {
		w.TryURLs = make([]string, 0, len(priorityMap))
	}

	var printMap map[string]string
	if logging.Level > 1 {
		printMap = make(map[string]string, len(priorityMap))
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

		switch {
		case k == consts.JWebpageURL:
			if webInfoFill(&w.WebpageURL, val, w) {
				isFilled = true
			}

		case k == consts.JURL:
			if webInfoFill(&w.VideoURL, val, w) {
				isFilled = true
			}

		case k == consts.JReferer:
			if webInfoFill(&w.Referer, val, w) {
				isFilled = true
			}

		case k == consts.JWebpageDomain, k == consts.JDomain:

			if webInfoFill(&w.Domain, val, w) {
				isFilled = true
			}

		default:
			continue
		}

		if logging.Level > 1 && val != "" {
			printMap[k] = val
		}

	}
	logging.D(2, "Stored URLs for scraping missing fields: %v", w.TryURLs)

	return isFilled
}

// webInfoFill fills web info data into the model.
func webInfoFill(s *string, val string, w *models.MetadataWebData) (filled bool) {
	if s == nil {
		logging.E(0, "String passed in null")
		return false
	}
	logging.D(3, "Got URL: %s", val)
	if *s == "" {
		*s = val
	}

	w.TryURLs = append(w.TryURLs, val)
	return *s != ""
}
