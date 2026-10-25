package fieldsjson

import (
	"metarr/internal/domain/logger"
	"metarr/internal/metadata/metawriters"
	"metarr/internal/models"
	"metarr/internal/utils/printout"

	"github.com/TubarrApp/gocommon/logging"
	"github.com/TubarrApp/gocommon/sharedtags"
)

// creditKeys orders the credit fields by importance, the first filled one filling any that are empty.
var creditKeys = []string{
	sharedtags.JCreator, sharedtags.JPerformer, sharedtags.JAuthor, sharedtags.JArtist, sharedtags.JAlbumArtist,
	sharedtags.JChannel, sharedtags.JDirector, sharedtags.JActor, sharedtags.JStudio, sharedtags.JProducer,
	sharedtags.JWriter, sharedtags.JUploader, sharedtags.JPublisher, sharedtags.JComposer,
}

// fillCredits fills in the metadator for credits (e.g. actor, director, uploader).
func fillCredits(fd *models.FileData, json map[string]any, jsonRW *metawriters.JSONFileRW) (map[string]any, bool) {
	var filled bool

	c := fd.MCredits

	// Credit fields by JSON key.
	fieldMap := map[string]*string{
		sharedtags.JCreator:     &c.Creator,
		sharedtags.JPerformer:   &c.Performer,
		sharedtags.JAuthor:      &c.Author,
		sharedtags.JArtist:      &c.Artist, // May be alias for "author" in some systems.
		sharedtags.JAlbumArtist: &c.AlbumArtist,
		sharedtags.JChannel:     &c.Channel,
		sharedtags.JDirector:    &c.Director,
		sharedtags.JActor:       &c.Actor,
		sharedtags.JStudio:      &c.Studio,
		sharedtags.JProducer:    &c.Producer,
		sharedtags.JWriter:      &c.Writer,
		sharedtags.JUploader:    &c.Uploader,
		sharedtags.JPublisher:   &c.Publisher,
		sharedtags.JComposer:    &c.Composer,
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

	// Album artist falls back to the artist and vice versa.
	if c.AlbumArtist == "" {
		c.AlbumArtist = c.Artist
	}
	if c.Artist == "" {
		c.Artist = c.AlbumArtist
	}

	// Find highest priority filled element.
	fillWith := c.Override
	if fillWith == "" {
		for _, k := range creditKeys {
			if ptr := fieldMap[k]; ptr != nil && *ptr != "" {
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
