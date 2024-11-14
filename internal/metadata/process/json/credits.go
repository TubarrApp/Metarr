package metadata

import (
	consts "metarr/internal/domain/constants"
	enums "metarr/internal/domain/enums"
	"metarr/internal/models"
	browser "metarr/internal/utils/browser"
	logging "metarr/internal/utils/logging"
	print "metarr/internal/utils/print"
	"strings"
)

// fillCredits fills in the metadator for credits (e.g. actor, director, uploader)
func fillCredits(fd *models.FileData, json map[string]interface{}) (map[string]interface{}, bool) {
	var (
		filled, overriden bool
	)

	c := fd.MCredits
	w := fd.MWebData

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
				print.PrintGrabbedFields("credits", printMap)
			}
		}()
	}

	// Set using override will fill all values anyway
	if len(models.SetOverrideMap) == 0 {
		if filled = unpackJSON(fieldMap, json); filled {
			logging.D(2, "Decoded credits JSON into field map")
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
			logging.D(2, "Value for '%s' is empty, attempting to fill by inference...", k)

			*ptr = fillEmptyCredits(c)
			if logging.Level > 1 {
				printMap[k] = *ptr
			}
			logging.D(2, "Set value to '%s'", *ptr)
		}
	}

	if printMap, overriden = overrideAll(fieldMap, printMap); overriden {
		if !filled {
			filled = overriden
		}
	}

	// Return if data filled or no web data, else scrape
	switch {
	case filled:
		rtn, err := fd.JSONFileRW.WriteJSON(fieldMap)
		if err != nil {
			logging.E(0, "Failed to write into JSON file '%s': %v", fd.JSONFilePath, err)
			return json, true
		}

		if rtn != nil {
			return rtn, true
		}
		return json, true

	case w.WebpageURL == "":
		logging.I("Page URL not found in metadata, so cannot scrape for missing credits in '%s'", fd.JSONFilePath)
		return json, false
	}

	// Scrape for missing data (write back to file if found)
	credits := browser.ScrapeMeta(w, enums.WEBCLASS_CREDITS)
	if credits != "" {
		for _, value := range fieldMap {
			if *value == "" {
				*value = credits
			}
		}

		rtn, err := fd.JSONFileRW.WriteJSON(fieldMap)
		if err != nil {
			logging.E(0, "Failed to write new metadata (%s) into JSON file '%s': %v", credits, fd.JSONFilePath, err)
			return json, true
		}

		if rtn != nil {
			json = rtn
			return json, true
		}
	}
	return json, false
}

// fillEmptyCredits attempts to fill empty fields by inference
func fillEmptyCredits(c *models.MetadataCredits) string {

	// Order by importance
	switch {
	case c.Override != "":
		return c.Override

	case c.Creator != "":
		return c.Creator

	case c.Author != "":
		return c.Author

	case c.Publisher != "":
		return c.Publisher

	case c.Producer != "":
		return c.Producer

	case c.Actor != "":
		return c.Actor

	case c.Channel != "":
		return c.Channel

	case c.Performer != "":
		return c.Performer

	case c.Uploader != "":
		return c.Uploader

	case c.Artist != "":
		return c.Artist

	case c.Director != "":
		return c.Director

	case c.Studio != "":
		return c.Studio

	case c.Writer != "":
		return c.Writer

	case c.Composer != "":
		return c.Composer

	default:
		return ""
	}
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
		if m, exists := models.ReplaceOverrideMap[enums.OVERRIDE_META_CREDITS]; exists {
			for k, ptr := range fieldMap {
				if ptr == nil {
					logging.E(0, "Entry is nil in fieldMap %v", fieldMap)
					continue
				}
				logging.I("Overriding old '%s' by replacing '%s' with '%s'", *ptr, m.Value, m.Replacement)
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
		if val, exists := models.SetOverrideMap[enums.OVERRIDE_META_CREDITS]; exists {
			for k, ptr := range fieldMap {
				if ptr == nil {
					logging.E(0, "Entry is nil in fieldMap %v", fieldMap)
					continue
				}
				logging.I("Overriding old '%s' to '%s'", *ptr, val)
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
		if val, exists := models.AppendOverrideMap[enums.OVERRIDE_META_CREDITS]; exists {
			for k, ptr := range fieldMap {
				if ptr == nil {
					logging.E(0, "Entry is nil in fieldMap %v", fieldMap)
					continue
				}

				logging.I("Overriding old '%s' by appending it with '%s'", *ptr, val)
				*ptr = *ptr + val

				if logging.Level > 1 {
					printMap[k] = *ptr
				}

				filled = true
			}
		}
	}

	return printMap, filled
}
