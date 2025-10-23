// Package fsread handles filesystem reads.
package fsread

import (
	"fmt"
	"metarr/internal/domain/consts"
	"metarr/internal/domain/enums"
	"metarr/internal/domain/keys"
	"metarr/internal/domain/regex"
	"metarr/internal/models"
	"metarr/internal/utils/logging"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/viper"
)

// Variable cache
var (
	inputPrefixes []string

	videoExtensions,
	metaExtensions map[string]bool
)

// InitFetchFilesVars sets up the cached variables to be used in file fetching ops.
func InitFetchFilesVars() (err error) {
	// Handle video extension input
	inVExts, ok := viper.Get(keys.InputVExtsEnum).([]enums.ConvertFromFiletype)
	if !ok {
		return fmt.Errorf("wrong type sent in. Received type %T", inVExts)
	}

	if videoExtensions, err = setVideoExtensions(inVExts); err != nil {
		return err
	}

	// Handle meta extension input
	inMExts, ok := viper.Get(keys.InputMExtsEnum).([]enums.MetaFiletypeFilter)
	if !ok {
		return fmt.Errorf("wrong type sent in. Received type %T", inMExts)
	}

	if metaExtensions, err = setMetaExtensions(inMExts); err != nil {
		return err
	}

	// Set prefix filter
	inputPrefixes = SetPrefixFilter(viper.GetStringSlice(keys.FilePrefixes))
	logging.D(2, "Setting prefix filter: %v", inputPrefixes)

	return nil
}

// GetVideoFiles fetches video files from a directory.
func GetVideoFiles(videoDir *os.File, metaOps *models.MetaOps) (map[string]*models.FileData, error) {
	files, err := videoDir.ReadDir(-1)
	if err != nil {
		return nil, fmt.Errorf("error reading video directory %q: %w", videoDir.Name(), err)
	}

	logging.P("\n\nFiltering directory %q:\n\nFile extensions: %v\nFile prefixes: %v\n\n", videoDir.Name(), videoExtensions, inputPrefixes)

	videoFiles := make(map[string]*models.FileData, len(files))

	for _, file := range files {

		if viper.IsSet(keys.FilePrefixes) {
			if !HasPrefix(file.Name(), inputPrefixes) {
				continue
			}
		}

		if !file.IsDir() && HasFileExtension(file.Name(), videoExtensions) {

			videoFilenameBase := filepath.Base(file.Name())

			m := models.NewFileData()
			m.MetaOps = models.EnsureMetaOps(metaOps)

			m.OriginalVideoPath = filepath.Join(videoDir.Name(), file.Name())
			m.OriginalVideoBaseName = strings.TrimSuffix(videoFilenameBase, filepath.Ext(file.Name()))
			m.VideoDirectory = videoDir.Name()

			if !strings.HasSuffix(m.OriginalVideoBaseName, consts.BackupTag) {
				videoFiles[file.Name()] = m
				logging.I("Added video to queue: %v", videoFilenameBase)
			} else {
				logging.I("Skipping file %q containing backup tag (%q)", m.OriginalVideoBaseName, consts.BackupTag)
			}
		}
	}

	if len(videoFiles) == 0 {
		return nil, fmt.Errorf("no video files with extensions: %v and prefixes: %v found in directory: %s", videoExtensions, inputPrefixes, videoDir.Name())
	}
	return videoFiles, nil
}

// GetMetadataFiles fetches metadata files from a directory.
func GetMetadataFiles(metaDir *os.File, metaOps *models.MetaOps) (map[string]*models.FileData, error) {
	files, err := metaDir.ReadDir(-1)
	if err != nil {
		return nil, fmt.Errorf("error reading metadata directory %q: %w", metaDir.Name(), err)
	}

	metaFiles := make(map[string]*models.FileData, len(files))

	for _, file := range files {
		ext := filepath.Ext(file.Name())
		logging.D(3, "Checking file %q with extension %q", file.Name(), ext)

		if file.IsDir() || !metaExtensions[ext] {
			continue
		}

		if viper.IsSet(keys.FilePrefixes) {
			if !HasPrefix(file.Name(), inputPrefixes) {
				continue
			}
		}

		metaFilenameBase := filepath.Base(file.Name())
		baseName := strings.TrimSuffix(metaFilenameBase, ext)

		m := models.NewFileData()
		m.MetaOps = models.EnsureMetaOps(metaOps)

		filePath := filepath.Join(metaDir.Name(), file.Name())

		switch ext {
		case consts.MExtJSON:

			logging.D(1, "Detected JSON file %q", file.Name())
			m.JSONFilePath = filePath
			m.JSONBaseName = baseName
			m.JSONDirectory = metaDir.Name()
			m.MetaFileType = enums.MetaFiletypeJSON

		case consts.MExtNFO:

			logging.D(1, "Detected NFO file %q", file.Name())
			m.NFOFilePath = filePath
			m.NFOBaseName = baseName
			m.NFODirectory = metaDir.Name()
			m.MetaFileType = enums.MetaFiletypeNFO
		}

		if !strings.Contains(baseName, consts.BackupTag) {
			metaFiles[file.Name()] = m
		} else {
			logging.I("Skipping file %q containing backup tag (%q)", baseName, consts.BackupTag)
		}
	}

	if len(metaFiles) == 0 {
		return nil, fmt.Errorf("no meta files with extensions: %v and prefixes: %v found in directory: %s", metaExtensions, inputPrefixes, metaDir.Name())
	}

	logging.D(3, "Returning meta files %v", metaFiles)
	return metaFiles, nil
}

// GetSingleVideoFile handles a single video file.
func GetSingleVideoFile(videoFile *os.File, metaOps *models.MetaOps) (map[string]*models.FileData, error) {
	videoMap := make(map[string]*models.FileData, 1)
	videoFilename := filepath.Base(videoFile.Name())

	videoData := models.NewFileData()
	videoData.MetaOps = models.EnsureMetaOps(metaOps)

	videoData.OriginalVideoPath = videoFile.Name()
	videoData.OriginalVideoBaseName = strings.TrimSuffix(videoFilename, filepath.Ext(videoFilename))
	videoData.VideoDirectory = filepath.Dir(videoFile.Name())

	logging.D(3, "Created video file data for single file: %s", videoFilename)

	videoMap[videoFilename] = videoData
	return videoMap, nil
}

// GetSingleMetadataFile handles a single metadata file.
func GetSingleMetadataFile(metaFile *os.File, metaOps *models.MetaOps) (map[string]*models.FileData, error) {
	metaMap := make(map[string]*models.FileData, 1)
	videoFilename := filepath.Base(metaFile.Name())

	metaFileData := models.NewFileData()
	metaFileData.MetaOps = models.EnsureMetaOps(metaOps)

	ext := filepath.Ext(metaFile.Name())

	switch ext {
	case consts.MExtJSON:

		metaFileData.MetaFileType = enums.MetaFiletypeJSON
		metaFileData.JSONFilePath = metaFile.Name()
		metaFileData.JSONBaseName = strings.TrimSuffix(videoFilename, ext)
		metaFileData.JSONDirectory = filepath.Dir(metaFile.Name())
		logging.D(3, "Created JSON metadata file data for single file: %s", videoFilename)

	case consts.MExtNFO:

		metaFileData.MetaFileType = enums.MetaFiletypeNFO
		metaFileData.NFOFilePath = metaFile.Name()
		metaFileData.NFOBaseName = strings.TrimSuffix(videoFilename, ext)
		metaFileData.NFODirectory = filepath.Dir(metaFile.Name())
		logging.D(3, "Created NFO metadata file data for single file: %s", videoFilename)

	default:
		return nil, fmt.Errorf("unsupported metadata file type: %s", ext)
	}

	metaMap[videoFilename] = metaFileData
	return metaMap, nil
}

// MatchVideoWithMetadata matches video files with their corresponding metadata files
func MatchVideoWithMetadata(videoFiles, metaFiles map[string]*models.FileData, batchID int64) (map[string]*models.FileData, error) {
	logging.D(3, "Entering metadata and video file matching loop...")

	specialChars := regex.SpecialCharsCompile()
	extraSpaces := regex.ExtraSpacesCompile()

	// Pre-process metaFiles into a lookup map
	metaLookup := make(map[string]*models.FileData, len(metaFiles))
	for metaName, metaData := range metaFiles {
		baseKey := NormalizeFilename(TrimMetafileSuffixes(metaName, ""), specialChars, extraSpaces)
		metaLookup[baseKey] = metaData
	}

	// Find metadata file matches for video files
	matchedFiles := make(map[string]*models.FileData, len(videoFiles))
	for videoFilename := range videoFiles {
		videoData := videoFiles[videoFilename]
		if videoData == nil {
			logging.W("Skipping nil video file entry: %s", videoFilename)
			continue
		}

		videoBase := strings.TrimSuffix(videoFilename, filepath.Ext(videoFilename))
		normalizedVideoBase := NormalizeFilename(videoBase, specialChars, extraSpaces)

		if fileData, exists := metaLookup[normalizedVideoBase]; exists && fileData != nil { // This checks if the key exists in the metaLookup map
			matchedFiles[videoFilename] = videoData
			matchedFiles[videoFilename].MetaFileType = fileData.MetaFileType

			// Type of metadata file
			switch fileData.MetaFileType {
			case enums.MetaFiletypeJSON: // JSON
				matchedFiles[videoFilename].JSONFilePath = fileData.JSONFilePath
				matchedFiles[videoFilename].JSONBaseName = fileData.JSONBaseName
				matchedFiles[videoFilename].JSONDirectory = fileData.JSONDirectory

			case enums.MetaFiletypeNFO: // NFO
				matchedFiles[videoFilename].NFOFilePath = fileData.NFOFilePath
				matchedFiles[videoFilename].NFOBaseName = fileData.NFOBaseName
				matchedFiles[videoFilename].NFODirectory = fileData.NFODirectory
			}
		}
	}

	if len(matchedFiles) == 0 {
		return nil, fmt.Errorf("no matching metadata files found for any videos (batch ID: %d)", batchID)
	}

	return matchedFiles, nil
}
