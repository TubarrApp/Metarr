package transformations

import (
	"fmt"
	"metarr/internal/abstractions"
	"metarr/internal/dates"
	"metarr/internal/domain/consts"
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
	case rName != enums.RenamingSkip:
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
		logging.E("%s setString is not set for filename %q", consts.LogTagDevError, filename)
		return filename
	}
	// Fill template
	result, _ := fp.metatagParser.FillMetaTemplateTag(setString.Value, fp.metadata)
	if result == "" {
		logging.W("setString result was empty for template %q", setString.Value)
		return filename
	}

	// If nothing changed, just return the original
	if result == filename {
		logging.D(2, "setString produced same name (%q), skipping", filename)
		return filename
	}
	return result
}

// replaceStrings applies configured string replacements to a filename.
func (fp *fileProcessor) replaceStrings(filename string, replaceStrings []models.FOpReplace) string {
	if len(replaceStrings) == 0 {
		logging.D(1, "No string replacements configured, keeping original filename: %q", filename)
		return filename
	}

	logging.D(2, "Processing filename %s with string replacements: %v", filename, replaceStrings)

	for _, rep := range replaceStrings {
		// Expand template tags
		replacement, isTemplate := fp.metatagParser.FillMetaTemplateTag(rep.Replacement, fp.metadata)
		if replacement == rep.Replacement && isTemplate {
			continue
		}

		// Process
		prevFilename := filename
		filename = strings.ReplaceAll(filename, rep.FindString, replacement)

		if filename == prevFilename {
			lowerFindString := strings.ToLower(rep.FindString)
			upperFindString := strings.ToUpper(rep.FindString)
			titleFindString := strings.ToTitle(rep.FindString)

			if strings.Contains(filename, lowerFindString) ||
				strings.Contains(filename, upperFindString) ||
				strings.Contains(filename, titleFindString) {
				logging.W("String replacements are case-sensitive!\nFound %q in string %q, but not user-specified %q.",
					lowerFindString, filename, rep.FindString)
			}
		} else {
			logging.D(2, "Replacement made: %s -> %s (replaced %q with %q)",
				prevFilename, filename, rep.FindString, replacement)
		}
	}
	return filename
}

// replaceSuffix applies configured suffix replacements to a filename.
func (fp *fileProcessor) replaceSuffix(filename string, suffixes []models.FOpReplaceSuffix) string {
	if len(suffixes) == 0 {
		logging.D(1, "No suffix replacements configured, keeping original filename: %q", filename)
		return filename
	}

	for _, suffix := range suffixes {
		// Expand template tags
		replacement, isTemplate := fp.metatagParser.FillMetaTemplateTag(suffix.Replacement, fp.metadata)
		if replacement == suffix.Replacement && isTemplate {
			continue
		}

		// Process
		if before, ok := strings.CutSuffix(filename, suffix.Suffix); ok {
			filename = before + replacement
			logging.D(2, "Applied suffix replacement: %q -> %q", suffix.Suffix, replacement)
			break
		}
	}
	return filename
}

// replacePrefix applies configured prefix replacements to a filename.
func (fp *fileProcessor) replacePrefix(filename string, prefixes []models.FOpReplacePrefix) string {
	if len(prefixes) == 0 {
		logging.D(1, "No prefix replacements configured, keeping original filename: %q", filename)
		return filename
	}

	for _, prefix := range prefixes {
		// Expand template tags
		replacement, isTemplate := fp.metatagParser.FillMetaTemplateTag(prefix.Replacement, fp.metadata)
		if replacement == prefix.Replacement && isTemplate {
			continue
		}

		// Process
		if after, ok := strings.CutPrefix(filename, prefix.Prefix); ok {
			filename = replacement + after
			logging.D(2, "Applied prefix replacement: %q -> %q", prefix.Prefix, replacement)
			break
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

	for _, app := range appends {
		// Expand template tags
		value, isTemplate := fp.metatagParser.FillMetaTemplateTag(app.Value, fp.metadata)
		if value == app.Value && isTemplate {
			continue
		}

		// Process
		prev := filename
		filename = filename + value
		logging.D(2, "Append made: %s -> %s (appended %q)", prev, filename, value)
	}
	return filename
}

// prefixStrings applies configured string prefixes to a filename.
func (fp *fileProcessor) prefixStrings(filename string, prefixes []models.FOpPrefix) string {
	if len(prefixes) == 0 {
		logging.D(1, "No string prefixes configured, keeping original filename: %q", filename)
		return filename
	}

	for _, pre := range prefixes {
		// Expand template tags
		value, isTemplate := fp.metatagParser.FillMetaTemplateTag(pre.Value, fp.metadata)
		if value == pre.Value && isTemplate {
			continue
		}

		// Process
		prev := filename
		filename = value + filename
		logging.D(2, "Prefix made: %s -> %s (prefixed %q)", prev, filename, value)
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
