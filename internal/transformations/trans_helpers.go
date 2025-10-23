package transformations

import (
	"fmt"
	"metarr/internal/cfg"
	"metarr/internal/domain/enums"
	"metarr/internal/domain/keys"
	"metarr/internal/domain/regex"
	"metarr/internal/models"
	"metarr/internal/transformations/transpresets"
	"metarr/internal/utils/browser"
	"metarr/internal/utils/logging"
	"strings"
	"unicode"
)

// shouldRename determines if file rename operations are needed for this file
func shouldRenameOrMove(fd *models.FileData) (rename, move bool) {
	dateFmt := cfg.GetString(keys.FileDateFmt)
	rName := enums.RenamingSkip

	var ok bool
	if cfg.IsSet(keys.Rename) {
		rName, ok = cfg.Get(keys.Rename).(enums.ReplaceToStyle)
		if !ok {
			logging.E("Got wrong type or null rename. Got %T, want %q", rName, "enums.ReplaceToStyle")
		}
	}

	switch {
	case fd.FilenameMetaPrefix != "",
		len(fd.ModelFileSfxReplace) > 0,
		len(fd.ModelFilePfxReplace) > 0,
		dateFmt != "",
		rName != enums.RenamingSkip:

		logging.I("Flag detected that %q should be renamed\n\nFilename prefix: %q\nFile suffix replacements: %v\nFile prefix replacements:%v\nFile date format: %q\nFile date tag: %q\nFile rename: %v",
			fd.OriginalVideoPath,
			fd.FilenameMetaPrefix,
			fd.ModelFileSfxReplace,
			fd.ModelFilePfxReplace,
			dateFmt,
			fd.FilenameDateTag,
			rName != enums.RenamingSkip)

		rename = true
	}

	if cfg.IsSet(keys.OutputDirectory) {
		move = true
	}

	if cfg.IsSet(keys.InputFileDatePfx) {
		rename = true
	}

	return rename, move
}

// TryTransPresets checks if any URLs in the video metadata have a known match.
// Applies preset transformations for those which match.
func TryTransPresets(urls []string, fd *models.FileData) (matches string) {

	for _, url := range urls {

		_, domain, _, _ := browser.ExtractDomainName(url)

		switch {
		case strings.Contains(domain, "censored.tv"):
			transpresets.CensoredTvTransformations(fd)
			logging.I("Found transformation preset for URL %q", url)
			return url
		default:
			// Not yet implemented
		}
	}
	return ""

}

// getMetafileData retrieves meta type specific data.
func getMetafileData(m *models.FileData) (metaBase, metaDir, metaPath string) {

	switch m.MetaFileType {
	case enums.MetaFiletypeJSON:
		return m.JSONBaseName, m.JSONDirectory, m.JSONFilePath
	case enums.MetaFiletypeNFO:
		return m.NFOBaseName, m.NFODirectory, m.NFOFilePath
	default:
		logging.E("No metafile type set in model %v", m)
		return "", "", ""
	}
}

// applyNamingStyle applies renaming conventions.
func applyNamingStyle(style enums.ReplaceToStyle, input string) (output string) {

	switch style {
	case enums.RenamingSpaces:
		output = strings.ReplaceAll(input, "_", " ")
	case enums.RenamingUnderscores:
		output = strings.ReplaceAll(input, " ", "_")
	default:
		logging.I("Skipping space or underscore renaming conventions...")
		output = input
	}
	return output
}

// addTags handles the tagging of the video files where necessary.
func addTags(renamedVideo, renamedMeta string, m *models.FileData, style enums.ReplaceToStyle) (renamedV, renamedM string) {

	if len(m.FilenameMetaPrefix) > 2 {
		renamedVideo = fmt.Sprintf("%s %s", m.FilenameMetaPrefix, renamedVideo)
		renamedMeta = fmt.Sprintf("%s %s", m.FilenameMetaPrefix, renamedMeta)
	}

	if len(m.FilenameDateTag) > 2 {
		renamedVideo = fmt.Sprintf("%s %s", m.FilenameDateTag, renamedVideo)
		renamedMeta = fmt.Sprintf("%s %s", m.FilenameDateTag, renamedMeta)
	}

	if style == enums.RenamingUnderscores {
		renamedVideo = strings.ReplaceAll(renamedVideo, " ", "_")
		renamedMeta = strings.ReplaceAll(renamedMeta, " ", "_")
	}

	return renamedVideo, renamedMeta
}

// fixContractions fixes the contractions created by FFmpeg's restrict-filenames flag.
func fixContractions(videoBase, metaBase string, fdVideoRef string, style enums.ReplaceToStyle) (renamedV, renamedM string, err error) {

	if videoBase == "" || metaBase == "" {
		return videoBase, metaBase, fmt.Errorf("empty input strings to fix contractions (file %q)", fdVideoRef)
	}

	var contractionsMap map[string]models.ContractionPattern

	switch style {

	case enums.RenamingSpaces:
		contractionsMap = regex.ContractionMapSpacesCompile()

	case enums.RenamingUnderscores:
		contractionsMap = regex.ContractionMapUnderscoresCompile()

	case enums.RenamingFixesOnly:
		contractionsMap = regex.ContractionMapAllCompile()

	default:
		return videoBase, metaBase, nil
	}

	videoBase = replaceLoneS(videoBase, style)
	metaBase = replaceLoneS(metaBase, style)

	fmt.Printf("After replacement - Video: %s, Meta: %s\n", videoBase, metaBase)

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

		logging.D(2, "Made contraction replacements for file %q", filename)
		return filename
	}

	// Replace contractions in both filenames
	videoBase = strings.TrimSpace(videoBase)
	metaBase = strings.TrimSpace(metaBase)
	return replaceContractions(videoBase),
		replaceContractions(metaBase),
		nil
}

// replaceSuffix applies configured suffix replacements to a filename.
func replaceSuffix(filename string, suffixes []models.FilenameReplaceSuffix) string {

	logging.D(2, "Received filename %s", filename)

	if len(suffixes) == 0 {
		logging.D(1, "No suffix replacements configured, keeping original filename: %q", filename)
		return filename
	}

	logging.D(2, "Processing filename %s with suffixes: %v", filename, suffixes)

	var result string
	for _, suffix := range suffixes {
		logging.D(2, "Checking suffix %q against filename %q", suffix.Suffix, filename)

		if strings.HasSuffix(filename, suffix.Suffix) {
			result = strings.TrimSuffix(filename, suffix.Suffix) + suffix.Replacement
			logging.D(2, "Applied suffix replacement: %q -> %q", suffix.Suffix, suffix.Replacement)
		}
	}

	if result != "" {
		logging.D(2, "Suffix replacement complete: %s -> %s", filename, result)
		return result
	}

	return filename
}

// replacePrefix applies configured suffix replacements to a filename.
func replacePrefix(filename string, prefixes []models.FilenameReplacePrefix) string {

	logging.D(2, "Received filename %s", filename)

	if len(prefixes) == 0 {
		logging.D(1, "No prefix replacements configured, keeping original filename: %q", filename)
		return filename
	}

	logging.D(2, "Processing filename %s with prefixes: %v", filename, prefixes)

	var result string
	for _, prefix := range prefixes {
		logging.D(2, "Checking prefix %q against filename %q", prefix.Prefix, filename)

		if strings.HasPrefix(filename, prefix.Prefix) {
			result = strings.TrimPrefix(filename, prefix.Prefix) + prefix.Replacement
			logging.D(2, "Applied prefix replacement: %q -> %q", prefix.Prefix, prefix.Replacement)
		}
	}

	if result != "" {
		logging.D(2, "Suffix replacement complete: %s -> %s", filename, result)
		return result
	}

	return filename
}

// replaceLoneS performs replacements without regex
func replaceLoneS(f string, style enums.ReplaceToStyle) string {
	if style == enums.RenamingSkip {
		return f
	}

	prevString := ""

	// Keep replacing until no more changes occur
	// fixes accidental double spaces or double underscores
	// in the "s" contractions
	for f != prevString {
		prevString = f

		if style == enums.RenamingSpaces || style == enums.RenamingFixesOnly {
			if strings.HasSuffix(f, " s") {
				f = fmt.Sprintf("%ss", f[:len(f)-2])
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

		if style == enums.RenamingUnderscores || style == enums.RenamingFixesOnly {
			if strings.HasSuffix(f, "_s") {
				f = fmt.Sprintf("%ss", f[:len(f)-2])
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
