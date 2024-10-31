package config

import (
	keys "Metarr/internal/domain/keys"
	"Metarr/internal/types"
	"strings"
)

func AutoPreset(url string) {
	if strings.Contains(url, "censored.tv") {
		censoredTvPreset()
	}
}

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

	Set(keys.MOverwrite, true)

	var newFields []types.MetaNewField

	creator := types.MetaNewField{
		Field: "creator",
		Value: "Atheism-is-Unstoppable",
	}
	newFields = append(newFields, creator)

	Set(keys.MNewField, newFields)

	var filenameReplaceSuffix []types.FilenameReplaceSuffix

	trimEnd := types.FilenameReplaceSuffix{
		Suffix:      " 1",
		Replacement: "",
	}
	filenameReplaceSuffix = append(filenameReplaceSuffix, trimEnd)
	Set(keys.FilenameReplaceSfx, filenameReplaceSuffix)

	Set(keys.FileDateFmt, "ymd")
}
