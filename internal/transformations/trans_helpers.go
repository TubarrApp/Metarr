package transformations

import (
	consts "Metarr/internal/domain/constants"
	enums "Metarr/internal/domain/enums"
	"Metarr/internal/models"
	presets "Metarr/internal/transformations/presets"
	logging "Metarr/internal/utils/logging"
	"fmt"
	"regexp"
	"strings"
	"unicode"
)

func TryTransPresets(urls []string, fd *models.FileData) (found bool) {

	for _, url := range urls {
		switch {
		case strings.Contains(url, "censored.tv"):
			presets.CensoredTvTransformations(fd)
			found = true
		}
	}
	return found
}

// getMetafileData retrieves meta type specific data
func getMetafileData(m *models.FileData) (string, string, string) {

	switch m.MetaFileType {
	case enums.METAFILE_JSON:
		return m.JSONBaseName, m.JSONDirectory, m.JSONFilePath
	case enums.METAFILE_NFO:
		return m.NFOBaseName, m.NFODirectory, m.NFOFilePath
	default:
		logging.PrintE(0, "No metafile type set in model %v", m)
		return "", "", ""
	}
}

// Renaming conventions
func applyNamingStyle(style enums.ReplaceToStyle, input string) (output string) {

	switch style {
	case enums.RENAMING_SPACES:
		output = strings.ReplaceAll(input, "_", " ")
	case enums.RENAMING_UNDERSCORES:
		output = strings.ReplaceAll(input, " ", "_")
	default:
		logging.PrintI("Skipping space or underscore renaming conventions...")
		output = input
	}
	return output
}

// addTags handles the tagging of the video files where necessary
func addTags(renamedVideo, renamedMeta string, m *models.FileData, style enums.ReplaceToStyle) (string, string) {

	if len(m.FilenameMetaPrefix) > 2 {
		renamedVideo = fmt.Sprintf("%s %s", m.FilenameMetaPrefix, renamedVideo)
		renamedMeta = fmt.Sprintf("%s %s", m.FilenameMetaPrefix, renamedMeta)
	}

	if len(m.FilenameDateTag) > 2 {
		renamedVideo = fmt.Sprintf("%s %s", m.FilenameDateTag, renamedVideo)
		renamedMeta = fmt.Sprintf("%s %s", m.FilenameDateTag, renamedMeta)
	}

	if style == enums.RENAMING_UNDERSCORES {
		renamedVideo = strings.ReplaceAll(renamedVideo, " ", "_")
		renamedMeta = strings.ReplaceAll(renamedMeta, " ", "_")
	}

	return renamedVideo, renamedMeta
}

// fixContractions fixes the contractions created by FFmpeg's restrict-filenames flag
func fixContractions(videoFilename, metaFilename string, style enums.ReplaceToStyle) (string, string, error) {

	contractionsMap := make(map[string]string, len(consts.ContractionsSpaced))

	// Rename style map to use
	switch style {
	case enums.RENAMING_SPACES:
		contractionsMap = consts.ContractionsSpaced
	case enums.RENAMING_UNDERSCORES:
		contractionsMap = consts.ContractionsUnderscored
	default:
		// Skip or other unsupported parameter returns unchanged
		return videoFilename, metaFilename, nil
	}

	// Function to replace contractions in a filename
	replaceContractions := func(filename string) string {
		for contraction, replacement := range contractionsMap {
			re := regexp.MustCompile(`\b` + regexp.QuoteMeta(contraction) + `\b`)
			repIdx := re.FindStringIndex(strings.ToLower(filename))
			if repIdx == nil {
				continue
			}
			originalContraction := filename[repIdx[0]:repIdx[1]]
			restoredReplacement := ""

			// Match original case for each character in the replacement
			for i, char := range replacement {
				if i < len(originalContraction) && unicode.IsUpper(rune(originalContraction[i])) {
					restoredReplacement += strings.ToUpper(string(char))
				} else {
					restoredReplacement += string(char)
				}
			}
			// Replace in filename with adjusted case
			filename = filename[:repIdx[0]] + restoredReplacement + filename[repIdx[1]:]
		}
		logging.PrintD(2, "Made contraction replacements for file '%s'", filename)
		return filename
	}
	// Replace contractions in both filenames
	videoFilename = replaceContractions(videoFilename)
	videoFilename = strings.TrimSpace(videoFilename)

	metaFilename = replaceContractions(metaFilename)
	metaFilename = strings.TrimSpace(metaFilename)

	return videoFilename, metaFilename, nil
}

// replaceSuffix applies configured suffix replacements to a filename
func replaceSuffix(filename string, suffixes []*models.FilenameReplaceSuffix) string {

	logging.PrintD(2, "Received filename %q", filename)

	if suffixes == nil {
		logging.PrintD(1, "No suffix replacements configured, keeping original filename: %q", filename)
		return filename
	}

	logging.PrintD(2, "Processing filename %q with suffixes: %v", filename, suffixes)

	var result string
	for _, suffix := range suffixes {
		logging.PrintD(2, "Checking suffix '%s' against filename '%s'", suffix.Suffix, filename)

		if strings.HasSuffix(filename, suffix.Suffix) {
			result = strings.TrimSuffix(filename, suffix.Suffix) + suffix.Replacement
			logging.PrintD(2, "Applied suffix replacement: %q -> %q", suffix.Suffix, suffix.Replacement)
		}
	}

	logging.PrintD(2, "Suffix replacement complete: %q -> %q", filename, result)
	return result
}
