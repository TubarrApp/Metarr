package metadata

import (
	config "metarr/internal/config"
	consts "metarr/internal/domain/constants"
	enums "metarr/internal/domain/enums"
	keys "metarr/internal/domain/keys"
	"metarr/internal/models"
	browser "metarr/internal/utils/browser"
	logging "metarr/internal/utils/logging"
)

// fillCredits fills in the metadator for credits (e.g. actor, director, uploader)
func fillCredits(fd *models.FileData, data map[string]interface{}) (map[string]interface{}, bool) {

	metaOW := config.GetBool(keys.MOverwrite)

	c := fd.MCredits
	w := fd.MWebData

	fieldMap := map[string]*string{
		// Order by importance
		consts.JOverrideCredits: &c.Override, // Users can assign to this flag to override all credits
		consts.JCreator:         &c.Creator,
		consts.JPerformer:       &c.Performer,
		consts.JAuthor:          &c.Author,
		consts.JArtist:          &c.Artist, // May be alias for "author" in some systems
		consts.JChannel:         &c.Channel,
		consts.JDirector:        &c.Director,
		consts.JActor:           &c.Actor,
		consts.JStudio:          &c.Studio,
		consts.JProducer:        &c.Producer,
		consts.JWriter:          &c.Writer,
		consts.JUploader:        &c.Uploader,
		consts.JPublisher:       &c.Publisher,
		consts.JComposer:        &c.Composer,
	}

	dataFilled := unpackJSON("credits", fieldMap, data)

	// Check if filled
	for key, val := range fieldMap {
		if val == nil {
			logging.PrintE(0, "Value is null")
			continue
		}
		if *val == "" || metaOW {
			logging.PrintD(2, "Value for '%s' is empty, attempting to fill by inference...", key)
			*val = fillEmptyCredits(c)
			logging.PrintD(2, "Set value to '%s'", *val)
			if *val != "" {
				dataFilled = true
			}
		} else if *val != "" {
			dataFilled = true
		}
	}

	switch {
	case dataFilled:
		rtn, err := fd.JSONFileRW.WriteMetadata(fieldMap)
		switch {
		case err != nil:
			logging.PrintE(0, "Failed to write into JSON file '%s': %v", fd.JSONFilePath, err)
			return data, true
		case rtn != nil:
			data = rtn
			return data, true
		}

	case w.WebpageURL == "":
		logging.PrintI("Page URL not found in metadata, so cannot scrape for missing credits in '%s'", fd.JSONFilePath)
		return data, false
	}

	credits := browser.ScrapeMeta(w, enums.WEBCLASS_CREDITS)
	if credits != "" {
		for _, value := range fieldMap {
			if *value == "" {
				*value = credits
			}
		}

		rtn, err := fd.JSONFileRW.WriteMetadata(fieldMap)
		switch {
		case err != nil:
			logging.PrintE(0, "Failed to write new metadata (%s) into JSON file '%s': %v", credits, fd.JSONFilePath, err)
			return data, true
		case rtn != nil:
			data = rtn
			return data, true
		}

	}
	return data, false
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
