package metadata

import (
	consts "Metarr/internal/domain/constants"
	enums "Metarr/internal/domain/enums"
	helpers "Metarr/internal/metadata/process/helpers"
	"Metarr/internal/types"
	logging "Metarr/internal/utils/logging"
)

// fillCredits fills in the metadator for credits (e.g. actor, director, uploader)
func fillCredits(fd *types.FileData, data map[string]interface{}) (map[string]interface{}, bool) {

	c := fd.MCredits
	w := fd.MWebData

	fieldMap := map[string]*string{
		// Order by importance
		consts.JCreator:   &c.Creator,
		consts.JPerformer: &c.Performer,
		consts.JAuthor:    &c.Author,
		consts.JArtist:    &c.Artist, // May be alias for "author" in some systems
		consts.JDirector:  &c.Director,
		consts.JActor:     &c.Actor,
		consts.JStudio:    &c.Studio,
		consts.JProducer:  &c.Producer,
		consts.JWriter:    &c.Writer,
		consts.JUploader:  &c.Uploader,
		consts.JPublisher: &c.Publisher,
		consts.JComposer:  &c.Composer,
	}

	dataFilled := unpackJSON("credits", fieldMap, data)

	if !dataFilled {
		for _, val := range fieldMap {
			if val != nil && *val == "" {
				continue
			} else {
				dataFilled = true
			}
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

	credits := helpers.ScrapeMeta(w, enums.WEBCLASS_CREDITS)

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
