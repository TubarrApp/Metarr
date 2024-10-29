package transformations

import (
	"Metarr/internal/config"
	consts "Metarr/internal/domain/constants"
	enums "Metarr/internal/domain/enums"
	keys "Metarr/internal/domain/keys"
	"Metarr/internal/types"
	logging "Metarr/internal/utils/logging"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"unicode"
)

// FileRename formats the file names
func FileRename(dataArray []*types.FileData, style enums.ReplaceToStyle) error {

	skipVideos := config.GetBool(keys.SkipVideos)

	var renamedVideo string
	var renamedMeta string
	var vidExt string
	var metaExt string

	for _, m := range dataArray {

		metaBase, metaDir, metaPath := getMetafileData(m)

		if !skipVideos {
			logging.PrintD(2, "Renaming video with data: %v...", metaPath)
			vidExt = filepath.Ext(m.OriginalVideoPath)
			metaExt = filepath.Ext(metaPath)

			logging.PrintD(2, "\n\nRename function fetched:\n\nVideo extension: %v\nVideo base name: %v\nMetafile extension: %v\nMetafile base name: %v\n\n", vidExt,
				m.FinalVideoBaseName,
				metaExt,
				metaBase)

		} else {
			logging.PrintD(2, "Renaming metafile: %v...", metaPath)
			metaExt = filepath.Ext(metaPath)

			logging.PrintD(2, "\n\nRename function fetched:\n\nMetafile extension: %v\nMetafile base name: %v\n\n", metaExt, metaBase)
		}

		renamedVideo = m.FinalVideoBaseName
		if !skipVideos {
			renamedMeta = m.FinalVideoBaseName // Rename to the same base name as the video
		} else {
			renamedMeta = metaBase
		}

		// Rename to spaces or underscores
		renamedVideo, renamedMeta = spacesOrUnderscores(skipVideos, style, renamedVideo, renamedMeta, m)

		if !skipVideos {
			logging.PrintD(2, "\n\nRename replacements:\n\nVideo: %v\nMetafile\n\n: %v", renamedVideo, renamedMeta)
		} else {
			logging.PrintD(2, "\n\nRename replacements:\n\nMetafile: %v\n\n", renamedMeta)
		}

		if style != enums.SKIP {
			var err error
			renamedVideo, renamedMeta, err = fixContractions(renamedVideo, renamedMeta, style)
			if err != nil {
				return fmt.Errorf("failed to fix contractions for %s. error: %v", renamedVideo, err)
			}
		}

		// Trim suffix
		logging.PrintD(3, "Entering suffix trim with video string '%s' and meta string '%s'", renamedVideo, renamedMeta)
		if config.IsSet(keys.FilenameReplaceSfx) {
			renamedVideo, renamedMeta = filenameReplaceSuffix(renamedVideo, renamedMeta)
		}

		// Add the metatag to the front of the filenames
		renamedVideo, renamedMeta = addTags(renamedVideo, renamedMeta, m)

		// Construct final output filepaths
		renamedVideoOut := filepath.Join(m.VideoDirectory, renamedVideo+vidExt)
		renamedMetaOut := filepath.Join(metaDir, renamedMeta+metaExt)

		if err := writeResults(skipVideos, renamedVideoOut, renamedMetaOut, metaPath, m.FinalVideoPath); err != nil {
			return err
		}
	}
	return nil
}

// getMetafileData retrieves meta type specific data
func getMetafileData(m *types.FileData) (string, string, string) {

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

// writeResults executes the final commands to write the transformed files
func writeResults(skipVideos bool, renamedVideoOut, renamedMetaOut, metaPath, finalVideoPath string) error {

	if !skipVideos {
		logging.PrintD(1, "\n\nRename function final commands:\n\nVideo: Replacing '%v' with '%v'\nMetafile: Replacing '%v' with '%v'\n\n", finalVideoPath, renamedVideoOut,
			metaPath, renamedMetaOut)
	} else {
		logging.PrintD(1, "\n\nRename function final commands:\nMetafile: Replacing '%v' with '%v'\n\n", metaPath, renamedMetaOut)
	}

	if !config.GetBool(keys.SkipVideos) && renamedVideoOut != "" {
		err := os.Rename(finalVideoPath, renamedVideoOut)
		if err != nil {
			return fmt.Errorf("failed to rename %s to %s. error: %v", finalVideoPath, renamedVideoOut, err)
		}
	}

	if renamedMetaOut != "" {
		err := os.Rename(metaPath, renamedMetaOut)
		if err != nil {
			return fmt.Errorf("failed to rename %s to %s. error: %v", metaPath, renamedMetaOut, err)
		}
	}
	return nil
}

// Renaming conventions
func spacesOrUnderscores(skipVideos bool, style enums.ReplaceToStyle, renamedVideo, renamedMeta string, m *types.FileData) (string, string) {

	metaBase, _, _ := getMetafileData(m)
	switch style {
	case enums.SPACES:
		if !skipVideos {
			renamedVideo = strings.ReplaceAll(m.FinalVideoBaseName, "_", " ")
			renamedMeta = strings.ReplaceAll(m.FinalVideoBaseName, "_", " ")
		} else {
			renamedMeta = strings.ReplaceAll(metaBase, "_", " ")
		}

	case enums.UNDERSCORES:
		if !skipVideos {
			renamedVideo = strings.ReplaceAll(m.FinalVideoBaseName, " ", "_")
			renamedMeta = strings.ReplaceAll(m.FinalVideoBaseName, " ", "_")
		} else {
			renamedMeta = strings.ReplaceAll(metaBase, " ", "_")
		}
	default:
		logging.PrintI("Skipping space or underscore renaming conventions...")
	}
	return renamedVideo, renamedMeta
}

// addTags handles the tagging of the video files where necessary
func addTags(renamedVideo, renamedMeta string, m *types.FileData) (string, string) {

	if len(m.FilenameMetaPrefix) > 2 {
		renamedVideo = fmt.Sprintf("%s %s", m.FilenameMetaPrefix, renamedVideo)
		renamedMeta = fmt.Sprintf("%s %s", m.FilenameMetaPrefix, renamedMeta)
	}

	if len(m.FilenameDateTag) > 2 {
		renamedVideo = fmt.Sprintf("%s %s", m.FilenameDateTag, renamedVideo)
		renamedMeta = fmt.Sprintf("%s %s", m.FilenameDateTag, renamedMeta)
	}

	return renamedVideo, renamedMeta
}

// fixContractions fixes the contractions created by FFmpeg's restrict-filenames flag
func fixContractions(videoFilename, metaFilename string, style enums.ReplaceToStyle) (string, string, error) {

	var contractionsMap map[string]string

	// Rename style map to use
	switch style {
	case enums.SPACES:
		contractionsMap = consts.ContractionsSpaced
	case enums.UNDERSCORES:
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

// filenameReplaceSuffix trims the end of a filename
func filenameReplaceSuffix(renamedVideo, renamedMeta string) (string, string) {

	suffixes, ok := config.Get(keys.FilenameReplaceSfx).([]types.FilenameReplaceSuffix)
	if !ok {
		logging.PrintE(0, "Entered filename replace suffix function but flag was never set")
		return renamedVideo, renamedMeta
	}

	if suffixes == nil {
		logging.PrintD(1, "Suffix trim array %v sent in empty for video: '%s' and metadata file '%s', returning...",
			suffixes, renamedVideo, renamedMeta)
		return renamedVideo, renamedMeta
	}

	logging.PrintI("Suffixes passed in for renaming video '%s' and metafile '%s': %v",
		renamedVideo, renamedMeta, suffixes)

	trimmedVideo := renamedVideo
	trimmedMeta := renamedMeta

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

		// Handle metafile
		baseName := strings.TrimSuffix(trimmedMeta, metaExt)
		if strings.HasSuffix(baseName, suffix.Suffix) {
			baseName = strings.TrimSuffix(baseName, suffix.Suffix) + suffix.Replacement
			trimmedMeta = baseName + metaExt
		}
	}

	logging.PrintD(2, "Leaving suffix trim with video string '%s' and metafile string '%s'", trimmedVideo, trimmedMeta)

	return trimmedVideo, trimmedMeta
}
