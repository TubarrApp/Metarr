package fieldsjson

import (
	"metarr/internal/domain/enums"
	"metarr/internal/domain/logger"
	"metarr/internal/metadata/metawriters"
	"metarr/internal/models"
	"metarr/internal/utils/printout"
	"strings"

	"github.com/TubarrApp/gocommon/logging"
	"github.com/TubarrApp/gocommon/sharedtags"
)

// fillCredits fills in the metadator for credits (e.g. actor, director, uploader).
func fillCredits(fd *models.FileData, json map[string]any, jsonRW *metawriters.JSONFileRW) (map[string]any, bool) {
	var filled, overidden bool

	c := fd.MCredits

	// Order by importance.
	fieldMap := map[string]*string{
		sharedtags.JCreator:   &c.Creator,
		sharedtags.JPerformer: &c.Performer,
		sharedtags.JAuthor:    &c.Author,
		sharedtags.JArtist:    &c.Artist, // May be alias for "author" in some systems.
		sharedtags.JChannel:   &c.Channel,
		sharedtags.JDirector:  &c.Director,
		sharedtags.JActor:     &c.Actor,
		sharedtags.JStudio:    &c.Studio,
		sharedtags.JProducer:  &c.Producer,
		sharedtags.JWriter:    &c.Writer,
		sharedtags.JUploader:  &c.Uploader,
		sharedtags.JPublisher: &c.Publisher,
		sharedtags.JComposer:  &c.Composer,
	}

	printMap := make(map[string]string, len(fieldMap))
	if logging.Level() > 1 {
		defer func() {
			if len(printMap) > 0 {
				printout.PrintGrabbedFields("credits", printMap)
			}
		}()
	}

	if filled = unpackJSON(fieldMap, json); filled {
		logger.Pl.D(2, "Decoded credits JSON into field map")
	}

	// Find highest priority filled element.
	fillWith := c.Override
	if fillWith == "" {
		for _, ptr := range fieldMap {
			if ptr != nil && *ptr != "" {
				fillWith = *ptr
				break
			}
		}
	}

	// Check if filled.
	for k, ptr := range fieldMap {
		if ptr == nil {
			logger.Pl.E("Unexpected nil pointer in credits fieldMap")
			continue
		}
		if *ptr != "" {
			if logging.Level() > 1 {
				printMap[k] = *ptr
			}
			filled = true
			continue
		}
		logger.Pl.D(2, "Value for %q is empty, attempting to fill by inference...", k)

		*ptr = fillWith
		if logging.Level() > 1 {
			printMap[k] = *ptr
		}
		logger.Pl.D(2, "Set value to %q", *ptr)
	}

	if printMap, overidden = overrideAll(fieldMap, fd, printMap); overidden {
		if !filled {
			filled = overidden
		}
	}

	if !filled {
		return json, false
	}
	rtn, err := jsonRW.WriteJSON(fieldMap)
	if err != nil {
		logger.Pl.E("Failed to write into JSON file %q: %v", fd.MetaFilePath, err)
		return json, true
	}
	if rtn != nil {
		return rtn, true
	}
	return json, true
}

// overrideAll makes override replacements if existent.
func overrideAll(fieldMap map[string]*string, fd *models.FileData, printMap map[string]string) (map[string]string, bool) {
	logger.Pl.D(2, "Checking credits field overrides...")
	if fieldMap == nil {
		logger.Pl.E("fieldMap passed in null")
		return printMap, false
	}

	filled := false
	// Note order of operations.
	if len(fd.MetaOps.ReplaceOverrides) > 0 {
		logger.Pl.I("Overriding credits with text replacements...")
		if m, exists := fd.MetaOps.ReplaceOverrides[enums.OverrideMetaCredits]; exists {
			for k, ptr := range fieldMap {
				if ptr == nil {
					logger.Pl.E("Entry is nil in fieldMap %v", fieldMap)
					continue
				}
				logger.Pl.I("Overriding old %q by replacing %q with %q", *ptr, m.Value, m.Replacement)
				*ptr = strings.ReplaceAll(*ptr, m.Value, m.Replacement)

				if logging.Level() > 1 {
					printMap[k] = *ptr
				}

				filled = true
			}
		}
	}

	if len(fd.MetaOps.AppendOverrides) > 0 {
		logger.Pl.I("Overriding credits with appends...")
		if val, exists := fd.MetaOps.AppendOverrides[enums.OverrideMetaCredits]; exists {
			for k, ptr := range fieldMap {
				if ptr == nil {
					logger.Pl.E("Entry is nil in fieldMap %v", fieldMap)
					continue
				}
				logger.Pl.I("Overriding old %q by appending it with %q", *ptr, val)
				*ptr += val

				if logging.Level() > 1 {
					printMap[k] = *ptr
				}
				filled = true
			}
		}
	}
	return printMap, filled
}
