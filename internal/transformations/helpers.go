package transformations

import (
	"cmp"
	"fmt"
	"maps"
	"metarr/internal/abstractions"
	"metarr/internal/dates"
	"metarr/internal/domain/consts"
	"metarr/internal/domain/enums"
	"metarr/internal/domain/keys"
	"metarr/internal/domain/logger"
	"metarr/internal/domain/regex"
	"metarr/internal/models"
	"metarr/internal/textedit"
	"slices"
	"strings"
	"unicode"
)

// shouldRename determines if file rename operations are needed for this file.
func shouldRenameOrMove(fd *models.FileData) (rename, move bool) {
	rName := enums.RenamingSkip

	var ok bool
	if abstractions.IsSet(keys.Rename) {
		rName, ok = abstractions.Get(keys.Rename).(enums.ReplaceToStyle)
		if !ok {
			logger.Pl.E("Got wrong type or null rename. Got %T, want %q", rName, "enums.ReplaceToStyle")
		}
	}

	switch {
	case rName != enums.RenamingSkip:
		logger.Pl.I("Flag detected that %q should be renamed", fd.OriginalVideoPath)
		rename = true
	}

	if abstractions.IsSet(keys.OutputDirectory) {
		move = true
	}

	if abstractions.IsSet(keys.FilenameOpsModels) || abstractions.IsSet(keys.FilteredFilenameOpsModels) {
		rename = true
	}

	return rename, move
}

// applyNamingStyle converts underscores to spaces or vice versa, trimming any left at either end.
func applyNamingStyle(style enums.ReplaceToStyle, input string) string {
	switch style {
	case enums.RenamingSpaces:
		return strings.TrimSpace(strings.ReplaceAll(input, "_", " "))
	case enums.RenamingUnderscores:
		return strings.Trim(strings.ReplaceAll(input, " ", "_"), "_")
	default:
		return input
	}
}

// fixContractions fixes the contractions created by FFmpeg's restrict-filenames flag. videoBase is blank, and stays so,
// when videos are skipped.
func fixContractions(videoBase, metaBase string, videoFilePath string, style enums.ReplaceToStyle, skipVideos bool) (renamedV, renamedM string, err error) {
	if metaBase == "" || (videoBase == "" && !skipVideos) {
		return videoBase, metaBase, fmt.Errorf("empty input strings to fix contractions (file %q)", videoFilePath)
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

	// Longest contractions first, for a stable result.
	contractions := slices.SortedFunc(maps.Keys(contractionsMap), func(a, b string) int {
		return cmp.Or(len(b)-len(a), strings.Compare(a, b))
	})

	// Function to replace contractions in a filename.
	replaceContractions := func(filename string) string {
		for _, c := range contractions {
			replacement := contractionsMap[c]
			filename = replacement.Regexp.ReplaceAllStringFunc(filename, func(match string) string {
				return matchCase(match, replacement.Replacement)
			})
		}
		logger.Pl.D(2, "Made contraction replacements for file %q", filename)
		return filename
	}
	// Trim whitespace and fix contractions in both filenames.
	videoBase = strings.TrimSpace(videoBase)
	metaBase = strings.TrimSpace(metaBase)

	return replaceContractions(videoBase),
		replaceContractions(metaBase),
		nil
}

// matchCase returns replacement, upper-casing each rune whose position in original holds an upper-case rune.
func matchCase(original, replacement string) string {
	orig := []rune(original)
	out := []rune(replacement)
	for i := range out {
		if i < len(orig) && unicode.IsUpper(orig[i]) {
			out[i] = unicode.ToUpper(out[i])
		}
	}
	return string(out)
}

// setString applies strings as a name for the current file.
func (fp *fileProcessor) setString(filename string, setString models.FOpSet) string {
	if !setString.IsSet {
		logger.Pl.E("%s setString is not set for filename %q", consts.LogTagDevError, filename)
		return filename
	}
	// Fill template.
	result, isTemplate := fp.metatagParser.FillMetaTemplateTag(setString.Value, fp.metadata)
	if result == "" {
		logger.Pl.W("setString result was empty for template %q", setString.Value)
		return filename
	}

	// An unresolved template means the metadata key was missing or non-string.
	// Keep the original name rather than writing the raw template into it.
	if result == setString.Value && isTemplate {
		logger.Pl.W("Could not resolve template %q against metadata, keeping name %q", setString.Value, filename)
		return filename
	}

	// If nothing changed, just return the original.
	if result == filename {
		logger.Pl.D(2, "setString produced same name (%q), skipping", filename)
		return filename
	}
	return result
}

// replaceStrings applies configured string replacements to a filename.
func (fp *fileProcessor) replaceStrings(filename string, replaceStrings []models.FOpReplace) string {
	if len(replaceStrings) == 0 {
		logger.Pl.D(1, "No string replacements configured, keeping original filename: %q", filename)
		return filename
	}

	logger.Pl.D(2, "Processing filename %s with string replacements: %v", filename, replaceStrings)

	for _, rep := range replaceStrings {
		// Expand template tags.
		replacement, isTemplate := fp.metatagParser.FillMetaTemplateTag(rep.Replacement, fp.metadata)
		if replacement == rep.Replacement && isTemplate {
			continue
		}

		// Process.
		prevFilename := filename
		filename = strings.ReplaceAll(filename, rep.FindString, replacement)

		if filename == prevFilename {
			lowerFindString := strings.ToLower(rep.FindString)
			upperFindString := strings.ToUpper(rep.FindString)
			titleFindString := strings.ToTitle(rep.FindString)

			if strings.Contains(filename, lowerFindString) ||
				strings.Contains(filename, upperFindString) ||
				strings.Contains(filename, titleFindString) {
				logger.Pl.W("String replacements are case-sensitive!\nFound %q in string %q, but not user-specified %q.",
					lowerFindString, filename, rep.FindString)
			}
		} else {
			logger.Pl.D(2, "Replacement made: %s -> %s (replaced %q with %q)",
				prevFilename, filename, rep.FindString, replacement)
		}
	}
	return filename
}

// regexReplace applies configured regex replacements to a filename.
func (fp *fileProcessor) regexReplace(filename string, rr []models.FOpRegexReplace) string {
	for _, r := range rr {
		replacement, isTemplate := fp.metatagParser.FillMetaTemplateTag(r.Replacement, fp.metadata)
		if replacement == r.Replacement && isTemplate {
			continue
		}
		prev := filename
		filename = r.Regex.ReplaceAllString(filename, replacement)
		logger.Pl.D(2, "Regex %q replacement: %q -> %q", r.Regex, prev, filename)
	}
	return filename
}

// replaceSuffix applies configured suffix replacements to a filename.
func (fp *fileProcessor) replaceSuffix(filename string, suffixes []models.FOpReplaceSuffix) string {
	if len(suffixes) == 0 {
		logger.Pl.D(1, "No suffix replacements configured, keeping original filename: %q", filename)
		return filename
	}

	for _, suffix := range suffixes {
		// Expand template tags.
		replacement, isTemplate := fp.metatagParser.FillMetaTemplateTag(suffix.Replacement, fp.metadata)
		if replacement == suffix.Replacement && isTemplate {
			continue
		}

		// Process.
		var changed bool
		if filename, changed = textedit.ReplaceSuffix(filename, suffix.Suffix, replacement); changed {
			logger.Pl.D(2, "Applied suffix replacement: %q -> %q", suffix.Suffix, replacement)
		}
	}
	return filename
}

// replacePrefix applies configured prefix replacements to a filename.
func (fp *fileProcessor) replacePrefix(filename string, prefixes []models.FOpReplacePrefix) string {
	if len(prefixes) == 0 {
		logger.Pl.D(1, "No prefix replacements configured, keeping original filename: %q", filename)
		return filename
	}

	for _, prefix := range prefixes {
		// Expand template tags.
		replacement, isTemplate := fp.metatagParser.FillMetaTemplateTag(prefix.Replacement, fp.metadata)
		if replacement == prefix.Replacement && isTemplate {
			continue
		}

		// Process.
		var changed bool
		if filename, changed = textedit.ReplacePrefix(filename, prefix.Prefix, replacement); changed {
			logger.Pl.D(2, "Applied prefix replacement: %q -> %q", prefix.Prefix, replacement)
		}
	}
	return filename
}

// suffixStrings applies configured string suffixes to a filename.
func (fp *fileProcessor) suffixStrings(filename string, suffixes []models.FOpSuffix) string {
	if len(suffixes) == 0 {
		logger.Pl.D(1, "No string suffixes configured, keeping original filename: %q", filename)
		return filename
	}

	values := make([]string, 0, len(suffixes))
	for _, suf := range suffixes {
		// Expand template tags.
		value, isTemplate := fp.metatagParser.FillMetaTemplateTag(suf.Value, fp.metadata)
		if value == suf.Value && isTemplate {
			continue
		}
		values = append(values, value)
	}

	// Process.
	result, changed := textedit.AddSuffixes(filename, values)
	if !changed {
		logger.Pl.D(2, "Filename %q already ends with %q, not suffixing it again", filename, values)
		return filename
	}
	logger.Pl.D(2, "Suffix made: %s -> %s (suffixed %q)", filename, result, values)
	return result
}

// prefixStrings applies configured string prefixes to a filename.
func (fp *fileProcessor) prefixStrings(filename string, prefixes []models.FOpPrefix) string {
	if len(prefixes) == 0 {
		logger.Pl.D(1, "No string prefixes configured, keeping original filename: %q", filename)
		return filename
	}

	values := make([]string, 0, len(prefixes))
	for _, pre := range prefixes {
		// Expand template tags.
		value, isTemplate := fp.metatagParser.FillMetaTemplateTag(pre.Value, fp.metadata)
		if value == pre.Value && isTemplate {
			continue
		}
		values = append(values, value)
	}

	// Process.
	result, changed := textedit.AddPrefixes(filename, values)
	if !changed {
		logger.Pl.D(2, "Filename %q already starts with %q, not prefixing it again", filename, values)
		return filename
	}
	logger.Pl.D(2, "Prefix made: %s -> %s (prefixed %q)", filename, result, values)
	return result
}

// addDateTag applies a date tag to a filename at the specified location.
func (fp *fileProcessor) addDateTag(filename string, dateTag models.FOpDateTag, dateTagStr string) string {
	if dateTag.DateFormat == enums.DateFmtSkip {
		logger.Pl.D(1, "No date tag configured, keeping original filename: %q", filename)
		return filename
	}

	if dateTagStr == "" {
		logger.Pl.W("No date tag string provided for date tag, keeping original filename: %q", filename)
		return filename
	}

	result, changed := dates.AddDateTag(filename, dateTagStr, dateTag.Loc)
	if changed {
		logger.Pl.D(2, "Added date tag: %s -> %s", filename, result)
	}
	return result
}

// deleteDateTag removes date tags from a filename at the specified location(s).
func (fp *fileProcessor) deleteDateTag(filename string, deleteTag models.FOpDeleteDateTag) string {
	if deleteTag.DateFormat == enums.DateFmtSkip {
		logger.Pl.D(1, "No delete date tag configured, keeping original filename: %q", filename)
		return filename
	}

	_, result := dates.StripDateTags(filename, deleteTag.Loc)
	logger.Pl.D(2, "Stripped date tags at %v: %s -> %s", deleteTag.Loc, filename, result)
	return result
}

// replaceLoneS performs replacements without regex.
func replaceLoneS(f string, style enums.ReplaceToStyle) string {
	if style == enums.RenamingSkip {
		return f
	}

	prevString := ""

	// Keep replacing until no more changes occur.
	// Fixes accidental double spaces or double underscores
	// in the "s" contractions.
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
