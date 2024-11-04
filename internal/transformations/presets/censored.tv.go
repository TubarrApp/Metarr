package transformations

import (
	config "Metarr/internal/config"
	keys "Metarr/internal/domain/keys"
	"Metarr/internal/models"
	logging "Metarr/internal/utils/logging"
)

// CensoredTvTransformations adds preset transformations to
// files for censored.tv videos
func CensoredTvTransformations(fd *models.FileData) {

	logging.PrintI("Making preset censored.tv meta replacements")

	censoredTvMSuffixes()
	censoredTvFSuffixes(fd)
}

// censoredTvMSuffixes adds meta suffix replacements
func censoredTvMSuffixes() {

	var (
		sfx []*models.MetaReplaceSuffix
		ok  bool
	)

	flagSet := config.IsSet(keys.MReplaceSfx)

	if !flagSet {
		sfx, ok = config.Get(keys.MReplaceSfx).([]*models.MetaReplaceSuffix)
		if !ok {
			logging.PrintE(2, "Got type %T, may be null", sfx)
		}
	}

	var new []*models.MetaReplaceSuffix
	new = append(new, models.NewMetaReplaceSuffix("title", " (1)", ""))
	new = append(new, models.NewMetaReplaceSuffix("fulltitle", " (1)", ""))
	new = append(new, models.NewMetaReplaceSuffix("id", "-1", ""))
	new = append(new, models.NewMetaReplaceSuffix("display_id", "-1", ""))

	for _, newSuffix := range new {
		exists := false
		for _, existingSuffix := range sfx {
			if existingSuffix.Field == newSuffix.Field {
				exists = true
				break
			}
		}
		if !exists {
			logging.PrintI("Adding new censored.tv meta suffix replacement: %v", newSuffix)
			sfx = append(sfx, newSuffix)
		}
	}

	config.Set(keys.MReplaceSfx, sfx)
}

// censoredTvFSuffixes adds filename suffix replacements
func censoredTvFSuffixes(fd *models.FileData) {

	var sfx []*models.FilenameReplaceSuffix

	v := fd.OriginalVideoBaseName

	if config.IsSet(keys.FilenameReplaceSfx) {
		existingSfx, ok := config.Get(keys.FilenameReplaceSfx).([]*models.FilenameReplaceSuffix)
		if !ok {
			logging.PrintE(2, "Unexpected type %T, initializing new suffix list.", existingSfx)
		} else {
			sfx = existingSfx
		}
	}

	logging.PrintD(3, "Retrieved file name: %s", v)
	vExt := ""
	if len(v) > 1 {
		check := v[len(v)-2:]
		logging.PrintD(3, "Got last element of file name: %s", check)
		switch check {
		case " 1", "_1":
			vExt = check
			logging.PrintD(2, "Found file name suffix: %s", vExt)
		}
	}

	// Check if suffix is already present
	alreadyExists := false
	for _, existingSuffix := range sfx {
		if existingSuffix.Suffix == vExt && existingSuffix.Replacement == "" {
			alreadyExists = true
			break
		}
	}

	// Add suffix if it does not already exist
	if !alreadyExists {
		sfx = append(sfx, models.NewFilenameReplaceSuffix(vExt, ""))
		logging.PrintI("Added filename suffix replacement '%s'", vExt)
	}

	config.Set(keys.FilenameReplaceSfx, sfx)
	logging.PrintI("Total filename suffix replacements: %d", len(sfx))
}
