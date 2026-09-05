package fieldsnfo

import (
	"metarr/internal/models"
	"metarr/internal/utils/printout"

	"github.com/TubarrApp/gocommon/logging"
	"github.com/TubarrApp/gocommon/sharedtags"
)

// FillWebData attempts to fill in web data from NFO.
func FillWebData(fd *models.FileData) (filled bool) {
	w := fd.MWebData
	n := fd.NFOData
	nw := n.WebpageInfo

	fieldMap := map[string]*string{
		sharedtags.NURL:   &w.WebpageURL,
		sharedtags.NThumb: &w.Thumbnail,
	}

	// Post-unmarshal clean.
	cleanEmptyFields(fieldMap)
	printMap := make(map[string]string, len(fieldMap))

	defer func() {
		if logging.Level > 0 && len(printMap) > 0 {
			printout.PrintGrabbedFields("web info", printMap)
		}
	}()

	// Prefer the movie-level fields, falling back to the nested <web> element.
	if w.WebpageURL == "" {
		switch {
		case n.URL != "":
			w.WebpageURL = n.URL
		case nw.URL != "":
			w.WebpageURL = nw.URL
		}
		if w.WebpageURL != "" {
			printMap[sharedtags.NURL] = w.WebpageURL
		}
	}

	if w.Thumbnail == "" {
		switch {
		case n.Thumb != "":
			w.Thumbnail = n.Thumb
		case n.Poster != "":
			w.Thumbnail = n.Poster
		case n.Fanart != "":
			w.Thumbnail = n.Fanart
		case nw.Thumb != "":
			w.Thumbnail = nw.Thumb
		case nw.Fanart != "":
			w.Thumbnail = nw.Fanart
		}
		if w.Thumbnail != "" {
			printMap[sharedtags.NThumb] = w.Thumbnail
		}
	}
	return true
}
