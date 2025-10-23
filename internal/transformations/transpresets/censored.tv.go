// Package transpresets contains preset transformations for specific websites.
package transpresets

import (
	"metarr/internal/cfg"
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
		trimSfx []models.MetaTrimSuffix
		ok      bool
	)

	if cfg.IsSet(keys.MTrimSuffix) {
		if trimSfx, ok = cfg.Get(keys.MTrimSuffix).([]models.MetaTrimSuffix); !ok {
			logging.E("Got type %T, may be null", trimSfx)
		}
	}

	var newSfx = make([]models.MetaTrimSuffix, 0, len(trimSfx)+4)

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
	fd.MetaOps.TrimSuffixes = trimSfx
}

// censoredTvFSuffixes adds filename suffix replacements.
func censoredTvFSuffixes(fd *models.FileData) {
	v := fd.OriginalVideoBaseName
	logging.D(3, "Retrieved file name: %s", v)

	if len(v) > 1 {
		check := v[len(v)-2:]
		logging.D(3, "Got last element of file name: %s", check)

		switch check {
		case " 1", "_1":
			addSuffix(fd, check, "")
			logging.I("Added filename suffix replacement %q -> (empty)", check)
		}
	}
	logging.I("Total filename suffix replacements: %d", len(fd.ModelFileSfxReplace))
}

// censoredSuffixExists checks if the suffix exists.
func censoredSuffixExists(suffixes []models.MetaTrimSuffix, field string) bool {
	for _, suffix := range suffixes {
		if suffix.Field == field {
			return true
		}
	}
	return false
}
