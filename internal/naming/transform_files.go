package naming

import (
	"Metarr/internal/cmd"
	"Metarr/internal/enums"
	"Metarr/internal/keys"
	"Metarr/internal/logging"
	"Metarr/internal/models"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"unicode"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

// FileRename formats the file names
func FileRename(dataArray []*models.FileData, style enums.ReplaceToStyle) error {

	skipVideos := cmd.GetBool(keys.SkipVideos)

	var renamedVideo string
	var renamedJSON string
	var vidExt string
	var jsonExt string

	for _, m := range dataArray {

		if !skipVideos {
			logging.PrintD(2, "Renaming video with data: %v...", m.JSONFilePath)
			vidExt = filepath.Ext(m.OriginalVideoPath)
			jsonExt = filepath.Ext(m.JSONFilePath)

			logging.PrintD(2, `
Rename function fetched:

Video extension: %v
Video base name: %v
JSON extension: %v
JSON base name: %v
`, vidExt,
				m.FinalVideoBaseName,
				jsonExt,
				m.JSONBaseName)

		} else {
			logging.PrintD(2, "Renaming JSON file: %v...", m.JSONFilePath)
			jsonExt = filepath.Ext(m.JSONFilePath)

			logging.PrintD(2, `
Rename function fetched:

JSON extension: %v
JSON base name: %v
`, jsonExt,
				m.JSONBaseName)
		}

		renamedVideo = m.FinalVideoBaseName
		if !skipVideos {
			renamedJSON = m.FinalVideoBaseName // Rename to the same base name as the video
		} else {
			renamedJSON = m.JSONBaseName
		}

		// Rename to spaces or underscores
		renamedVideo, renamedJSON = spacesOrUnderscores(skipVideos, style, renamedVideo, renamedJSON, m)

		if !skipVideos {
			logging.PrintD(2, `
Rename replacements:

Video: %v
JSON: %v
`, renamedVideo,
				renamedJSON)
		} else {
			logging.PrintD(2, `
Rename replacements:

JSON: %v
`, renamedJSON)
		}

		if style != enums.SKIP {

			var err error
			renamedVideo, renamedJSON, err = fixContractions(renamedVideo, renamedJSON, style)
			if err != nil {
				return fmt.Errorf("failed to fix contractions for %s. error: %v", renamedVideo, err)
			}
		}

		// Trim suffix
		logging.PrintD(3, "Entering suffix trim with video string '%s' and JSON string '%s'", renamedVideo, renamedJSON)
		if cmd.IsSet(keys.FilenameReplaceSfx) {
			renamedVideo, renamedJSON = filenameReplaceSuffix(renamedVideo, renamedJSON)
		}

		// Add the metatag to the front of the filenames
		renamedVideo, renamedJSON = addTags(renamedVideo, renamedJSON, m)

		// Construct final output filepaths
		renamedVideoOut := filepath.Join(m.VideoDirectory, renamedVideo+vidExt)
		renamedJsonOut := filepath.Join(m.JSONDirectory, renamedJSON+jsonExt)

		if err := writeResults(skipVideos, renamedVideoOut, renamedJsonOut, m); err != nil {
			return err
		}
	}
	return nil
}

// writeResults executes the final commands to write the transformed files
func writeResults(skipVideos bool, renamedVideoOut, renamedJsonOut string, m *models.FileData) error {
	if !skipVideos {
		logging.PrintD(1, `
Rename function final commands:

Video: Replacing "%v" with "%v"
JSON: Replacing "%v" with "%v"
`, m.FinalVideoPath, renamedVideoOut,
			m.JSONFilePath, renamedJsonOut)
	} else {
		logging.PrintD(1, `
Rename function final commands:

JSON: Replacing "%v" with "%v"
`, m.JSONFilePath, renamedJsonOut)
	}

	if !cmd.GetBool(keys.SkipVideos) && renamedVideoOut != "" {
		err := os.Rename(m.FinalVideoPath, renamedVideoOut)
		if err != nil {
			return fmt.Errorf("failed to rename %s to %s. error: %v", m.FinalVideoPath, renamedVideoOut, err)
		}
	}

	if renamedJsonOut != "" {
		err := os.Rename(m.JSONFilePath, renamedJsonOut)
		if err != nil {
			return fmt.Errorf("failed to rename %s to %s. error: %v", m.JSONFilePath, renamedJsonOut, err)
		}
	}
	return nil
}

// Renaming conventions
func spacesOrUnderscores(skipVideos bool, style enums.ReplaceToStyle, renamedVideo, renamedJSON string, m *models.FileData) (string, string) {
	switch style {
	case enums.SPACES:
		if !skipVideos {
			renamedVideo = strings.ReplaceAll(m.FinalVideoBaseName, "_", " ")
			renamedJSON = strings.ReplaceAll(m.FinalVideoBaseName, "_", " ")
		} else {
			renamedJSON = strings.ReplaceAll(m.JSONBaseName, "_", " ")
		}

	case enums.UNDERSCORES:
		if !skipVideos {
			renamedVideo = strings.ReplaceAll(m.FinalVideoBaseName, " ", "_")
			renamedJSON = strings.ReplaceAll(m.FinalVideoBaseName, " ", "_")
		} else {
			renamedJSON = strings.ReplaceAll(m.JSONBaseName, " ", "_")
		}
	default:
		logging.PrintI("Skipping space or underscore renaming conventions...")
	}
	return renamedVideo, renamedJSON
}

// addTags handles the tagging of the video files where necessary
func addTags(renamedVideo, renamedJSON string, m *models.FileData) (string, string) {

	if len(m.FilenameMetaPrefix) > 2 {
		renamedVideo = fmt.Sprintf("%s %s", m.FilenameMetaPrefix, renamedVideo)
		renamedJSON = fmt.Sprintf("%s %s", m.FilenameMetaPrefix, renamedJSON)
	}

	if len(m.FilenameDateTag) > 2 {
		renamedVideo = fmt.Sprintf("%s %s", m.FilenameDateTag, renamedVideo)
		renamedJSON = fmt.Sprintf("%s %s", m.FilenameDateTag, renamedJSON)
	}

	return renamedVideo, renamedJSON
}

// fixContractions fixes the contractions created by ffmpeg's restrict-filenames flag
func fixContractions(videoFilename, jsonFilename string, style enums.ReplaceToStyle) (string, string, error) {

	var contractionsMap map[string]string
	if style == enums.SPACES {
		contractionsMap = contractions
	} else if style == enums.UNDERSCORES {
		contractionsMap = contractionsUnderscored
	} else {
		return videoFilename, jsonFilename, nil
	}

	// Initialize the title caser to handle case transformations
	caser := cases.Title(language.English)

	// Function to replace contractions in a filename
	replaceContractions := func(filename string) string {
		for contraction, replacement := range contractionsMap {

			contractionPattern := regexp.MustCompile(`\b` + regexp.QuoteMeta(contraction) + `\b`)
			filename = contractionPattern.ReplaceAllStringFunc(filename, func(match string) string {

				return fixCase(match, replacement, caser)
			})
		}
		logging.PrintD(2, "Made contraction replacements for file '%s'", filename)
		return filename
	}

	// Replace contractions in both filenames
	videoFilename = replaceContractions(videoFilename)
	jsonFilename = replaceContractions(jsonFilename)

	return videoFilename, jsonFilename, nil
}

// fixCase checks if the first character of the match is uppercase and adjust the replacement
// If the first letter of the match is uppercase, it adjusts the replacement to also have
// the first letter as uppercase
func fixCase(match, replacement string, caser cases.Caser) string {

	trimmedMatch := strings.TrimSpace(match)
	if len(trimmedMatch) > 0 && unicode.IsUpper(rune(trimmedMatch[0])) {
		return caser.String(replacement) // Title case the replacement
	}
	return replacement
}

// filenameReplaceSuffix trims the end of a filename
func filenameReplaceSuffix(renamedVideo, renamedJSON string) (string, string) {

	suffixes, ok := cmd.Get(keys.FilenameReplaceSfx).([]models.FilenameReplaceSuffix)
	if !ok {
		logging.PrintE(0, "Entered filename replace suffix function but flag was never set")
		return renamedVideo, renamedJSON
	}

	if suffixes == nil {
		logging.PrintD(1, "Suffix trim array %v sent in empty for video: '%s' and metadata file '%s', returning...",
			suffixes, renamedVideo, renamedJSON)
		return renamedVideo, renamedJSON
	}

	logging.PrintI("Suffixes passed in for renaming video '%s' and metafile '%s': %v",
		renamedVideo, renamedJSON, suffixes)

	trimmedVideo := renamedVideo
	trimmedMeta := renamedJSON

	// Common known compound extensions
	var metaExt string
	switch {
	case strings.HasSuffix(trimmedMeta, ".info.json"):
		metaExt = ".info.json"
	case strings.HasSuffix(trimmedMeta, ".metadata.json"):
		metaExt = ".metadata.json"
	case strings.HasSuffix(trimmedMeta, ".model.json"):
		metaExt = ".model.json"
	default:
		metaExt = filepath.Ext(trimmedMeta)
	}

	for _, suffix := range suffixes {
		// Handle video file
		if strings.HasSuffix(trimmedVideo, suffix.Suffix) {
			trimmedVideo = strings.TrimSuffix(trimmedVideo, suffix.Suffix) + suffix.Replacement
		}

		// Handle JSON file
		baseName := strings.TrimSuffix(trimmedMeta, metaExt)
		if strings.HasSuffix(baseName, suffix.Suffix) {
			baseName = strings.TrimSuffix(baseName, suffix.Suffix) + suffix.Replacement
			trimmedMeta = baseName + metaExt
		}
	}

	logging.PrintD(2, "Leaving suffix trim with video string '%s' and metafile string '%s'", trimmedVideo, trimmedMeta)

	return trimmedVideo, trimmedMeta
}
