// Package transformations handles the transforming of files, e.g. generating new filenames.
package transformations

import (
	"fmt"
	"metarr/internal/abstractions"
	"metarr/internal/domain/enums"
	"metarr/internal/domain/keys"
	"metarr/internal/models"
	"metarr/internal/parsing"
	"metarr/internal/utils/fs/fswrite"
	"metarr/internal/utils/logging"
	"metarr/internal/utils/validation"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
)

var filenameTaken sync.Map

// getUniqueFilename appends numbers onto a filename if the filename already exists.
func getUniqueFilename(base string) string {
	counter, _ := filenameTaken.LoadOrStore(base, &atomic.Int32{})
	n := counter.(*atomic.Int32).Add(1)
	var candidate string
	if n == 1 {
		candidate = base
	} else {
		candidate = fmt.Sprintf("%s (%d)", base, n-1)
	}
	return candidate
}

// fileProcessor handles the renaming and moving of files.
type fileProcessor struct {
	fd            *models.FileData
	style         enums.ReplaceToStyle
	metatagParser *parsing.MetaTemplateParser
	metadata      map[string]any
	skipVideos    bool
}

// FileRename formats the file names
func FileRename(fileData *models.FileData, style enums.ReplaceToStyle, skipVideos bool) (err error) {
	fp := &fileProcessor{
		fd:            fileData,
		style:         style,
		skipVideos:    skipVideos,
		metatagParser: parsing.NewMetaTemplateParser(fileData.JSONFilePath),
	}

	metaFile, err := os.Open(fp.fd.JSONFilePath)
	if err != nil {
		return err
	}
	defer func() {
		if err := metaFile.Close(); err != nil {
			logging.E("Error closing file %q: %v", metaFile.Name(), err)
		}
	}()

	fp.metadata, err = fileData.JSONFileRW.DecodeJSON(metaFile)
	if err != nil {
		return err
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
		renamedVideo = fp.constructNewNames(videoBase, fp.style, fp.fd)
		renamedMeta = renamedVideo // Video name as meta base (if possible) for better consistency
		logging.D(2, "Renamed video to %q", renamedVideo)
	} else {
		renamedMeta = fp.constructNewNames(metaBase, fp.style, fp.fd)
		logging.D(3, "Renamed meta now %q", renamedMeta)
	}
	return renamedVideo, renamedMeta
}

// constructFinalPaths creates and validates the final file paths.
func (fp *fileProcessor) constructFinalPaths(renamedVideo, renamedMeta, vidExt, metaDir, metaExt string) (err error) {
	renamedVPath := filepath.Join(fp.fd.VideoDirectory, renamedVideo+vidExt)
	renamedMPath := filepath.Join(metaDir, renamedMeta+metaExt)

	logging.D(1, "Final paths with extensions:\nVideo: %s\nMeta: %s", renamedVPath, renamedMPath)

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

// constructNewNames constructs the new file names.
func (fp *fileProcessor) constructNewNames(fileBase string, style enums.ReplaceToStyle, fd *models.FileData) string {
	logging.D(2, "Processing metafile base name: %q", fileBase)
	fOps := fd.FilenameOps

	replacements := fOps.Replaces
	suffixes := fOps.ReplaceSuffixes
	prefixes := fOps.ReplacePrefixes
	appends := fOps.Appends
	prefixOps := fOps.Prefixes
	dateTag := fOps.DateTag
	deleteDateTags := fOps.DeleteDateTags
	set := fOps.Set

	// Check if any renaming to do
	if len(suffixes) == 0 && len(prefixes) == 0 && len(replacements) == 0 &&
		len(appends) == 0 && len(prefixOps) == 0 && dateTag.DateFormat == enums.DateFmtSkip &&
		deleteDateTags.DateFormat == enums.DateFmtSkip && style == enums.RenamingSkip &&
		!set.IsSet {
		logging.D(1, "No filename operations or naming style to apply")
		return fileBase
	}

	// Delete date tags first (if configured)
	if deleteDateTags.DateFormat != enums.DateFmtSkip {
		fileBase = fp.deleteDateTag(fileBase, deleteDateTags)
	}

	// Set strings
	if set.IsSet {
		fileBase = fp.setString(fileBase, set)
	}

	// Make string replacements
	if len(replacements) > 0 {
		fileBase = fp.replaceStrings(fileBase, replacements)
	}

	// Replace/trim prefixes
	if len(prefixes) > 0 {
		fileBase = fp.replacePrefix(fileBase, prefixes)
	}

	// Replace/trim suffixes
	if len(suffixes) > 0 {
		fileBase = fp.replaceSuffix(fileBase, suffixes)
	}

	// Add prefixes
	if len(prefixOps) > 0 {
		fileBase = fp.prefixStrings(fileBase, prefixOps)
	}

	// Add appends
	if len(appends) > 0 {
		fileBase = fp.appendStrings(fileBase, appends)
	}

	// Add date tag
	if fd.FilenameDateTag != "" {
		if !strings.Contains(fileBase, fd.FilenameDateTag) {
			fileBase = fp.addDateTag(fileBase, dateTag, fd.FilenameDateTag)
		} else {
			logging.D(2, "Date tag already present in filename, skipping addition")
		}
	}

	// Apply naming style
	if style != enums.RenamingSkip {
		fileBase = applyNamingStyle(style, fileBase)
	} else {
		logging.D(1, "No naming style selected, skipping rename style")
	}
	return fileBase
}
