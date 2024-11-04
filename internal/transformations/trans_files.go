package transformations

import (
	"Metarr/internal/config"
	enums "Metarr/internal/domain/enums"
	keys "Metarr/internal/domain/keys"
	"Metarr/internal/models"
	writefs "Metarr/internal/utils/fs/write"
	logging "Metarr/internal/utils/logging"
	"fmt"
	"path/filepath"
	"strings"
)

// FileRename formats the file names
func FileRename(dataArray []*models.FileData, style enums.ReplaceToStyle) error {

	var vidExt string

	skipVideos := config.GetBool(keys.SkipVideos)

	for _, fd := range dataArray {
		metaBase, metaDir, originalMPath := getMetafileData(fd)
		metaExt := filepath.Ext(originalMPath)

		videoBase := fd.FinalVideoBaseName
		originalVPath := fd.FinalVideoPath
		vidExt = filepath.Ext(fd.OriginalVideoPath)

		renamedVideo := ""
		renamedMeta := ""

		if !skipVideos {
			renamedVideo = renameVideo(videoBase, style)
			renamedMeta = renamedVideo // Use video name as base to ensure best filename consistency
		} else {
			renamedMeta = renameMeta(metaBase, style)
		}

		var err error
		if renamedVideo, renamedMeta, err = fixContractions(renamedVideo, renamedMeta, style); err != nil {
			return fmt.Errorf("failed to fix contractions for %s. error: %v", renamedVideo, err)
		}

		// Add the metatag to the front of the filenames
		renamedVideo, renamedMeta = addTags(renamedVideo, renamedMeta, fd)

		// Trim trailing spaces
		renamedVideo = strings.TrimSpace(renamedVideo)
		renamedMeta = strings.TrimSpace(renamedMeta)

		logging.PrintD(2, "Rename replacements:\n\nVideo: %v\nMetafile: %v\n\n", renamedVideo, renamedMeta)

		// Construct final output filepaths
		renamedVPath := filepath.Join(fd.VideoDirectory, renamedVideo+vidExt)
		renamedMPath := filepath.Join(metaDir, renamedMeta+metaExt)

		// Save into model. May want to save to FinalVideoPath (etc) instead, but currently saves to new field
		fd.RenamedVideoPath = renamedVPath
		fd.RenamedMetaPath = renamedMPath

		fsWriter := writefs.NewFSFileWriter(skipVideos, renamedVPath, originalVPath, renamedMPath, originalMPath)

		if err := fsWriter.WriteResults(); err != nil {
			return err
		}
		if config.IsSet(keys.MoveOnComplete) {
			if err := fsWriter.MoveFile(); err != nil {
				logging.PrintE(0, "Failed to move to destination folder: %v", err)
			}
		}
	}
	return nil
}

// Performs name transformations for video files
func renameVideo(videoBase string, style enums.ReplaceToStyle) string {
	logging.PrintD(2, "Processing video base name: %q", videoBase)

	if !config.IsSet(keys.FilenameReplaceSfx) && style == enums.RENAMING_SKIP {
		return videoBase
	}

	// Transformations
	name := videoBase
	if config.IsSet(keys.FilenameReplaceSfx) {
		name = replaceSuffix(name)
	}

	if style != enums.RENAMING_SKIP {
		name = applyNamingStyle(style, name)
	}
	return name
}

// Performs name transformations for metafiles
func renameMeta(metaBase string, style enums.ReplaceToStyle) string {
	logging.PrintD(2, "Processing metafile base name: %q", metaBase)

	if !config.IsSet(keys.FilenameReplaceSfx) && style == enums.RENAMING_SKIP {
		return metaBase
	}

	// Transformations
	name := metaBase
	if config.IsSet(keys.FilenameReplaceSfx) {
		name = replaceSuffix(name)
	}

	if style != enums.RENAMING_SKIP {
		name = applyNamingStyle(style, name)
	}
	return name
}
