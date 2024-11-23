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
	"sync"
)

// fileProcessor handles the renaming and moving of files
type fileProcessor struct {
	fd         *models.FileData
	style      enums.ReplaceToStyle
	skipVideos bool
}

// FileRename formats the file names
func FileRename(dataArray []*models.FileData, style enums.ReplaceToStyle, skipVideos bool) error {
	var wg sync.WaitGroup
	conc := cfg.GetInt(keys.Concurrency)

	sem := make(chan struct{}, conc)
	errChan := make(chan error, len(dataArray))

	for i := range dataArray {
		wg.Add(1)
		go func(fileData *models.FileData) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			fp := &fileProcessor{
				fd:         fileData,
				style:      style,
				skipVideos: skipVideos,
			}

			if err := fp.process(); err != nil {
				errChan <- fmt.Errorf("error processing %s: %w", fileData.OriginalVideoBaseName, err)
			}
		}(dataArray[i])
	}

	wg.Wait()
	close(errChan)

	var errors []error
	for err := range errChan {
		errors = append(errors, err)
	}

	if len(errors) > 0 {
		return fmt.Errorf("encountered %d errors during rename: %v", len(errors), errors)
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

	fsWriter, err := writefs.NewFSFileWriter(fp.fd, fp.skipVideos)
	if err != nil {
		return err
	}

	if err := fsWriter.WriteResults(); err != nil {
		return err
	}

	if cfg.IsSet(keys.MetaPurge) {
		if err, deletedMeta = fsWriter.DeleteMetafile(fp.fd.JSONFilePath); err != nil {
			return fmt.Errorf("failed to purge metafile: %v", err)
		}
	}

	if cfg.IsSet(keys.MoveOnComplete) {
		if err := fsWriter.MoveFile(deletedMeta); err != nil {
			return fmt.Errorf("failed to move to destination folder: %v", err)
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
	if renamedVideo, renamedMeta, err = fixContractions(renamedVideo, renamedMeta, fp.style); err != nil {
		return fmt.Errorf("failed to fix contractions for %s. error: %v", renamedVideo, err)
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
	if !cfg.IsSet(keys.OutputFiletype) {
		return filepath.Ext(originalPath)
	}

	vidExt := validate.ValidateExtension(cfg.GetString(keys.OutputFiletype))
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

// constructNewNames constructs the new file names.
func constructNewNames(fileBase string, style enums.ReplaceToStyle, fd *models.FileData) string {
	logging.D(2, "Processing metafile base name: %q", fileBase)

	var (
		suffixes []models.FilenameReplaceSuffix
		ok       bool
	)

	if len(fd.ModelFileSfxReplace) > 0 {
		suffixes = fd.ModelFileSfxReplace
	} else if cfg.IsSet(keys.FilenameReplaceSfx) {
		suffixes, ok = cfg.Get(keys.FilenameReplaceSfx).([]models.FilenameReplaceSuffix)
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
