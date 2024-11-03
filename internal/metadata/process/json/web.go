package metadata

import (
	consts "Metarr/internal/domain/constants"
	"Metarr/internal/types"
	logging "Metarr/internal/utils/logging"
	print "Metarr/internal/utils/print"
)

// Grabs details necessary to scrape the web for missing metafields
func fillWebpageDetails(fd *types.FileData, data map[string]interface{}) bool {

	var isFilled bool

	w := fd.MWebData

	priorityMap := [5]string{consts.JWebpageURL,
		consts.JURL,
		consts.JReferer,
		consts.JWebpageDomain,
		consts.JDomain}

	printMap := make(map[string]string, len(priorityMap))

	for _, wanted := range priorityMap {
		for key, value := range data {

			if val, ok := value.(string); ok && val != "" {
				if key == wanted {
					switch {
					case key == consts.JWebpageURL:

						logging.PrintD(3, "Got URL: %s", val)

						if w.WebpageURL == "" {
							w.WebpageURL = val
						}
						printMap[key] = val
						w.TryURLs = append(w.TryURLs, val)

						isFilled = true

					case key == consts.JURL:

						logging.PrintD(3, "Got URL: %s", val)

						if w.VideoURL == "" {
							w.VideoURL = val
						}
						printMap[key] = val
						w.TryURLs = append(w.TryURLs, val)

						isFilled = true

					case key == consts.JReferer:

						logging.PrintD(3, "Got URL: %s", val)

						if w.Referer == "" {
							w.Referer = val
						}
						printMap[key] = val
						w.TryURLs = append(w.TryURLs, val)

						isFilled = true

					case key == consts.JWebpageDomain, key == consts.JDomain:

						logging.PrintD(3, "Got URL: %s", val)

						if w.Domain == "" {
							w.Domain = val
						}
						printMap[key] = val

						isFilled = true
					}
				}
			}
		}
	}

	logging.PrintD(2, "Stored URLs for scraping missing fields: %v", w.TryURLs)

	print.PrintGrabbedFields("web details", &printMap)

	return isFilled
}
