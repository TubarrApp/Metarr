package jsonfields

import (
	"metarr/internal/domain/consts"
	"metarr/internal/domain/enums"
	"metarr/internal/models"
	"metarr/internal/utils/browser"
	"metarr/internal/utils/logging"
	"metarr/internal/utils/printout"
	"strings"
)

// fillCredits fills in the metadator for credits (e.g. actor, director, uploader)
func fillCredits(fd *models.FileData, json map[string]any) (map[string]any, bool) {
	var (
		filled, overidden bool
	)

	c := fd.MCredits
	w := fd.MWebData

	// Order by importance
	fieldMap := map[string]*string{
		consts.JCreator:   &c.Creator,
		consts.JPerformer: &c.Performer,
		consts.JAuthor:    &c.Author,
		consts.JArtist:    &c.Artist, // May be alias for "author" in some systems
		consts.JChannel:   &c.Channel,
		consts.JDirector:  &c.Director,
		consts.JActor:     &c.Actor,
		consts.JStudio:    &c.Studio,
		consts.JProducer:  &c.Producer,
		consts.JWriter:    &c.Writer,
		consts.JUploader:  &c.Uploader,
		consts.JPublisher: &c.Publisher,
		consts.JComposer:  &c.Composer,
	}

	var printMap map[string]string
	if logging.Level > 1 {
		printMap = make(map[string]string, len(fieldMap))
		defer func() {
			if len(printMap) > 0 {
				printout.PrintGrabbedFields("credits", printMap)
			}
		}()
	}

	// Set using override will fill all values anyway
	if len(models.SetOverrideMap) == 0 {
		if filled = unpackJSON(fieldMap, json); filled {
			logging.D(2, "Decoded credits JSON into field map")
		}

		// Find highest priority filled element
		fillWith := c.Override
		if fillWith == "" {
			for _, ptr := range fieldMap {
				if ptr == nil {
					continue
				}
				if *ptr != "" {
					fillWith = *ptr
					break
				}
			}
		}

		// Check if filled
		for k, ptr := range fieldMap {
			if ptr == nil {
				logging.E(0, "Unexpected nil pointer in credits fieldMap")
				continue
			}

			if *ptr != "" {
				if logging.Level > 1 {
					printMap[k] = *ptr
				}
				filled = true
				continue
			}
			logging.D(2, "Value for %q is empty, attempting to fill by inference...", k)

			*ptr = fillWith
			if logging.Level > 1 {
				printMap[k] = *ptr
			}
			logging.D(2, "Set value to %q", *ptr)
		}
	}

	if printMap, overidden = overrideAll(fieldMap, printMap); overidden {
		if !filled {
			filled = overidden
		}
	}

	// Return if data filled or no web data, else scrape
	switch {
	case filled:
		rtn, err := fd.JSONFileRW.WriteJSON(fieldMap)
		if err != nil {
			logging.E(0, "Failed to write into JSON file %q: %v", fd.JSONFilePath, err)
			return json, true
		}

		if rtn != nil {
			return rtn, true
		}
		return json, true

	case w.WebpageURL == "":
		logging.I("Page URL not found in metadata, so cannot scrape for missing credits in %q", fd.JSONFilePath)
		return json, false
	}

	// Scrape for missing data (write back to file if found)
	credits := browser.ScrapeMeta(w, enums.WebclassCredits)
	if credits != "" {
		for _, value := range fieldMap {
			if *value == "" {
				*value = credits
			}
		}

		rtn, err := fd.JSONFileRW.WriteJSON(fieldMap)
		if err != nil {
			logging.E(0, "Failed to write new metadata (%s) into JSON file %q: %v", credits, fd.JSONFilePath, err)
			return json, true
		}

		if rtn != nil {
			json = rtn
			return json, true
		}
	}
	return json, false
}

// overrideAll makes override replacements if existent
func overrideAll(fieldMap map[string]*string, printMap map[string]string) (map[string]string, bool) {
	logging.D(2, "Checking credits field overrides...")
	if fieldMap == nil {
		logging.E(0, "fieldMap passed in null")
		return printMap, false
	}

	filled := false

	// Note order of operations
	if len(models.ReplaceOverrideMap) > 0 {
		logging.I("Overriding credits with text replacements...")
		if m, exists := models.ReplaceOverrideMap[enums.OverrideMetaCredits]; exists {
			for k, ptr := range fieldMap {
				if ptr == nil {
					logging.E(0, "Entry is nil in fieldMap %v", fieldMap)
					continue
				}
				logging.I("Overriding old %q by replacing %q with %q", *ptr, m.Value, m.Replacement)
				*ptr = strings.ReplaceAll(*ptr, m.Value, m.Replacement)

				if logging.Level > 1 {
					printMap[k] = *ptr
				}

				filled = true
			}
		}
	}

	if len(models.SetOverrideMap) > 0 {
		logging.I("Overriding credits with new values...")
		if val, exists := models.SetOverrideMap[enums.OverrideMetaCredits]; exists {
			for k, ptr := range fieldMap {
				if ptr == nil {
					logging.E(0, "Entry is nil in fieldMap %v", fieldMap)
					continue
				}
				logging.I("Overriding old %q → %q", *ptr, val)
				*ptr = val

				if logging.Level > 1 {
					printMap[k] = *ptr
				}

				filled = true
			}
		}
	}

	if len(models.AppendOverrideMap) > 0 {
		logging.I("Overriding credits with appends...")
		if val, exists := models.AppendOverrideMap[enums.OverrideMetaCredits]; exists {
			for k, ptr := range fieldMap {
				if ptr == nil {
					logging.E(0, "Entry is nil in fieldMap %v", fieldMap)
					continue
				}

				logging.I("Overriding old %q by appending it with %q", *ptr, val)
				*ptr += val

				if logging.Level > 1 {
					printMap[k] = *ptr
				}

				filled = true
			}
		}
	}

	return printMap, filled
}
