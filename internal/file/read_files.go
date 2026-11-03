// Package file handles operations related to reading and writing files.
package file

import (
	"fmt"
	"maps"
	"metarr/internal/abstractions"
	"metarr/internal/domain/keys"
	"metarr/internal/domain/logger"
	"metarr/internal/models"
	"metarr/internal/parsing"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/TubarrApp/gocommon/sharedconsts"
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
	for k := range sharedconsts.FilterByVidExtensions {
		// Set all true.
		if allV {
			sharedconsts.FilterByVidExtensions[k] = true
			continue
		}
		// Selective set.
		for _, ve := range inVExts {
			if k == ve {
				sharedconsts.FilterByVidExtensions[k] = true
			}
		}
	}

	// Set meta map.
	for k := range sharedconsts.FilterByMetaExtension {
		// Set all true.
		if allM {
			sharedconsts.FilterByMetaExtension[k] = true
			continue
		}
		// Selective set.
		for _, me := range inMExts {
			if k == me {
				sharedconsts.FilterByMetaExtension[k] = true
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
	logger.Pl.I("Filtering video directory %q:\nFile extensions: %v\n\n", videoDir.Name(), sharedconsts.FilterByVidExtensions)

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
		if !file.IsDir() && hasFileExtension(file.Name(), sharedconsts.FilterByVidExtensions) {
			videoFilenameBase := filepath.Base(file.Name())

			m := models.NewFileData()

			m.OriginalVideoPath = filepath.Join(videoDir.Name(), file.Name())
			m.VideoDirectory = videoDir.Name()

			if !strings.Contains(parsing.GetBaseNameWithoutExt(m.OriginalVideoPath), sharedconsts.BackupTag) {
				videoFiles[file.Name()] = m
				logger.Pl.I("Added video to queue: %v", videoFilenameBase)
			} else {
				logger.Pl.I("Skipping file %q containing backup tag (%q)", m.OriginalVideoPath, sharedconsts.BackupTag)
			}
		}
	}
	if len(videoFiles) == 0 {
		return nil, fmt.Errorf("no video files with extensions: %v or matching file filters found in directory: %s", sharedconsts.FilterByVidExtensions, videoDir.Name())
	}
	return videoFiles, nil
}

// GetMetadataFiles fetches metadata files from a directory.
func GetMetadataFiles(metaDir *os.File) (map[string]*models.FileData, error) {
	files, err := metaDir.ReadDir(-1)
	if err != nil {
		return nil, fmt.Errorf("error reading metadata directory %q: %w", metaDir.Name(), err)
	}
	logger.Pl.I("Filtering video directory %q:\nFile extensions: %v\n\n", metaDir.Name(), sharedconsts.FilterByMetaExtension)

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
		if file.IsDir() || !sharedconsts.FilterByMetaExtension[ext] {
			continue
		}

		// Check extensions.
		m := models.NewFileData()
		metaFilenameBase := filepath.Base(file.Name())
		metaBaseName := parsing.GetBaseNameWithoutExt(metaFilenameBase)

		filePath := filepath.Join(metaDir.Name(), file.Name())

		// Check if valid metafile is present.
		for k := range sharedconsts.FilterByMetaExtension {
			if ext == k {
				logger.Pl.D(1, "Detected %s file %q", strings.ToUpper(ext), file.Name())
				m.MetaFilePath = filePath
				m.MetaDirectory = metaDir.Name()
				m.MetaFileType = strings.ToLower(ext)
			}
		}

		// Skip if it's a Metarr-generated backup file.
		if !strings.Contains(metaBaseName, sharedconsts.BackupTag) {
			metaFiles[file.Name()] = m
		} else {
			logger.Pl.I("Skipping file %q containing backup tag (%q)", metaBaseName, sharedconsts.BackupTag)
		}
	}
	if len(metaFiles) == 0 {
		return nil, fmt.Errorf("no meta files with extensions: %v or matching file filters found in directory: %s", sharedconsts.FilterByMetaExtension, metaDir.Name())
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
	for k := range sharedconsts.FilterByMetaExtension {
		if ext == k {
			logger.Pl.D(1, "Detected %s file %q", strings.ToUpper(ext), metaFile.Name())
			m.MetaFilePath = filename
			m.MetaDirectory = dir
			m.MetaFileType = strings.ToLower(ext)
		}
	}

	metaMap[metaBaseFilename] = m
	return metaMap, nil
}

// MatchVideoWithMetadata matches video files with their metadata files, closest names first, each metafile going to one video.
func MatchVideoWithMetadata(videoFiles, metaFiles map[string]*models.FileData, batchID int64) (map[string]*models.FileData, error) {
	logger.Pl.D(3, "Entering metadata and video file matching loop...")

	// Pre-process metaFiles into lookup maps by the video name they match, in name order so runs match alike.
	exact := make(map[string][]*models.FileData, len(metaFiles))      // The metafile's name, whole or without its extension.
	trimmed := make(map[string][]*models.FileData, len(metaFiles))    // Without a known suffix, e.g. ".info.json".
	normalized := make(map[string][]*models.FileData, len(metaFiles)) // Any of those, regardless of case and spacing.
	for _, metaFilename := range slices.Sorted(maps.Keys(metaFiles)) {
		metaFileData := metaFiles[metaFilename]
		metaBase := parsing.GetBaseNameWithoutExt(metaFilename)
		trimmedBase := TrimMetafileSuffixes(metaFilename, "")

		exact[metaFilename] = append(exact[metaFilename], metaFileData)
		exact[metaBase] = append(exact[metaBase], metaFileData)
		trimmed[trimmedBase] = append(trimmed[trimmedBase], metaFileData)
		for _, name := range []string{metaFilename, metaBase, trimmedBase} {
			normalized[NormalizeFilename(name)] = append(normalized[NormalizeFilename(name)], metaFileData)
		}
	}

	// Find metadata file matches for video files, closest matches first.
	videoNames := make([]string, 0, len(videoFiles))
	for _, videoFilename := range slices.Sorted(maps.Keys(videoFiles)) {
		if videoFiles[videoFilename] == nil {
			logger.Pl.W("Skipping nil video file entry: %s", videoFilename)
			continue
		}
		videoNames = append(videoNames, videoFilename)
	}
	matchedFiles := make(map[string]*models.FileData, len(videoFiles))
	claimed := make(map[*models.FileData]bool, len(metaFiles))
	for _, step := range []struct {
		lookup    map[string][]*models.FileData
		normalize bool
	}{{exact, false}, {trimmed, false}, {normalized, true}} {
		for _, videoFilename := range videoNames {
			if _, matched := matchedFiles[videoFilename]; matched {
				continue
			}
			videoData := videoFiles[videoFilename]
			if videoData == nil {
				continue
			}

			videoBase := parsing.GetBaseNameWithoutExt(videoFilename)
			if step.normalize {
				videoBase = NormalizeFilename(videoBase)
			}
			for _, fileData := range step.lookup[videoBase] {
				if fileData == nil || claimed[fileData] {
					continue
				}
				claimed[fileData] = true
				videoData.MetaFilePath = fileData.MetaFilePath
				videoData.MetaDirectory = fileData.MetaDirectory
				videoData.MetaFileType = fileData.MetaFileType
				matchedFiles[videoFilename] = videoData
				break
			}
		}
	}
	if len(matchedFiles) == 0 {
		return nil, fmt.Errorf("no matching metadata files found for any videos (batch ID: %d)", batchID)
	}
	return matchedFiles, nil
}
