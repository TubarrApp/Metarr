package config

import (
	keys "Metarr/internal/domain/keys"
	"Metarr/internal/models"
	"strings"
)

// AutoPreset is a switch point for selecting from a range of presets.
// These presets apply common fixes for video/meta files for specific
// video sources.
//
// For example, censored.tv downloads through yt-dlp are often affixed
// with "_1" when filenames are restricted. And titles are often
// affixed with " (1)"
func AutoPreset(url string) {
	if strings.Contains(url, "censored.tv") {
		censoredTvPreset()
	}
}

// censoredTvPreset for censored.tv:
//
// Removes (1) from title fields
// Removes -1 from id and display_id (probably inconsequential)
// Removes the _1 suffix from restricted filenames
func censoredTvPreset() {

	var (
		metaReplaceSuffix []*models.MetaReplaceSuffix
		sfx               *models.MetaReplaceSuffix
	)

	sfx = &models.MetaReplaceSuffix{
		Field:       "title",
		Suffix:      " (1)",
		Replacement: "",
	}
	metaReplaceSuffix = append(metaReplaceSuffix, sfx)

	sfx = &models.MetaReplaceSuffix{
		Field:       "fulltitle",
		Suffix:      " (1)",
		Replacement: "",
	}
	metaReplaceSuffix = append(metaReplaceSuffix, sfx)

	sfx = &models.MetaReplaceSuffix{
		Field:       "id",
		Suffix:      "-1",
		Replacement: "",
	}
	metaReplaceSuffix = append(metaReplaceSuffix, sfx)

	sfx = &models.MetaReplaceSuffix{
		Field:       "display_id",
		Suffix:      "-1",
		Replacement: "",
	}
	metaReplaceSuffix = append(metaReplaceSuffix, sfx)

	Set(keys.MReplaceSfx, metaReplaceSuffix)

	var filenameReplaceSuffix []*models.FilenameReplaceSuffix

	trimEnd := &models.FilenameReplaceSuffix{
		Suffix:      "_1",
		Replacement: "",
	}
	filenameReplaceSuffix = append(filenameReplaceSuffix, trimEnd)
	Set(keys.FilenameReplaceSfx, filenameReplaceSuffix)
}
