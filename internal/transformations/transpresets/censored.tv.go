// Package transpresets contains preset transformations for specific websites.
package transpresets

import (
	"metarr/internal/abstractions"
	"metarr/internal/domain/keys"
	"metarr/internal/models"
	"metarr/internal/utils/logging"
)

// CensoredTvTransformations adds preset transformations to
// files for censored.tv videos.
func CensoredTvTransformations(fd *models.FileData) {
	logging.I("Making preset censored.tv meta replacements")

	censoredTvTrimSuffixes(fd)
	censoredTvFSuffixes(fd)
}

// censoredTvMSuffixes adds meta suffix replacements.
func censoredTvTrimSuffixes(fd *models.FileData) {
	var (
		trimSfx []models.MetaReplaceSuffix
		ok      bool
	)

	if abstractions.IsSet(keys.MTrimSuffix) {
		if trimSfx, ok = abstractions.Get(keys.MTrimSuffix).([]models.MetaReplaceSuffix); !ok {
			logging.E("Got type %T, may be null", trimSfx)
		}
	}

	var newSfx = make([]models.MetaReplaceSuffix, 0, len(trimSfx)+4)

	newSfx = append(newSfx, models.MetaReplaceSuffix{
		Field:       "title",
		Suffix:      " (1)",
		Replacement: "",
	}, models.MetaReplaceSuffix{
		Field:       "fulltitle",
		Suffix:      " (1)",
		Replacement: "",
	}, models.MetaReplaceSuffix{
		Field:       "id",
		Suffix:      "-1",
		Replacement: "",
	}, models.MetaReplaceSuffix{
		Field:       "display_id",
		Suffix:      "-1",
		Replacement: "",
	})

	for _, newSuffix := range newSfx {
		if !censoredSuffixExists(trimSfx, newSuffix.Field) {
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
	fd.MetaOps.ReplaceSuffixes = trimSfx
}

// censoredTvFSuffixes adds filename suffix replacements.
func censoredTvFSuffixes(fd *models.FileData) {
	vBaseName := fd.OriginalVideoBaseName
	logging.D(3, "Retrieved file name: %s", vBaseName)

	if len(vBaseName) > 1 {
		checkForSuffix := vBaseName[len(vBaseName)-2:]
		logging.D(3, "Got last element of file name: %s", checkForSuffix)

		switch checkForSuffix {
		case " 1", "_1":
			addSuffix(fd, checkForSuffix, "")
			logging.I(`Added filename suffix replacement %q -> ""`, checkForSuffix)
		}
	}
	logging.I("Total filename suffix replacements: %d", len(fd.FilenameOps.ReplaceSuffixes))
}

// censoredSuffixExists checks if the suffix exists.
func censoredSuffixExists(suffixes []models.MetaReplaceSuffix, field string) bool {
	for _, suffix := range suffixes {
		if suffix.Field == field {
			return true
		}
	}
	return false
}
