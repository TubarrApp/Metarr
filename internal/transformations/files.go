package transformations

import (
	"Metarr/internal/config"
	enums "Metarr/internal/domain/enums"
	keys "Metarr/internal/domain/keys"
	"Metarr/internal/types"
	writefs "Metarr/internal/utils/fs/write"
	logging "Metarr/internal/utils/logging"
	"fmt"
	"path/filepath"
)

// FileRename formats the file names
func FileRename(dataArray []*types.FileData, style enums.ReplaceToStyle) error {

	skipVideos := config.GetBool(keys.SkipVideos)

	var vidExt string

	for _, fd := range dataArray {

		metaBase, metaDir, metaPath := getMetafileData(fd)
		metaExt := filepath.Ext(metaPath)

		videoBase := fd.FinalVideoBaseName
		vidExt = filepath.Ext(fd.OriginalVideoPath)

		renamedVideo := ""
		renamedMeta := ""

		if !skipVideos {
			renamedVideo = renameVideo(videoBase, style)
			renamedMeta = renamedVideo
		} else {
			renamedMeta = renameMeta(metaBase, style)
		}

		logging.PrintD(2, "\n\nRename replacements:\n\nVideo: %v\nMetafile\n\n: %v", renamedVideo, renamedMeta)

		var err error
		if renamedVideo, renamedMeta, err = fixContractions(renamedVideo, renamedMeta, style); err != nil {
			return fmt.Errorf("failed to fix contractions for %s. error: %v", renamedVideo, err)
		}

		// Add the metatag to the front of the filenames
		renamedVideo, renamedMeta = addTags(renamedVideo, renamedMeta, fd)

		// Construct final output filepaths
		renamedVideoOut := filepath.Join(fd.VideoDirectory, renamedVideo+vidExt)
		renamedMetaOut := filepath.Join(metaDir, renamedMeta+metaExt)

		fd.RenamedVideo = renamedVideoOut
		fd.RenamedMeta = renamedMetaOut

		fsWriter := writefs.NewFSFileWriter(skipVideos, renamedVideoOut, fd.FinalVideoPath, renamedMetaOut, metaPath)

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

	// If no transformations are needed, return early
	if !config.IsSet(keys.FilenameReplaceSfx) && style == enums.SKIP {
		return videoBase
	}

	name := videoBase
	if config.IsSet(keys.FilenameReplaceSfx) {
		name = replaceSuffix(name)
	}

	if style != enums.SKIP {
		name = toNamingStyle(style, name)
	}

	return name
}

// Performs name transformations for metafiles
func renameMeta(metaBase string, style enums.ReplaceToStyle) string {
	logging.PrintD(2, "Processing metafile base name: %q", metaBase)

	// If no transformations are needed, return early
	if !config.IsSet(keys.FilenameReplaceSfx) && style == enums.SKIP {
		return metaBase
	}

	name := metaBase
	if config.IsSet(keys.FilenameReplaceSfx) {
		name = replaceSuffix(name)
	}

	if style != enums.SKIP {
		name = toNamingStyle(style, name)
	}

	return name
}
