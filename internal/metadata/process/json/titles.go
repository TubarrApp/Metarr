package metadata

import (
	consts "Metarr/internal/domain/constants"
	enums "Metarr/internal/domain/enums"
	"Metarr/internal/models"
	browser "Metarr/internal/utils/browser"
	print "Metarr/internal/utils/print"
)

// fillTitles grabs the fulltitle ("title")
func fillTitles(fd *models.FileData, data map[string]interface{}) bool {

	t := fd.MTitleDesc
	w := fd.MWebData

	printMap := make(map[string]string, len(data))

	for key, value := range data {
		if val, ok := value.(string); ok && val != "" {
			switch {
			case key == consts.JTitle:
				t.Title = val
				printMap[key] = val

			case key == consts.JFallbackTitle:
				t.FallbackTitle = val
				printMap[key] = val

			case key == consts.JSubtitle:
				t.Subtitle = val
				printMap[key] = val
			}
		}
	}
	if t.Title == "" && t.FallbackTitle != "" {
		t.Title = t.FallbackTitle
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
