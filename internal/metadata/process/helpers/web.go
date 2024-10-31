package metadata

import (
	enums "Metarr/internal/domain/enums"
	"Metarr/internal/types"
	browser "Metarr/internal/utils/browser"
	logging "Metarr/internal/utils/logging"
)

// scrapeMeta gets cookies for a given URL and returns a grabbed string
func ScrapeMeta(w *types.MetadataWebData, find enums.WebClassTags) string {

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
