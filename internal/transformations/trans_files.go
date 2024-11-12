package transformations

import (
	"fmt"
	"metarr/internal/cfg"
	enums "metarr/internal/domain/enums"
	keys "metarr/internal/domain/keys"
	"metarr/internal/models"
	writefs "metarr/internal/utils/fs/write"
	logging "metarr/internal/utils/logging"
	validate "metarr/internal/utils/validation"
	"path/filepath"
	"strings"
)

// FileRename formats the file names
func FileRename(dataArray []*models.FileData, style enums.ReplaceToStyle, skipVideos bool) error {
	var (
		vidExt,
		renamedVideo,
		renamedMeta string
	)

	for _, fd := range dataArray {

		logging.D(2, "In file renaming loop with '%s'", fd.OriginalVideoBaseName)
		metaBase, metaDir, originalMPath := getMetafileData(fd)
		metaExt := filepath.Ext(originalMPath)

		videoBase := fd.FinalVideoBaseName
		originalVPath := fd.FinalVideoPath

		// Ensure we have the proper video extension
		if cfg.IsSet(keys.OutputFiletype) {
			vidExt = validate.ValidateExtension(cfg.GetString(keys.OutputFiletype))
			if vidExt == "" {
				vidExt = filepath.Ext(originalVPath)
			}
		} else {
			vidExt = filepath.Ext(originalVPath)
		}

		if !skipVideos {
			renamedVideo = renameFile(videoBase, style, fd)
			renamedMeta = renamedVideo // Use video name as base to ensure best filename consistency
			logging.D(2, "Renamed video to '%s' with extension '%s'", renamedVideo, vidExt)
		} else {
			renamedMeta = renameFile(metaBase, style, fd)
			logging.D(3, "Renamed meta now '%s'", renamedMeta)
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

		logging.D(2, "Rename replacements:\nVideo: %v\nMetafile: %v", renamedVideo, renamedMeta)

		// Construct final output filepaths - ensure video gets its extension
		renamedVPath := filepath.Join(fd.VideoDirectory, renamedVideo+vidExt) // Add extension here
		renamedMPath := filepath.Join(metaDir, renamedMeta+metaExt)

		// Log the complete paths to verify extension
		logging.D(1, "Final paths with extensions:\nVideo: %s\nMeta: %s", renamedVPath, renamedMPath)

		// Save into model
		fd.RenamedVideoPath = renamedVPath
		fd.RenamedMetaPath = renamedMPath

		fsWriter := writefs.NewFSFileWriter(skipVideos, renamedVPath, originalVPath, renamedMPath, originalMPath)

		logging.I("Writing final file transformations to filesystem...")
		if err := fsWriter.WriteResults(); err != nil {
			return err
		}

		var deletedMeta bool
		if cfg.IsSet(keys.MetaPurge) {
			if err, deletedMeta = fsWriter.DeleteMetafile(renamedMPath); err != nil {
				logging.E(0, "Failed to purge metafile: %v", err)
			}
		}

		if cfg.IsSet(keys.MoveOnComplete) {
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
	} else if cfg.IsSet(keys.FilenameReplaceSfx) {
		suffixes, ok = cfg.Get(keys.FilenameReplaceSfx).([]*models.FilenameReplaceSuffix)
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
