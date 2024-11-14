package metadata

import (
	consts "metarr/internal/domain/constants"
	"metarr/internal/models"
	logging "metarr/internal/utils/logging"
	print "metarr/internal/utils/print"
)

// Grabs details necessary to scrape the web for missing metafields
func FillWebpageDetails(fd *models.FileData, data map[string]interface{}) bool {

	var isFilled bool

	priorityMap := [5]string{consts.JWebpageURL,
		consts.JURL,
		consts.JReferer,
		consts.JWebpageDomain,
		consts.JDomain}

	printMap := make(map[string]string, len(priorityMap))
	defer func() {
		if len(printMap) > 0 && logging.Level > 1 {
			print.PrintGrabbedFields("web data", printMap)
		}
	}()

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

			logging.D(3, "Got URL: %s", val)

			if fd.MWebData.WebpageURL == "" {
				fd.MWebData.WebpageURL = val
			}
			printMap[k] = val
			fd.MWebData.TryURLs = append(fd.MWebData.TryURLs, val)

			isFilled = true

		case k == consts.JURL:

			logging.D(3, "Got URL: %s", val)

			if fd.MWebData.VideoURL == "" {
				fd.MWebData.VideoURL = val
			}
			printMap[k] = val
			fd.MWebData.TryURLs = append(fd.MWebData.TryURLs, val)

			isFilled = true

		case k == consts.JReferer:

			logging.D(3, "Got URL: %s", val)

			if fd.MWebData.Referer == "" {
				fd.MWebData.Referer = val
			}
			printMap[k] = val
			fd.MWebData.TryURLs = append(fd.MWebData.TryURLs, val)

			isFilled = true

		case k == consts.JWebpageDomain, k == consts.JDomain:

			logging.D(3, "Got URL: %s", val)

			if fd.MWebData.Domain == "" {
				fd.MWebData.Domain = val
			}
			printMap[k] = val

			isFilled = true
		}

	}
	logging.D(2, "Stored URLs for scraping missing fields: %v", fd.MWebData.TryURLs)

	return isFilled
}
