// Package file handles operations related to reading and writing files.
package file

import (
	"fmt"
	"metarr/internal/abstractions"
	"metarr/internal/domain/consts"
	"metarr/internal/domain/keys"
	"metarr/internal/domain/logger"
	"metarr/internal/domain/lookupmaps"
	"metarr/internal/models"
	"metarr/internal/parsing"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// InitFetchFilesVars sets up the cached variables to be used in file fetching ops.
func InitFetchFilesVars() (err error) {
	// Handle video extension input.
	inVExts := abstractions.GetStringSlice(keys.InputVExts)
	inMExts := abstractions.GetStringSlice(keys.InputMExts)

	// Check slice for "all".
	allV := false
	allM := false
	if slices.Contains(inVExts, "all") {
		allV = true
	}
	if slices.Contains(inMExts, "all") {
		allM = true
	}

	// Set video map.
	for k := range lookupmaps.AllVidExtensions {
		// Set all true.
		if allV {
			lookupmaps.AllVidExtensions[k] = true
			continue
		}
		// Selective set.
		for _, ve := range inVExts {
			if k == ve {
				lookupmaps.AllVidExtensions[k] = true
			}
		}
	}

	// Set meta map.
	for k := range lookupmaps.AllMetaExtensions {
		// Set all true.
		if allM {
			lookupmaps.AllMetaExtensions[k] = true
			continue
		}
		// Selective set.
		for _, me := range inMExts {
			if k == me {
				lookupmaps.AllMetaExtensions[k] = true
			}
		}
	}
	return nil
}

// GetVideoFiles fetches video files from a directory.
func GetVideoFiles(videoDir *os.File) (map[string]*models.FileData, error) {
	files, err := videoDir.ReadDir(-1)
	if err != nil {
		return nil, fmt.Errorf("error reading video directory %q: %w", videoDir.Name(), err)
	}
	logger.Pl.I("Filtering video directory %q:\nFile extensions: %v\n\n", videoDir.Name(), lookupmaps.AllVidExtensions)

	// Iterate over video files in directory.
	videoFiles := make(map[string]*models.FileData, len(files))
	for _, file := range files {
		// Text filters
		if abstractions.IsSet(keys.FilePrefixes) {
			if !matchesFilenameFilter(file.Name(), abstractions.GetStringSlice(keys.FilePrefixes), strings.HasPrefix) {
				continue
			}
		}
		if abstractions.IsSet(keys.FileSuffixes) {
			if !matchesFilenameFilter(file.Name(), abstractions.GetStringSlice(keys.FileSuffixes), strings.HasSuffix) {
				continue
			}
		}
		if abstractions.IsSet(keys.FileContains) {
			if !matchesFilenameFilter(file.Name(), abstractions.GetStringSlice(keys.FileContains), strings.Contains) {
				continue
			}
		}
		if abstractions.IsSet(keys.FileOmits) {
			if matchesFilenameFilter(file.Name(), abstractions.GetStringSlice(keys.FileOmits), strings.Contains) {
				continue
			}
		}

		// Other checks (is not a directory, has a video extension, is not a Metarr backup).
		if !file.IsDir() && hasFileExtension(file.Name(), lookupmaps.AllVidExtensions) {
			videoFilenameBase := filepath.Base(file.Name())

			m := models.NewFileData()

			m.OriginalVideoPath = filepath.Join(videoDir.Name(), file.Name())
			m.VideoDirectory = videoDir.Name()

			if !strings.Contains(parsing.GetBaseNameWithoutExt(m.OriginalVideoPath), consts.BackupTag) {
				videoFiles[file.Name()] = m
				logger.Pl.I("Added video to queue: %v", videoFilenameBase)
			} else {
				logger.Pl.I("Skipping file %q containing backup tag (%q)", m.OriginalVideoPath, consts.BackupTag)
			}
		}
	}
	if len(videoFiles) == 0 {
		return nil, fmt.Errorf("no video files with extensions: %v or matching file filters found in directory: %s", lookupmaps.AllVidExtensions, videoDir.Name())
	}
	return videoFiles, nil
}

// GetMetadataFiles fetches metadata files from a directory.
func GetMetadataFiles(metaDir *os.File) (map[string]*models.FileData, error) {
	files, err := metaDir.ReadDir(-1)
	if err != nil {
		return nil, fmt.Errorf("error reading metadata directory %q: %w", metaDir.Name(), err)
	}
	logger.Pl.I("Filtering video directory %q:\nFile extensions: %v\n\n", metaDir.Name(), lookupmaps.AllMetaExtensions)

	// Iterate over metadata files in directory.
	metaFiles := make(map[string]*models.FileData, len(files))
	for _, file := range files {
		ext := filepath.Ext(file.Name())
		logger.Pl.D(3, "Checking file %q with extension %q", file.Name(), ext)

		// Text filters.
		if abstractions.IsSet(keys.FilePrefixes) {
			if !matchesFilenameFilter(file.Name(), abstractions.GetStringSlice(keys.FilePrefixes), strings.HasPrefix) {
				continue
			}
		}
		if abstractions.IsSet(keys.FileSuffixes) {
			if !matchesFilenameFilter(file.Name(), abstractions.GetStringSlice(keys.FileSuffixes), strings.HasSuffix) {
				continue
			}
		}
		if abstractions.IsSet(keys.FileContains) {
			if !matchesFilenameFilter(file.Name(), abstractions.GetStringSlice(keys.FileContains), strings.Contains) {
				continue
			}
		}
		if abstractions.IsSet(keys.FileOmits) {
			if matchesFilenameFilter(file.Name(), abstractions.GetStringSlice(keys.FileOmits), strings.Contains) {
				continue
			}
		}

		// File is a directory or does not have meta extensions.
		if file.IsDir() || !lookupmaps.AllMetaExtensions[ext] {
			continue
		}

		// Check extensions.
		m := models.NewFileData()
		metaFilenameBase := filepath.Base(file.Name())
		metaBaseName := parsing.GetBaseNameWithoutExt(metaFilenameBase)

		filePath := filepath.Join(metaDir.Name(), file.Name())

		// Check if valid metafile is present.
		for k := range lookupmaps.AllMetaExtensions {
			if ext == k {
				logger.Pl.D(1, "Detected %s file %q", strings.ToUpper(ext), file.Name())
				m.MetaFilePath = filePath
				m.MetaDirectory = metaDir.Name()
				m.MetaFileType = ext
			}
		}

		// Skip if it's a Metarr-generated backup file.
		if !strings.Contains(metaBaseName, consts.BackupTag) {
			metaFiles[file.Name()] = m
		} else {
			logger.Pl.I("Skipping file %q containing backup tag (%q)", metaBaseName, consts.BackupTag)
		}
	}
	if len(metaFiles) == 0 {
		return nil, fmt.Errorf("no meta files with extensions: %v or matching file filters found in directory: %s", lookupmaps.AllMetaExtensions, metaDir.Name())
	}
	logger.Pl.D(3, "Returning meta files %v", metaFiles)
	return metaFiles, nil
}

// GetSingleVideoFile handles a single video file.
func GetSingleVideoFile(videoFile *os.File) (map[string]*models.FileData, error) {
	videoMap := make(map[string]*models.FileData, 1)
	videoBaseFilename := filepath.Base(videoFile.Name())

	videoData := models.NewFileData()
	videoData.OriginalVideoPath = videoFile.Name()
	videoData.VideoDirectory = filepath.Dir(videoFile.Name())

	logger.Pl.D(3, "Created video file data for single file: %s", videoBaseFilename)
	videoMap[videoBaseFilename] = videoData
	return videoMap, nil
}

// GetSingleMetadataFile handles a single metadata file.
func GetSingleMetadataFile(metaFile *os.File) (map[string]*models.FileData, error) {
	metaMap := make(map[string]*models.FileData, 1)
	metaBaseFilename := filepath.Base(metaFile.Name())

	m := models.NewFileData()
	ext := filepath.Ext(metaFile.Name())
	filename := metaFile.Name()
	dir := filepath.Dir(metaFile.Name())

	// Check if valid metafile is present.
	for k := range lookupmaps.AllMetaExtensions {
		if ext == k {
			logger.Pl.D(1, "Detected %s file %q", strings.ToUpper(ext), metaFile.Name())
			m.MetaFilePath = filename
			m.MetaDirectory = dir
			m.MetaFileType = ext
		}
	}

	metaMap[metaBaseFilename] = m
	return metaMap, nil
}

// MatchVideoWithMetadata matches video files with their corresponding metadata files.
func MatchVideoWithMetadata(videoFiles, metaFiles map[string]*models.FileData, batchID int64) (map[string]*models.FileData, error) {
	logger.Pl.D(3, "Entering metadata and video file matching loop...")

	// Pre-process metaFiles into a lookup map.
	metaLookup := make(map[string]*models.FileData, len(metaFiles))
	for metaFilename, metaFileData := range metaFiles {
		baseKey := NormalizeFilename(TrimMetafileSuffixes(metaFilename, ""))
		metaLookup[baseKey] = metaFileData
	}

	// Find metadata file matches for video files.
	matchedFiles := make(map[string]*models.FileData, len(videoFiles))
	for videoFilename := range videoFiles {
		videoData := videoFiles[videoFilename]
		if videoData == nil {
			logger.Pl.W("Skipping nil video file entry: %s", videoFilename)
			continue
		}
		videoBase := parsing.GetBaseNameWithoutExt(videoFilename)
		normalizedVideoBase := NormalizeFilename(videoBase)

		if fileData, exists := metaLookup[normalizedVideoBase]; exists && fileData != nil { // This checks if the key exists in the metaLookup map.
			matchedFiles[videoFilename] = videoData
			matchedFiles[videoFilename].MetaFilePath = fileData.MetaFilePath
			matchedFiles[videoFilename].MetaDirectory = fileData.MetaDirectory
			matchedFiles[videoFilename].MetaFileType = fileData.MetaFileType
		}
	}
	if len(matchedFiles) == 0 {
		return nil, fmt.Errorf("no matching metadata files found for any videos (batch ID: %d)", batchID)
	}
	return matchedFiles, nil
}
