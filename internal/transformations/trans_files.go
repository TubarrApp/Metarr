// Package transformations handles the transforming of files, e.g. generating new filenames.
package transformations

import (
	"fmt"
	"metarr/internal/abstractions"
	"metarr/internal/dates"
	"metarr/internal/domain/enums"
	"metarr/internal/domain/keys"
	"metarr/internal/domain/regex"
	"metarr/internal/models"
	"metarr/internal/utils/fs/fswrite"
	"metarr/internal/utils/logging"
	"metarr/internal/utils/validation"
	"os"
	"path/filepath"
	"strings"
)

// fileProcessor handles the renaming and moving of files.
type fileProcessor struct {
	fd         *models.FileData
	style      enums.ReplaceToStyle
	skipVideos bool
}

// FileRename formats the file names
func FileRename(fileData *models.FileData, style enums.ReplaceToStyle, skipVideos bool) error {

	fp := &fileProcessor{
		fd:         fileData,
		style:      style,
		skipVideos: skipVideos,
	}

	if err := fp.process(); err != nil {
		return fmt.Errorf("error processing %s: %w", fileData.OriginalVideoBaseName, err)
	}
	return nil
}

// process handles the main file transformation processing logic.
func (fp *fileProcessor) process() error {
	rename, move := shouldRenameOrMove(fp.fd)

	if !rename && !move {
		logging.D(1, "Do not need to rename or move %q", fp.fd.FinalVideoPath)
		return nil
	}

	if !rename {
		logging.D(1, "Do not need to rename %q, just moving...", fp.fd.FinalVideoPath)
		if err := fp.writeResult(); err != nil {
			return err
		}

		return nil
	}

	// Handle renaming
	if err := fp.handleRenaming(); err != nil {
		return err
	}

	// Write changes and handle final operations
	logging.I("Writing final file transformations to filesystem...")
	if err := fp.writeResult(); err != nil {
		return err
	}
	return nil
}

// writeResult handles the purge and move operations.
func (fp *fileProcessor) writeResult() error {
	var (
		err         error
		deletedMeta bool
	)

	fsWriter, err := fswrite.NewFSFileWriter(fp.fd, fp.skipVideos)
	if err != nil {
		return err
	}

	if err := fsWriter.WriteResults(); err != nil {
		return err
	}

	if abstractions.IsSet(keys.MetaPurge) {
		if deletedMeta, err = fsWriter.DeleteMetafile(fp.fd.JSONFilePath); err != nil {
			return fmt.Errorf("failed to purge metafile: %w", err)
		}
	}

	if abstractions.IsSet(keys.OutputDirectory) {
		if err := fsWriter.MoveFile(deletedMeta); err != nil {
			return fmt.Errorf("failed to move to destination folder: %w", err)
		}
	}
	return nil
}

// handleRenaming processes the renaming operations.
func (fp *fileProcessor) handleRenaming() error {
	metaBase, metaDir, originalMPath := getMetafileData(fp.fd)
	videoBase := fp.fd.FinalVideoBaseName
	originalVPath := fp.fd.FinalVideoPath

	// Get ext
	vidExt := fp.determineVideoExtension(originalVPath)

	// Rename
	renamedVideo, renamedMeta := fp.processRenames(videoBase, metaBase)

	// Fix contractions
	var err error
	if renamedVideo, renamedMeta, err = fixContractions(renamedVideo, renamedMeta, fp.fd.OriginalVideoBaseName, fp.style); err != nil {
		return fmt.Errorf("failed to fix contractions for %s. error: %w", renamedVideo, err)
	}

	// Add tags and trim
	renamedVideo, renamedMeta = addTags(renamedVideo, renamedMeta, fp.fd, fp.style)
	renamedVideo = strings.TrimSpace(renamedVideo)
	renamedMeta = strings.TrimSpace(renamedMeta)

	logging.D(2, "Rename replacements:\nVideo: %v\nMetafile: %v", renamedVideo, renamedMeta)

	// Construct and validate final paths
	if err := fp.constructFinalPaths(renamedVideo, renamedMeta, vidExt, metaDir, filepath.Ext(originalMPath)); err != nil {
		return err
	}

	return nil
}

// determineVideoExtension gets the appropriate video extension.
func (fp *fileProcessor) determineVideoExtension(originalPath string) string {
	if !abstractions.IsSet(keys.OutputFiletype) {
		return filepath.Ext(originalPath)
	}

	vidExt := validation.ValidateExtension(abstractions.GetString(keys.OutputFiletype))
	if vidExt == "" {
		vidExt = filepath.Ext(originalPath)
	}
	return vidExt
}

// processRenames handles the renaming logic for both video and meta files.
func (fp *fileProcessor) processRenames(videoBase, metaBase string) (string, string) {
	var renamedVideo, renamedMeta string

	if !fp.skipVideos {
		renamedVideo = constructNewNames(videoBase, fp.style, fp.fd)
		renamedMeta = renamedVideo // Video name as meta base (if possible) for better consistency
		logging.D(2, "Renamed video to %q", renamedVideo)
	} else {
		renamedMeta = constructNewNames(metaBase, fp.style, fp.fd)
		logging.D(3, "Renamed meta now %q", renamedMeta)
	}

	return renamedVideo, renamedMeta
}

// constructFinalPaths creates and validates the final file paths.
func (fp *fileProcessor) constructFinalPaths(renamedVideo, renamedMeta, vidExt, metaDir, metaExt string) error {

	renamedVPath := filepath.Join(fp.fd.VideoDirectory, renamedVideo+vidExt)
	renamedMPath := filepath.Join(metaDir, renamedMeta+metaExt)

	logging.D(1, "Final paths with extensions:\nVideo: %s\nMeta: %s", renamedVPath, renamedMPath)

	var err error

	if filepath.IsAbs(renamedVPath) {
		fp.fd.RenamedVideoPath = renamedVPath
	} else {
		fp.fd.RenamedVideoPath, err = filepath.Abs(renamedVPath)
		if err != nil {
			return fmt.Errorf("failed to get absolute path for renamed video: %w", err)
		}
	}

	if filepath.IsAbs(renamedMPath) {
		fp.fd.RenamedMetaPath = renamedMPath
	} else {
		fp.fd.RenamedMetaPath, err = filepath.Abs(renamedMPath)
		if err != nil {
			return fmt.Errorf("failed to get absolute path for renamed meta: %w", err)
		}
	}

	// Handle final paths if they're set
	if fp.fd.FinalVideoPath != "" && !filepath.IsAbs(fp.fd.FinalVideoPath) {
		fp.fd.FinalVideoPath, err = filepath.Abs(fp.fd.FinalVideoPath)
		if err != nil {
			return fmt.Errorf("failed to get absolute path for final video: %w", err)
		}
	}

	if fp.fd.JSONFilePath != "" && !filepath.IsAbs(fp.fd.JSONFilePath) {
		fp.fd.JSONFilePath, err = filepath.Abs(fp.fd.JSONFilePath)
		if err != nil {
			return fmt.Errorf("failed to get absolute path for JSON file: %w", err)
		}
	}

	logging.D(1, "Saved into struct:\nVideo: %s\nMeta: %s", fp.fd.RenamedVideoPath, fp.fd.RenamedMetaPath)
	return nil
}

// StripDateTagFromFilename strips [date] prefixes from video and metadata files.
func StripDateTagFromFilename(
	matchedFiles map[string]*models.FileData,
	videoMap map[string]*models.FileData,
	metaMap map[string]*models.FileData,
) error {
	// Guard against nil maps
	if matchedFiles == nil {
		return fmt.Errorf("matchedFiles map is nil")
	}
	if videoMap == nil {
		return fmt.Errorf("videoMap is nil")
	}
	if metaMap == nil {
		return fmt.Errorf("metaMap is nil")
	}

	for oldPath, fdata := range matchedFiles {
		if fdata == nil {
			logging.W("Skipping nil FileData for path: %s", oldPath)
			continue
		}

		// --- Handle video file ---
		if fdata.OriginalVideoPath != "" {
			dir := filepath.Dir(fdata.OriginalVideoPath)
			videoBase := filepath.Base(fdata.OriginalVideoPath)

			openTag := strings.IndexRune(videoBase, '[')
			closeTag := strings.IndexRune(videoBase, ']')

			validDateTag := false
			if openTag == 0 && closeTag > openTag {
				dateStr := videoBase[openTag+1 : closeTag]
				if regex.DateTagCompile().MatchString(dateStr) {
					validDateTag = true
				} else {
					logging.I("%v in file %v is not a valid date", dateStr, fdata.OriginalVideoPath)
				}
			}

			if validDateTag {
				newBase := dates.StripDateTag(videoBase, enums.DateTagLogPrefix)
				newVideoPath := filepath.Join(dir, newBase)

				if err := os.Rename(fdata.OriginalVideoPath, newVideoPath); err != nil {
					return fmt.Errorf("failed to rename video %q -> %q: %w", fdata.OriginalVideoPath, newVideoPath, err)
				}

				// Update FileData fields
				fdata.OriginalVideoPath = newVideoPath
				fdata.OriginalVideoBaseName = newBase

				// Update map keys
				delete(videoMap, oldPath)
				videoMap[newVideoPath] = fdata

				delete(matchedFiles, oldPath)
				matchedFiles[newVideoPath] = fdata
				oldPath = newVideoPath
			}
		}

		// --- Handle metadata file ---
		var metaPath, metaBase string
		if fdata.JSONFilePath != "" {
			metaPath = fdata.JSONFilePath
			metaBase = filepath.Base(metaPath)
		} else if fdata.NFOFilePath != "" {
			metaPath = fdata.NFOFilePath
			metaBase = filepath.Base(metaPath)
		} else {
			continue
		}

		openTag := strings.IndexRune(metaBase, '[')
		closeTag := strings.IndexRune(metaBase, ']')

		if openTag == 0 && closeTag > openTag {
			dateStr := metaBase[openTag+1 : closeTag]
			if !regex.DateTagCompile().MatchString(dateStr) {
				logging.I("%v in file %v is not a valid date", dateStr, fdata.OriginalVideoPath)
				continue
			}

			newBase := dates.StripDateTag(metaBase, enums.DateTagLogPrefix)
			newMetaPath := filepath.Join(filepath.Dir(metaPath), newBase)

			if err := os.Rename(metaPath, newMetaPath); err != nil {
				return fmt.Errorf("failed to rename metadata %q -> %q: %w", metaPath, newMetaPath, err)
			}

			// Update FileData fields
			if fdata.JSONFilePath != "" {
				fdata.JSONFilePath = newMetaPath
				fdata.JSONBaseName = newBase
			} else if fdata.NFOFilePath != "" {
				fdata.NFOFilePath = newMetaPath
				fdata.NFOBaseName = newBase
			}

			// Update map keys
			delete(metaMap, oldPath)
			metaMap[newMetaPath] = fdata

			delete(matchedFiles, oldPath)
			matchedFiles[newMetaPath] = fdata
		}
	}
	return nil
}

// constructNewNames constructs the new file names.
func constructNewNames(fileBase string, style enums.ReplaceToStyle, fd *models.FileData) string {
	logging.D(2, "Processing metafile base name: %q", fileBase)

	replacements := fd.FilenameReplaceStrings
	suffixes := fd.FilenameReplaceSuffix
	prefixes := fd.FilenameReplacePrefix

	// Renaming to do?
	if len(suffixes) == 0 && len(prefixes) == 0 && len(replacements) == 0 && style == enums.RenamingSkip {
		return fileBase
	}

	// Make replacements
	if len(replacements) > 0 {
		fileBase = replaceStrings(fileBase, replacements)
	}
	if len(prefixes) > 0 {
		fileBase = replacePrefix(fileBase, prefixes)
	}
	if len(suffixes) > 0 {
		fileBase = replaceSuffix(fileBase, suffixes)
	}

	if style != enums.RenamingSkip {
		fileBase = applyNamingStyle(style, fileBase)
	} else {
		logging.D(1, "No naming style selected, skipping rename style")
	}
	return fileBase
}
