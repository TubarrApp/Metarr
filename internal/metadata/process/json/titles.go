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

	fieldMap := map[string]*string{
		consts.JTitle:     &t.Title,
		consts.JFulltitle: &t.Fulltitle,
		consts.JSubtitle:  &t.Subtitle,
	}

	printMap := make(map[string]string, len(fieldMap))
	defer func() {
		if len(printMap) > 0 && logging.Level > 1 {
			print.PrintGrabbedFields("title", printMap)
		}
	}()

	if filled := unpackJSON(fieldMap, json); filled {
		logging.D(2, "Decoded titles JSON into field map")
	}

	for k, ptr := range fieldMap {
		if ptr == nil {
			logging.E(0, "fieldMap entry pointer unexpectedly nil")
			continue
		}

		v, exists := json[k]
		if !exists {
			continue
		}

		val, ok := v.(string)
		if !ok {
			continue
		}

		if *ptr == "" {
			*ptr = val
		}

		if logging.Level > 1 {
			printMap[k] = val
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

	data, err := fd.JSONFileRW.WriteJSON(fieldMap)
	if err != nil {
		logging.E(0, err.Error())
		return data, false
	}

	return data, true
}
