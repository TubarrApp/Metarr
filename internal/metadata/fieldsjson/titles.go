package fieldsjson

import (
	"metarr/internal/domain/consts"
	"metarr/internal/domain/enums"
	"metarr/internal/metadata/metawriters"
	"metarr/internal/models"
	"metarr/internal/utils/browser"
	"metarr/internal/utils/logging"
	"metarr/internal/utils/printout"
)

// fillTitles grabs titles, subtitles, etc, from JSON.
func fillTitles(fd *models.FileData, json map[string]any, jsonRW *metawriters.JSONFileRW) (map[string]any, bool) {
	t := fd.MTitleDesc
	w := fd.MWebData

	fieldMap := map[string]*string{
		consts.JTitle:     &t.Title,
		consts.JFulltitle: &t.Fulltitle,
		consts.JSubtitle:  &t.Subtitle,
	}

	printMap := make(map[string]string, len(fieldMap))
	if logging.Level > 1 {
		defer func() {
			if len(printMap) > 0 {
				printout.PrintGrabbedFields("titles", printMap)
			}
		}()
	}

	if filled := unpackJSON(fieldMap, json); filled {
		logging.D(2, "Decoded titles JSON into field map")
	}

	// Fill fieldMap entries
	for k, ptr := range fieldMap {
		if ptr == nil {
			logging.E("fieldMap entry pointer unexpectedly nil")
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

	// Infer empty fields
	if t.Title == "" && t.Fulltitle != "" {
		t.Title = t.Fulltitle
	}

	if t.Fulltitle == "" && t.Title != "" {
		t.Fulltitle = t.Title
	}

	if t.Title == "" {
		logging.I("Title is blank, scraping web for missing title data...")

		title := browser.ScrapeMeta(w, enums.WebclassTitle)
		if title != "" {
			t.Title = title
		}
	}

	data, err := jsonRW.WriteJSON(fieldMap)
	if err != nil {
		logging.E("Error writing JSON for file %q: %v", fd.MetaFilePath, err)
		return data, false
	}

	return data, true
}
