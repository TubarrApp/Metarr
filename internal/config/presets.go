package config

import (
	keys "Metarr/internal/domain/keys"
	"Metarr/internal/types"
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
	var metaReplaceSuffix []types.MetaReplaceSuffix

	sfxArg1 := types.MetaReplaceSuffix{
		Field:       "title",
		Suffix:      " (1)",
		Replacement: "",
	}
	metaReplaceSuffix = append(metaReplaceSuffix, sfxArg1)

	sfxArg2 := types.MetaReplaceSuffix{
		Field:       "fulltitle",
		Suffix:      " (1)",
		Replacement: "",
	}
	metaReplaceSuffix = append(metaReplaceSuffix, sfxArg2)

	sfxArg3 := types.MetaReplaceSuffix{
		Field:       "id",
		Suffix:      "-1",
		Replacement: "",
	}
	metaReplaceSuffix = append(metaReplaceSuffix, sfxArg3)

	sfxArg4 := types.MetaReplaceSuffix{
		Field:       "display_id",
		Suffix:      "-1",
		Replacement: "",
	}
	metaReplaceSuffix = append(metaReplaceSuffix, sfxArg4)

	Set(keys.MReplaceSfx, metaReplaceSuffix)

	var filenameReplaceSuffix []types.FilenameReplaceSuffix

	trimEnd := types.FilenameReplaceSuffix{
		Suffix:      "_1",
		Replacement: "",
	}
	filenameReplaceSuffix = append(filenameReplaceSuffix, trimEnd)
	Set(keys.FilenameReplaceSfx, filenameReplaceSuffix)
}
