package transformations

import (
	"fmt"
	"metarr/internal/config"
	enums "metarr/internal/domain/enums"
	keys "metarr/internal/domain/keys"
	"metarr/internal/models"
	writefs "metarr/internal/utils/fs/write"
	logging "metarr/internal/utils/logging"
	"path/filepath"
	"strings"
)

// FileRename formats the file names
func FileRename(dataArray []*models.FileData, style enums.ReplaceToStyle) error {

	var (
		vidExt,
		renamedVideo,
		renamedMeta string
	)

	skipVideos := config.GetBool(keys.SkipVideos)

	for _, fd := range dataArray {
		metaBase, metaDir, originalMPath := getMetafileData(fd)
		metaExt := filepath.Ext(originalMPath)

		videoBase := fd.FinalVideoBaseName
		originalVPath := fd.FinalVideoPath

		if config.IsSet(keys.OutputFiletype) {
			vidExt = config.GetString(keys.OutputFiletype)
		} else {
			vidExt = filepath.Ext(fd.OriginalVideoPath)
		}

		if !skipVideos {
			renamedVideo = renameFile(videoBase, style, fd)
			renamedMeta = renamedVideo // Use video name as base to ensure best filename consistency

			logging.D(2, "Renamed video to %s and metafile to %s", renamedVideo, renamedMeta)
		} else {
			renamedMeta = renameFile(metaBase, style, fd)

			logging.D(2, "Renamed metafile to %s", renamedMeta)
		}

		var err error
		if renamedVideo, renamedMeta, err = fixContractions(renamedVideo, renamedMeta, style); err != nil {
			return fmt.Errorf("failed to fix contractions for %s. error: %v", renamedVideo, err)
		}

		// Add the metatag to the front of the filenames
		renamedVideo, renamedMeta = addTags(renamedVideo, renamedMeta, fd, style)

		// Trim trailing spaces
		renamedVideo = strings.TrimSpace(renamedVideo)
		renamedMeta = strings.TrimSpace(renamedMeta)

		logging.D(2, "Rename replacements:\n\nVideo: %v\nMetafile: %v\n\n", renamedVideo, renamedMeta)

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

		var deletedMeta bool
		if config.IsSet(keys.MetaPurge) {
			if err, deletedMeta = fsWriter.DeleteMetafile(renamedMPath); err != nil {
				logging.E(0, "Failed to purge metafile: %v", err)
			}
		}

		if config.IsSet(keys.MoveOnComplete) {
			if err := fsWriter.MoveFile(deletedMeta); err != nil {
				logging.E(0, "Failed to move to destination folder: %v", err)
			}
		}
	}
	return nil
}

// Performs name transformations for metafiles
func renameFile(fileBase string, style enums.ReplaceToStyle, fd *models.FileData) string {
	logging.D(2, "Processing metafile base name: %q", fileBase)

	var (
		suffixes []*models.FilenameReplaceSuffix
		ok       bool
	)

	if len(fd.ModelFileSfxReplace) > 0 {
		suffixes = fd.ModelFileSfxReplace
	} else if config.IsSet(keys.FilenameReplaceSfx) {
		suffixes, ok = config.Get(keys.FilenameReplaceSfx).([]*models.FilenameReplaceSuffix)
		if !ok && len(fd.ModelFileSfxReplace) == 0 {
			logging.E(0, "Got wrong type %T for filename replace suffixes", suffixes)
			return fileBase
		}
	}

	if len(suffixes) == 0 && style == enums.RENAMING_SKIP {
		return fileBase
	} else if len(suffixes) > 0 {
		fileBase = replaceSuffix(fileBase, suffixes)
	}

	if style != enums.RENAMING_SKIP {
		fileBase = applyNamingStyle(style, fileBase)
	} else {
		logging.D(1, "No naming style selected, skipping rename style")
	}
	return fileBase
}
