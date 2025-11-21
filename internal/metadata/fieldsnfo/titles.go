package fieldsnfo

import (
	"metarr/internal/domain/logger"
	"metarr/internal/models"
	"metarr/internal/utils/printout"

	"github.com/TubarrApp/gocommon/logging"
	"github.com/TubarrApp/gocommon/sharedtags"
)

// fillNFOTitles attempts to fill in titles from NFO.
func fillNFOTitles(fd *models.FileData) (filled bool) {
	t := fd.MTitleDesc
	n := fd.NFOData

	fieldMap := map[string]*string{
		sharedtags.NTitle:         &t.Title,
		sharedtags.NOriginalTitle: &t.Fulltitle,
		sharedtags.NTagline:       &t.Subtitle,
	}

	// Post-unmarshal clean.
	cleanEmptyFields(fieldMap)
	printMap := make(map[string]string, len(fieldMap))

	defer func() {
		if logging.Level > 0 && len(printMap) > 0 {
			printout.PrintGrabbedFields("titles", printMap)
		}
	}()

	logger.Pl.I("Grab NFO metadata: %v", t)

	if n.Title.Main != "" {
		if t.Title == "" {
			t.Title = n.Title.Main
			printMap[sharedtags.NTitle] = t.Title
		}
	}

	if n.Title.Original != "" {
		if t.Fulltitle == "" {
			t.Fulltitle = n.Title.Original
		}
		if t.Title == "" {
			t.Title = n.Title.Original
			printMap[sharedtags.NTitle] = t.Title
		}
	}

	if n.Title.Sub != "" {
		if t.Subtitle == "" {
			t.Subtitle = n.Title.Sub
			printMap[sharedtags.NSubtitle] = t.Subtitle
		}
	}

	if n.Title.PlainText != "" {
		if t.Title == "" {
			t.Title = n.Title.PlainText
			printMap[sharedtags.NTitle] = t.Title
		}
	}
	return true
}
