package transformations

import (
	"metarr/internal/cfg"
	keys "metarr/internal/domain/keys"
	"metarr/internal/models"
	logging "metarr/internal/utils/logging"
)

// CensoredTvTransformations adds preset transformations to
// files for censored.tv videos
func CensoredTvTransformations(fd *models.FileData) {

	logging.I("Making preset censored.tv meta replacements")

	censoredTvTrimSuffixes(fd)
	censoredTvFSuffixes(fd)
}

// censoredTvMSuffixes adds meta suffix replacements
func censoredTvTrimSuffixes(fd *models.FileData) {

	var (
		trimSfx []models.MetaTrimSuffix
		ok      bool
	)

	if cfg.IsSet(keys.MTrimSuffix) {
		trimSfx, ok = cfg.Get(keys.MTrimSuffix).([]models.MetaTrimSuffix)
		if !ok {
			logging.E(2, "Got type %T, may be null", trimSfx)
		}
	}

	var newSfx = make([]models.MetaTrimSuffix, 0, 4)

	newSfx = append(newSfx, models.MetaTrimSuffix{
		Field:  "title",
		Suffix: " (1)",
	}, models.MetaTrimSuffix{
		Field:  "fulltitle",
		Suffix: " (1)",
	}, models.MetaTrimSuffix{
		Field:  "id",
		Suffix: "-1",
	}, models.MetaTrimSuffix{
		Field:  "display_id",
		Suffix: "-1",
	})

	for _, newSuffix := range newSfx {
		exists := false
		for _, existingSuffix := range trimSfx {
			if existingSuffix.Field == newSuffix.Field {
				exists = true
				break
			}
		}
		if !exists {
			logging.I("Adding new censored.tv meta suffix replacement: %v", newSuffix)
			trimSfx = append(trimSfx, newSuffix)
		}
	}

	if logging.Level >= 2 {
		var entries []string
		for _, entry := range trimSfx {
			entries = append(entries, "("+entry.Field+":", entry.Suffix+")")
		}
		logging.I("After adding preset suffixes, suffixes to be trimmed for %q: %v", fd.OriginalVideoBaseName, entries)
	}

	fd.ModelMTrimSuffix = trimSfx
}

// censoredTvFSuffixes adds filename suffix replacements
func censoredTvFSuffixes(fd *models.FileData) {

	var sfx []models.FilenameReplaceSuffix

	v := fd.OriginalVideoBaseName

	if cfg.IsSet(keys.FilenameReplaceSfx) {
		existingSfx, ok := cfg.Get(keys.FilenameReplaceSfx).([]models.FilenameReplaceSuffix)
		if !ok {
			logging.E(2, "Unexpected type %T, initializing new suffix list.", existingSfx)
		} else {
			sfx = existingSfx
		}
	}

	logging.D(3, "Retrieved file name: %s", v)
	vExt := ""
	if len(v) > 1 {
		check := v[len(v)-2:]
		logging.D(3, "Got last element of file name: %s", check)
		switch check {
		case " 1", "_1":
			vExt = check
			logging.D(2, "Found file name suffix: %s", vExt)
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
		sfx = append(sfx, models.FilenameReplaceSuffix{
			Suffix:      "_1",
			Replacement: "",
		})
		logging.I("Added filename suffix replacement %q", vExt)
	}

	fd.ModelFileSfxReplace = sfx
	logging.I("Total filename suffix replacements: %d", len(sfx))
}
