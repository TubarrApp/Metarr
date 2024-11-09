package metadata

import (
	consts "metarr/internal/domain/constants"
	enums "metarr/internal/domain/enums"
	"metarr/internal/models"
	browser "metarr/internal/utils/browser"
	print "metarr/internal/utils/print"
)

// fillTitles grabs the fulltitle ("title")
func fillTitles(fd *models.FileData, data map[string]interface{}) bool {

	t := fd.MTitleDesc
	w := fd.MWebData

	printMap := make(map[string]string, len(data))

	for key, value := range data {
		if val, ok := value.(string); ok && val != "" {
			switch {
			case key == consts.JFulltitle:
				t.Fulltitle = val
				printMap[key] = val

			case key == consts.JTitle:
				t.Title = val
				printMap[key] = val

			case key == consts.JSubtitle:
				t.Subtitle = val
				printMap[key] = val
			}
		}
	}

	if t.Fulltitle != "" {
		t.Title = t.Fulltitle
	} else if t.Fulltitle == "" && t.Title != "" {
		t.Fulltitle = t.Title
	}

	if t.Title == "" {
		title := browser.ScrapeMeta(w, enums.WEBCLASS_TITLE)
		if title != "" {
			t.Title = title
		}
	}
	print.PrintGrabbedFields("title", &printMap)

	return t.Title != ""
}
