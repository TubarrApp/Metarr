// Package fsread handles filesystem reads.
package fsread

import (
	"fmt"
	"metarr/internal/cfg"
	"metarr/internal/domain/consts"
	"metarr/internal/domain/enums"
	"metarr/internal/domain/keys"
	"metarr/internal/domain/regex"
	"metarr/internal/models"
	"metarr/internal/utils/logging"
	"os"
	"path/filepath"
	"strings"
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
	inVExts, ok := cfg.Get(keys.InputVExtsEnum).([]enums.ConvertFromFiletype)
	if !ok {
		return fmt.Errorf("wrong type sent in. Received type %T", inVExts)
	}

	if videoExtensions, err = setVideoExtensions(inVExts); err != nil {
		return err
	}

	// Handle meta extension input
	inMExts, ok := cfg.Get(keys.InputMExtsEnum).([]enums.MetaFiletypeFilter)
	if !ok {
		return fmt.Errorf("wrong type sent in. Received type %T", inMExts)
	}

	if metaExtensions, err = setMetaExtensions(inMExts); err != nil {
		return err
	}

	// Set prefix filter
	inputPrefixes = SetPrefixFilter(cfg.GetStringSlice(keys.FilePrefixes))
	logging.D(2, "Setting prefix filter: %v", inputPrefixes)

	return nil
}

// GetVideoFiles fetches video files from a directory.
func GetVideoFiles(videoDir *os.File) (map[string]*models.FileData, error) {
	files, err := videoDir.ReadDir(-1)
	if err != nil {
		return nil, fmt.Errorf("error reading video directory %q: %w", videoDir.Name(), err)
	}

	logging.P("\n\nFiltering directory %q:\n\nFile extensions: %v\nFile prefixes: %v\n\n", videoDir.Name(), videoExtensions, inputPrefixes)

	videoFiles := make(map[string]*models.FileData, len(files))

	for _, file := range files {

		if cfg.IsSet(keys.FilePrefixes) {
			if !HasPrefix(file.Name(), inputPrefixes) {
				continue
			}
		}

		if !file.IsDir() && HasFileExtension(file.Name(), videoExtensions) {

			videoFilenameBase := filepath.Base(file.Name())

			m := models.NewFileData()
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
func GetMetadataFiles(metaDir *os.File) (map[string]*models.FileData, error) {
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

		if cfg.IsSet(keys.FilePrefixes) {
			if !HasPrefix(file.Name(), inputPrefixes) {
				continue
			}
		}

		metaFilenameBase := filepath.Base(file.Name())
		baseName := strings.TrimSuffix(metaFilenameBase, ext)

		m := models.NewFileData()
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
func GetSingleVideoFile(videoFile *os.File) (map[string]*models.FileData, error) {
	videoMap := make(map[string]*models.FileData, 1)
	videoFilename := filepath.Base(videoFile.Name())

	videoData := models.NewFileData()
	videoData.OriginalVideoPath = videoFile.Name()
	videoData.OriginalVideoBaseName = strings.TrimSuffix(videoFilename, filepath.Ext(videoFilename))
	videoData.VideoDirectory = filepath.Dir(videoFile.Name())

	logging.D(3, "Created video file data for single file: %s", videoFilename)

	videoMap[videoFilename] = videoData
	return videoMap, nil
}

// GetSingleMetadataFile handles a single metadata file.
func GetSingleMetadataFile(metaFile *os.File) (map[string]*models.FileData, error) {
	metaMap := make(map[string]*models.FileData, 1)
	videoFilename := filepath.Base(metaFile.Name())

	fileData := models.NewFileData()
	ext := filepath.Ext(metaFile.Name())

	switch ext {
	case consts.MExtJSON:

		fileData.MetaFileType = enums.MetaFiletypeJSON
		fileData.JSONFilePath = metaFile.Name()
		fileData.JSONBaseName = strings.TrimSuffix(videoFilename, ext)
		fileData.JSONDirectory = filepath.Dir(metaFile.Name())
		logging.D(3, "Created JSON metadata file data for single file: %s", videoFilename)

	case consts.MExtNFO:

		fileData.MetaFileType = enums.MetaFiletypeNFO
		fileData.NFOFilePath = metaFile.Name()
		fileData.NFOBaseName = strings.TrimSuffix(videoFilename, ext)
		fileData.NFODirectory = filepath.Dir(metaFile.Name())
		logging.D(3, "Created NFO metadata file data for single file: %s", videoFilename)

	default:
		return nil, fmt.Errorf("unsupported metadata file type: %s", ext)
	}

	metaMap[videoFilename] = fileData
	return metaMap, nil
}

// MatchVideoWithMetadata matches video files with their corresponding metadata files
func MatchVideoWithMetadata(videoFiles, metaFiles map[string]*models.FileData, batchID int64) (map[string]*models.FileData, error) {
	logging.D(3, "Entering metadata and video file matching loop...")

	matchedFiles := make(map[string]*models.FileData, len(videoFiles))

	specialChars := regex.SpecialCharsCompile()
	extraSpaces := regex.ExtraSpacesCompile()

	// Pre-process metaFiles into a lookup map
	metaLookup := make(map[string]*models.FileData, len(metaFiles))
	for metaName, metaData := range metaFiles {
		baseKey := NormalizeFilename(TrimMetafileSuffixes(metaName, ""), specialChars, extraSpaces)
		metaLookup[baseKey] = metaData
	}

	for videoFilename := range videoFiles {
		videoBase := strings.TrimSuffix(videoFilename, filepath.Ext(videoFilename))
		normalizedVideoBase := NormalizeFilename(videoBase, specialChars, extraSpaces)

		if metaData, exists := metaLookup[normalizedVideoBase]; exists { // This checks if the key exists in the metaLookup map
			matchedFiles[videoFilename] = videoFiles[videoFilename]
			matchedFiles[videoFilename].MetaFileType = metaData.MetaFileType

			switch metaData.MetaFileType {
			case enums.MetaFiletypeJSON:
				matchedFiles[videoFilename].JSONFilePath = metaData.JSONFilePath
				matchedFiles[videoFilename].JSONBaseName = metaData.JSONBaseName
				matchedFiles[videoFilename].JSONDirectory = metaData.JSONDirectory

			case enums.MetaFiletypeNFO:
				matchedFiles[videoFilename].NFOFilePath = metaData.NFOFilePath
				matchedFiles[videoFilename].NFOBaseName = metaData.NFOBaseName
				matchedFiles[videoFilename].NFODirectory = metaData.NFODirectory
			}
		}
	}

	if len(matchedFiles) == 0 {
		return nil, fmt.Errorf("no matching metadata files found for any videos (batch ID: %d)", batchID)
	}

	return matchedFiles, nil
}
