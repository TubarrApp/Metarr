package metadata

import (
	consts "metarr/internal/domain/constants"
	enums "metarr/internal/domain/enums"
	"metarr/internal/models"
	browser "metarr/internal/utils/browser"
	logging "metarr/internal/utils/logging"
	print "metarr/internal/utils/print"
)

// fillTitles grabs the fulltitle ("title")
func fillTitles(fd *models.FileData, json map[string]interface{}) (map[string]interface{}, bool) {

	t := fd.MTitleDesc
	w := fd.MWebData

	printMap := make(map[string]string, len(json))

	fieldMap := map[string]*string{
		consts.JTitle:     &t.Title,
		consts.JFulltitle: &t.Fulltitle,
		consts.JSubtitle:  &t.Subtitle,
	}

	if filled := unpackJSON("titles", fieldMap, json); filled {
		logging.D(2, "Decoded titles JSON into field map")
	}

	for k, v := range json {
		logging.I("Checking title key '%s' exists in JSON", k)
		if val, ok := v.(string); ok && val != "" {
			logging.I("Title key '%s' exists with value '%s'", k, val)

			switch k {
			case consts.JFulltitle:
				t.Fulltitle = val
				printMap[k] = val

			case consts.JTitle:
				t.Title = val
				printMap[k] = val

			case consts.JSubtitle:
				t.Subtitle = val
				printMap[k] = val
			}
		}
	}

	if t.Title == "" && t.Fulltitle != "" {
		t.Title = t.Fulltitle
	}

	if t.Fulltitle == "" && t.Title != "" {
		t.Fulltitle = t.Title
	}

	if t.Title == "" {
		logging.I("Title is blank, scraping web for missing title data...")

		title := browser.ScrapeMeta(w, enums.WEBCLASS_TITLE)
		if title != "" {
			t.Title = title
		}
	}

	if logging.Level > -1 {
		print.PrintGrabbedFields("title", &printMap)
	}

	data, err := fd.JSONFileRW.WriteJSON(fieldMap)
	if err != nil {
		logging.E(0, err.Error())
		return data, false
	}

	return data, true
}
