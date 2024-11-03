package utils

import (
	"Metarr/internal/config"
	consts "Metarr/internal/domain/constants"
	enums "Metarr/internal/domain/enums"
	keys "Metarr/internal/domain/keys"
	"Metarr/internal/types"
	logging "Metarr/internal/utils/logging"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// Variable cache
var (
	videoExtensions,
	inputPrefixes []string
	metaExtensions = []string{".json", ".nfo"}
)

// InitFetchFilesVars sets up the cached variables to be used in file fetching ops
func InitFetchFilesVars() error {

	if inExts, ok := config.Get(keys.InputExtsEnum).([]enums.ConvertFromFiletype); ok {
		videoExtensions = SetExtensions(inExts)
	} else {
		return fmt.Errorf("wrong type sent in. Received type %T", inExts)
	}

	inputPrefixes = SetPrefixFilter(config.GetStringSlice(keys.FilePrefixes))
	metaExtensions = []string{".json", ".nfo"}

	return nil
}

// GetVideoFiles fetches video files from a directory
func GetVideoFiles(videoDir *os.File) (map[string]*types.FileData, error) {
	files, err := videoDir.ReadDir(-1)
	if err != nil {
		return nil, fmt.Errorf("error reading video directory: %w", err)
	}

	logging.Print("\n\nFiltering directory '%s':\n\nFile extensions: %v\nFile prefixes: %v\n\n", videoDir.Name(), videoExtensions, inputPrefixes)

	videoFiles := make(map[string]*types.FileData, len(files))

	for _, file := range files {
		if !file.IsDir() && HasFileExtension(file.Name(), videoExtensions) && HasPrefix(file.Name(), inputPrefixes) {
			filenameBase := filepath.Base(file.Name())

			m := types.NewFileData()
			m.OriginalVideoPath = filepath.Join(videoDir.Name(), file.Name())
			m.OriginalVideoBaseName = strings.TrimSuffix(filenameBase, filepath.Ext(file.Name()))
			m.VideoDirectory = videoDir.Name()

			if !strings.HasSuffix(m.OriginalVideoBaseName, consts.OldTag) {
				videoFiles[file.Name()] = m
				logging.PrintI("Added video to queue: %v", filenameBase)
			} else {
				logging.PrintI("Skipping file '%s' containing backup tag ('%s')", m.OriginalVideoBaseName, consts.OldTag)
			}
		}
	}

	if len(videoFiles) == 0 {
		return nil, fmt.Errorf("no video files with extensions: %v and prefixes: %v found in directory: %s", videoExtensions, inputPrefixes, videoDir.Name())
	}
	return videoFiles, nil
}

// GetMetadataFiles fetches metadata files from a directory
func GetMetadataFiles(metaDir *os.File) (map[string]*types.FileData, error) {
	files, err := metaDir.ReadDir(-1)
	if err != nil {
		return nil, fmt.Errorf("error reading metadata directory: %w", err)
	}

	metaFiles := make(map[string]*types.FileData, len(files))

	for _, file := range files {
		if !file.IsDir() && HasPrefix(file.Name(), inputPrefixes) {
			ext := filepath.Ext(file.Name())
			if ext != ".json" && ext != ".nfo" {
				continue
			}

			filenameBase := filepath.Base(file.Name())
			baseName := strings.TrimSuffix(filenameBase, ext)

			m := types.NewFileData()
			filePath := filepath.Join(metaDir.Name(), file.Name())

			switch ext {
			case ".json":
				logging.PrintD(1, "Detected JSON file '%s'", file.Name())
				m.JSONFilePath = filePath
				m.JSONBaseName = baseName
				m.JSONDirectory = metaDir.Name()
				m.MetaFileType = enums.METAFILE_JSON

			case ".nfo":
				logging.PrintD(1, "Detected NFO file '%s'", file.Name())
				m.NFOFilePath = filePath
				m.NFOBaseName = baseName
				m.NFODirectory = metaDir.Name()
				m.MetaFileType = enums.METAFILE_NFO
			}

			if !strings.Contains(baseName, consts.OldTag) {
				metaFiles[file.Name()] = m
			} else {
				logging.PrintI("Skipping file '%s' containing backup tag ('%s')", baseName, consts.OldTag)
			}
		}
	}

	if len(metaFiles) == 0 {
		return nil, fmt.Errorf("no meta files with extensions: %v and prefixes: %v found in directory: %s", metaExtensions, inputPrefixes, metaDir.Name())
	}

	logging.PrintD(3, "Returning meta files %v", metaFiles)
	return metaFiles, nil
}

// GetSingleVideoFile handles a single video file
func GetSingleVideoFile(videoFile *os.File) (map[string]*types.FileData, error) {
	videoMap := make(map[string]*types.FileData, 1)
	filename := filepath.Base(videoFile.Name())

	videoData := types.NewFileData()
	videoData.OriginalVideoPath = videoFile.Name()
	videoData.OriginalVideoBaseName = strings.TrimSuffix(filename, filepath.Ext(filename))
	videoData.VideoDirectory = filepath.Dir(videoFile.Name())
	videoData.VideoFile = videoFile

	logging.PrintD(3, "Created video file data for single file: %s", filename)

	videoMap[filename] = videoData
	return videoMap, nil
}

// GetSingleMetadataFile handles a single metadata file
func GetSingleMetadataFile(metaFile *os.File) (map[string]*types.FileData, error) {
	metaMap := make(map[string]*types.FileData, 1)
	filename := filepath.Base(metaFile.Name())

	fileData := types.NewFileData()
	ext := filepath.Ext(metaFile.Name())

	switch ext {
	case ".json":
		fileData.MetaFileType = enums.METAFILE_JSON
		fileData.JSONFilePath = metaFile.Name()
		fileData.JSONBaseName = strings.TrimSuffix(filename, ext)
		fileData.JSONDirectory = filepath.Dir(metaFile.Name())
		logging.PrintD(3, "Created JSON metadata file data for single file: %s", filename)

	case ".nfo":
		fileData.MetaFileType = enums.METAFILE_NFO
		fileData.NFOFilePath = metaFile.Name()
		fileData.NFOBaseName = strings.TrimSuffix(filename, ext)
		fileData.NFODirectory = filepath.Dir(metaFile.Name())
		logging.PrintD(3, "Created NFO metadata file data for single file: %s", filename)

	default:
		return nil, fmt.Errorf("unsupported metadata file type: %s", ext)
	}

	metaMap[filename] = fileData
	return metaMap, nil
}

// MatchVideoWithMetadata matches video files with their corresponding metadata files
func MatchVideoWithMetadata(videoFiles, metaFiles map[string]*types.FileData) (map[string]*types.FileData, error) {
	logging.PrintD(3, "Entering metadata and video file matching loop...")

	matchedFiles := make(map[string]*types.FileData, len(videoFiles))

	specialChars := regexp.MustCompile(`[^\w\s-]`)
	extraSpaces := regexp.MustCompile(`\s+`)

	// Pre-process metaFiles into a lookup map
	metaLookup := make(map[string]*types.FileData, len(metaFiles))
	for metaName, metaData := range metaFiles {
		baseKey := NormalizeFilename(TrimMetafileSuffixes(metaName, ""), specialChars, extraSpaces)
		metaLookup[baseKey] = metaData
	}

	for videoName := range videoFiles {
		videoBase := strings.TrimSuffix(videoName, filepath.Ext(videoName))
		normalizedVideoBase := NormalizeFilename(videoBase, specialChars, extraSpaces)

		if metaData, exists := metaLookup[normalizedVideoBase]; exists { // This checks if the key exists in the metaLookup map
			matchedFiles[videoName] = videoFiles[videoName]
			matchedFiles[videoName].MetaFileType = metaData.MetaFileType

			switch metaData.MetaFileType {
			case enums.METAFILE_JSON:
				matchedFiles[videoName].JSONFilePath = metaData.JSONFilePath
				matchedFiles[videoName].JSONBaseName = metaData.JSONBaseName
				matchedFiles[videoName].JSONDirectory = metaData.JSONDirectory

			case enums.METAFILE_NFO:
				matchedFiles[videoName].NFOFilePath = metaData.NFOFilePath
				matchedFiles[videoName].NFOBaseName = metaData.NFOBaseName
				matchedFiles[videoName].NFODirectory = metaData.NFODirectory
			}
		}
	}

	if len(matchedFiles) == 0 {
		return nil, fmt.Errorf("no matching metadata files found for any videos")
	}

	return matchedFiles, nil
}
