package transformations

import (
	"fmt"
	enums "metarr/internal/domain/enums"
	"metarr/internal/domain/regex"
	"metarr/internal/models"
	presets "metarr/internal/transformations/presets"
	logging "metarr/internal/utils/logging"
	"strings"
	"unicode"
)

// TryTransPresets checks if any URLs in the video metadata have a known match.
// Applies preset transformations for those which match.
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

// getMetafileData retrieves meta type specific data.
func getMetafileData(m *models.FileData) (metaBase, metaDir, metaPath string) {

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

// applyNamingStyle applies renaming conventions.
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

// addTags handles the tagging of the video files where necessary.
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

// fixContractions fixes the contractions created by FFmpeg's restrict-filenames flag.
func fixContractions(videoFilename, metaFilename string, style enums.ReplaceToStyle) (string, string, error) {

	contractionsMap := make(map[string]*models.ContractionPattern)
	// Rename style map to use
	switch style {

	case enums.RENAMING_SPACES:
		contractionsMap = regex.ContractionMapSpacesCompile()

	case enums.RENAMING_UNDERSCORES:
		contractionsMap = regex.ContractionMapUnderscoresCompile()

	case enums.RENAMING_FIXES_ONLY:
		contractionsMap = regex.ContractionMapAllCompile()

	default:
		return videoFilename, metaFilename, nil
	}

	videoFilename = replaceLoneS(videoFilename, style)
	metaFilename = replaceLoneS(metaFilename, style)

	fmt.Printf("After replacement - Video: %s, Meta: %s\n", videoFilename, metaFilename)

	// Function to replace contractions in a filename
	replaceContractions := func(filename string) string {
		for _, replacement := range contractionsMap {
			repIdx := replacement.Regexp.FindStringIndex(strings.ToLower(filename))
			if repIdx == nil {
				continue
			}

			var b strings.Builder
			b.Grow(len(replacement.Replacement))
			originalContraction := filename[repIdx[0]:repIdx[1]]

			// Match original case for each character in the replacement
			for i, char := range replacement.Replacement {
				if i < len(originalContraction) && unicode.IsUpper(rune(originalContraction[i])) {
					b.WriteString(strings.ToUpper(string(char)))
				} else {
					b.WriteString(string(char))
				}
			}

			// Replace in filename with adjusted case
			filename = filename[:repIdx[0]] + b.String() + filename[repIdx[1]:]
			b.Reset()
		}

		logging.PrintD(2, "Made contraction replacements for file '%s'", filename)
		return filename
	}

	// Replace contractions in both filenames
	videoFilename = strings.TrimSpace(videoFilename)
	metaFilename = strings.TrimSpace(metaFilename)
	return replaceContractions(videoFilename),
		replaceContractions(metaFilename),
		nil
}

// replaceSuffix applies configured suffix replacements to a filename.
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

// replaceLoneS performs replacements without regex
func replaceLoneS(f string, style enums.ReplaceToStyle) string {
	if style == enums.RENAMING_SKIP {
		return f
	}

	prevString := ""

	// Keep replacing until no more changes occur
	// fixes accidental double spaces or double underscores
	// in the "s" contractions
	for f != prevString {
		prevString = f

		if style == enums.RENAMING_SPACES || style == enums.RENAMING_FIXES_ONLY {
			if strings.HasSuffix(f, " s") {
				f = f[:len(f)-2] + "s"
			}

			f = strings.ReplaceAll(f, " s ", "s ")
			f = strings.ReplaceAll(f, " s.", "s.")
			f = strings.ReplaceAll(f, " s[", "s[")
			f = strings.ReplaceAll(f, " s(", "s(")
			f = strings.ReplaceAll(f, " s)", "s)")
			f = strings.ReplaceAll(f, " s]", "s]")
			f = strings.ReplaceAll(f, " s-", "s-")
			f = strings.ReplaceAll(f, " s_", "s_")
			f = strings.ReplaceAll(f, " s,", "s,")
			f = strings.ReplaceAll(f, " s!", "s!")
			f = strings.ReplaceAll(f, " s'", "s'")
			f = strings.ReplaceAll(f, " s&", "s&")
			f = strings.ReplaceAll(f, " s=", "s=")
			f = strings.ReplaceAll(f, " s;", "s;")
			f = strings.ReplaceAll(f, " s#", "s#")
			f = strings.ReplaceAll(f, " s@", "s@")
			f = strings.ReplaceAll(f, " s$", "s$")
			f = strings.ReplaceAll(f, " s%", "s%")
			f = strings.ReplaceAll(f, " s+", "s+")
			f = strings.ReplaceAll(f, " s{", "s{")
			f = strings.ReplaceAll(f, " s}", "s}")
		}

		if style == enums.RENAMING_UNDERSCORES || style == enums.RENAMING_FIXES_ONLY {
			if strings.HasSuffix(f, "_s") {
				f = f[:len(f)-2] + "s"
			}

			f = strings.ReplaceAll(f, "_s_", "s_")
			f = strings.ReplaceAll(f, "_s.", "s.")
			f = strings.ReplaceAll(f, "_s[", "s[")
			f = strings.ReplaceAll(f, "_s(", "s(")
			f = strings.ReplaceAll(f, "_s)", "s)")
			f = strings.ReplaceAll(f, "_s]", "s]")
			f = strings.ReplaceAll(f, "_s-", "s-")
			f = strings.ReplaceAll(f, "_s ", "s ")
			f = strings.ReplaceAll(f, "_s,", "s,")
			f = strings.ReplaceAll(f, "_s!", "s!")
			f = strings.ReplaceAll(f, "_s'", "s'")
			f = strings.ReplaceAll(f, "_s&", "s&")
			f = strings.ReplaceAll(f, "_s=", "s=")
			f = strings.ReplaceAll(f, "_s;", "s;")
			f = strings.ReplaceAll(f, "_s#", "s#")
			f = strings.ReplaceAll(f, "_s@", "s@")
			f = strings.ReplaceAll(f, "_s$", "s$")
			f = strings.ReplaceAll(f, "_s%", "s%")
			f = strings.ReplaceAll(f, "_s+", "s+")
			f = strings.ReplaceAll(f, "_s{", "s{")
			f = strings.ReplaceAll(f, "_s}", "s}")
		}
	}
	return f
}
