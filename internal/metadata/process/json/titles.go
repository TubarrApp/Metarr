package metadata

import (
	consts "Metarr/internal/domain/constants"
	enums "Metarr/internal/domain/enums"
	helpers "Metarr/internal/metadata/process/helpers"
	"Metarr/internal/types"
	print "Metarr/internal/utils/print"
)

// fillTitles grabs the fulltitle ("title")
func fillTitles(fd *types.FileData, data map[string]interface{}) bool {

	printMap := make(map[string]string)
	t := fd.MTitleDesc
	w := fd.MWebData

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
		title := helpers.ScrapeMeta(w, enums.WEBCLASS_TITLE)
		if title != "" {
			t.Title = title
		}
	}
	print.PrintGrabbedFields("title", &printMap)

	return t.Title != ""
}
