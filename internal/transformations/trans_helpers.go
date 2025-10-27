package transformations

import (
	"fmt"
	"metarr/internal/abstractions"
	"metarr/internal/dates"
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
	rName := enums.RenamingSkip

	var ok bool
	if abstractions.IsSet(keys.Rename) {
		rName, ok = abstractions.Get(keys.Rename).(enums.ReplaceToStyle)
		if !ok {
			logging.E("Got wrong type or null rename. Got %T, want %q", rName, "enums.ReplaceToStyle")
		}
	}

	switch {
	case fd.FilenameMetaPrefix != "",
		rName != enums.RenamingSkip:

		logging.I("Flag detected that %q should be renamed", fd.OriginalVideoPath)
		rename = true
	}

	if abstractions.IsSet(keys.OutputDirectory) {
		move = true
	}

	if abstractions.IsSet(keys.FilenameOpsModels) {
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

	// Trim whitespace and fix contractions in both filenames
	videoBase = strings.TrimSpace(videoBase)
	metaBase = strings.TrimSpace(metaBase)

	return replaceContractions(videoBase),
		replaceContractions(metaBase),
		nil
}

// setString applies strings as a name for the current file.
func (fp *fileProcessor) setString(filename string, setString models.FOpSet) string {
	if !setString.IsSet {
		logging.E("Dev error: setString is not set for filename %q", filename)
		return filename
	}
	filename = setString.Value
	return fp.metatagParser.FillMetaTemplateTag(filename, fp.metadata)
}

// replaceStrings applies configured string replacements to a filename.
func (fp *fileProcessor) replaceStrings(filename string, replaceStrings []models.FOpReplace) string {
	if len(replaceStrings) == 0 {
		logging.D(1, "No string replacements configured, keeping original filename: %q", filename)
		return filename
	}

	logging.D(2, "Processing filename %s with string replacements: %v", filename, replaceStrings)

	for _, rep := range replaceStrings {
		prevFilename := filename
		filename = fp.metatagParser.FillMetaTemplateTag(filename, fp.metadata)
		filename = strings.ReplaceAll(filename, rep.FindString, rep.Replacement)

		if filename == prevFilename {
			lowerFindString := strings.ToLower(rep.FindString)
			upperFindString := strings.ToUpper(rep.FindString)
			titleFindString := strings.ToTitle(rep.FindString)

			if strings.Contains(filename, lowerFindString) ||
				strings.Contains(filename, upperFindString) ||
				strings.Contains(filename, titleFindString) {
				logging.W("String replacements are case-sensitive!\nFound %q in string %q, but not user-specified %q.", lowerFindString, filename, rep.FindString)
			}
		} else {
			logging.D(2, "Replacement made: %s -> %s (replaced %q with %q)", prevFilename, filename, rep.FindString, rep.Replacement)
		}
	}
	return filename
}

// replaceSuffix applies configured suffix replacements to a filename.
func (fp *fileProcessor) replaceSuffix(filename string, suffixes []models.FOpReplaceSuffix) string {
	logging.D(2, "Received filename %s", filename)

	if len(suffixes) == 0 {
		logging.D(1, "No suffix replacements configured, keeping original filename: %q", filename)
		return filename
	}

	logging.D(2, "Processing filename %s with suffixes: %v", filename, suffixes)

	for _, suffix := range suffixes {
		filename = fp.metatagParser.FillMetaTemplateTag(filename, fp.metadata)
		logging.D(2, "Checking suffix %q against filename %q", suffix.Suffix, filename)

		if before, ok := strings.CutSuffix(filename, suffix.Suffix); ok {
			filename = before + suffix.Replacement
			logging.D(2, "Applied suffix replacement: %q -> %q", suffix.Suffix, suffix.Replacement)
			break // Break after suffix found and removed
		}
	}
	return filename
}

// replacePrefix applies configured suffix replacements to a filename.
func (fp *fileProcessor) replacePrefix(filename string, prefixes []models.FOpReplacePrefix) string {
	logging.D(2, "Received filename %s", filename)

	if len(prefixes) == 0 {
		logging.D(1, "No prefix replacements configured, keeping original filename: %q", filename)
		return filename
	}

	logging.D(2, "Processing filename %s with prefixes: %v", filename, prefixes)

	for _, prefix := range prefixes {
		filename = fp.metatagParser.FillMetaTemplateTag(filename, fp.metadata)
		logging.D(2, "Checking prefix %q against filename %q", prefix.Prefix, filename)

		if after, ok := strings.CutPrefix(filename, prefix.Prefix); ok {
			filename = prefix.Replacement + after
			logging.D(2, "Applied prefix replacement: %q -> %q", prefix.Prefix, prefix.Replacement)
			break // Break after prefix found and removed
		}
	}
	return filename
}

// appendStrings applies configured string appends to a filename.
func (fp *fileProcessor) appendStrings(filename string, appends []models.FOpAppend) string {
	if len(appends) == 0 {
		logging.D(1, "No string appends configured, keeping original filename: %q", filename)
		return filename
	}
	logging.D(2, "Processing filename %s with string appends: %v", filename, appends)
	for _, app := range appends {
		filename = fp.metatagParser.FillMetaTemplateTag(filename, fp.metadata)
		prevFilename := filename
		filename = filename + app.Value
		logging.D(2, "Append made: %s -> %s (appended %q)", prevFilename, filename, app.Value)
	}
	return filename
}

// prefixStrings applies configured string prefixes to a filename.
func (fp *fileProcessor) prefixStrings(filename string, prefixes []models.FOpPrefix) string {
	if len(prefixes) == 0 {
		logging.D(1, "No string prefixes configured, keeping original filename: %q", filename)
		return filename
	}
	logging.D(2, "Processing filename %s with string prefixes: %v", filename, prefixes)
	for _, pre := range prefixes {
		filename = fp.metatagParser.FillMetaTemplateTag(filename, fp.metadata)
		prevFilename := filename
		filename = pre.Value + filename
		logging.D(2, "Prefix made: %s -> %s (prefixed %q)", prevFilename, filename, pre.Value)
	}
	return filename
}

// addDateTag applies a date tag to a filename at the specified location.
func (fp *fileProcessor) addDateTag(filename string, dateTag models.FOpDateTag, dateTagStr string) string {
	if dateTag.DateFormat == enums.DateFmtSkip {
		logging.D(1, "No date tag configured, keeping original filename: %q", filename)
		return filename
	}

	if dateTagStr == "" {
		logging.W("No date tag string provided for date tag, keeping original filename: %q", filename)
		return filename
	}

	var result string
	switch dateTag.Loc {
	case enums.DateTagLocPrefix:
		if strings.HasPrefix(filename, dateTagStr) {
			return filename
		}

		result = dateTagStr + " " + filename
		logging.D(2, "Added date tag prefix: %s -> %s", filename, result)
	case enums.DateTagLocSuffix:
		if strings.HasSuffix(filename, dateTagStr) {
			return filename
		}

		result = filename + " " + dateTagStr
		logging.D(2, "Added date tag suffix: %s -> %s", filename, result)
	default:
		logging.W("Invalid date tag location: %v, keeping original filename", dateTag.Loc)
		return filename
	}

	return result
}

// deleteDateTag removes date tags from a filename at the specified location(s).
func (fp *fileProcessor) deleteDateTag(filename string, deleteTag models.FOpDeleteDateTag) string {
	if deleteTag.DateFormat == enums.DateFmtSkip {
		logging.D(1, "No delete date tag configured, keeping original filename: %q", filename)
		return filename
	}

	prevFilename := filename

	switch deleteTag.Loc {
	case enums.DateTagLocPrefix:
		_, filename = dates.StripDateTags(filename, enums.DateTagLocPrefix)
		logging.D(2, "Stripped prefix date tag: %s -> %s", prevFilename, filename)
	case enums.DateTagLocSuffix:
		_, filename = dates.StripDateTags(filename, enums.DateTagLocSuffix)
		logging.D(2, "Stripped suffix date tag: %s -> %s", prevFilename, filename)
	case enums.DateTagLocAll:
		// Strip all date tags from anywhere in the string
		for {
			oldFilename := filename
			// Look for any [date] pattern
			openTag := strings.Index(filename, "[")
			if openTag == -1 {
				break
			}
			closeTag := strings.Index(filename[openTag:], "]")
			if closeTag == -1 {
				break
			}
			closeTag += openTag

			dateStr := filename[openTag+1 : closeTag]
			if regex.DateTagCompile().MatchString(dateStr) {
				// Remove this date tag
				filename = filename[:openTag] + filename[closeTag+1:]
			} else {
				// Not a valid date tag, skip past this bracket
				if closeTag+1 >= len(filename) {
					break
				}
				// Replace the opening bracket temporarily to skip it
				filename = filename[:openTag] + "\x00" + filename[openTag+1:]
			}

			if oldFilename == filename {
				break
			}
		}
		// Restore any temporarily replaced brackets
		filename = strings.ReplaceAll(filename, "\x00", "[")
		logging.D(2, "Stripped all date tags: %s -> %s", prevFilename, filename)
	default:
		logging.W("Unknown date tag location: %v, keeping original filename", deleteTag.Loc)
		return filename
	}

	return filename
}

// replaceLoneS performs replacements without regex
func replaceLoneS(f string, style enums.ReplaceToStyle) string {
	if style == enums.RenamingSkip {
		return f
	}

	prevString := ""

	// Keep replacing until no more changes occur.
	// Fixes accidental double spaces or double underscores
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
