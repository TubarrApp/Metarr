package fieldsnfo

import (
	"metarr/internal/domain/consts"
	"metarr/internal/domain/logger"
	"metarr/internal/models"
	"metarr/internal/utils/printout"

	"github.com/TubarrApp/gocommon/logging"
)

// fillNFOTitles attempts to fill in titles from NFO.
func fillNFOTitles(fd *models.FileData) (filled bool) {
	t := fd.MTitleDesc
	n := fd.NFOData

	fieldMap := map[string]*string{
		consts.NTitle:         &t.Title,
		consts.NOriginalTitle: &t.Fulltitle,
		consts.NTagline:       &t.Subtitle,
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
			printMap[consts.NTitle] = t.Title
		}
	}

	if n.Title.Original != "" {
		if t.Fulltitle == "" {
			t.Fulltitle = n.Title.Original
		}
		if t.Title == "" {
			t.Title = n.Title.Original
			printMap[consts.NTitle] = t.Title
		}
	}

	if n.Title.Sub != "" {
		if t.Subtitle == "" {
			t.Subtitle = n.Title.Sub
			printMap[consts.NSubtitle] = t.Subtitle
		}
	}

	if n.Title.PlainText != "" {
		if t.Title == "" {
			t.Title = n.Title.PlainText
			printMap[consts.NTitle] = t.Title
		}
	}
	return true
}
