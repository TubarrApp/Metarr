package metadata

import (
	consts "Metarr/internal/domain/constants"
	enums "Metarr/internal/domain/enums"
	"Metarr/internal/types"
	browser "Metarr/internal/utils/browser"
	logging "Metarr/internal/utils/logging"
	print "Metarr/internal/utils/print"
)

// Grabs details necessary to scrape the web for missing metafields
func fillWebpageDetails(fd *types.FileData, data map[string]interface{}) bool {

	w := fd.MWebData

	printMap := make(map[string]string)
	priorityMap := []string{consts.JWebpageURL, consts.JURL, consts.JReferer, consts.JWebpageDomain, consts.JDomain}

	var isFilled bool

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

// scrapeMeta gets cookies for a given URL and returns a grabbed string
func scrapeMeta(w *types.MetadataWebData, find enums.WebClassTags) string {

	var err error
	data := ""

	w.Cookies, err = browser.GetBrowserCookies(w.WebpageURL)
	if err != nil {
		logging.PrintE(2, "Was unable to grab browser cookies: %v", err)
	}
	for _, try := range w.TryURLs {
		data, err = browser.ScrapeForMetadata(try, w.Cookies, find)
		if err != nil {
			logging.PrintE(0, "Failed to scrape '%s' for credits: %v", try, err)
		} else {
			break
		}
	}
	return data
}
