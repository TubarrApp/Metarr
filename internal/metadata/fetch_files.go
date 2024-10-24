package metadata

import (
	"Metarr/internal/cmd"
	"Metarr/internal/consts"
	"Metarr/internal/enums"
	"Metarr/internal/keys"
	"Metarr/internal/logging"
	"Metarr/internal/models"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// GetVideoFiles fetches video files from a directory
func GetVideoFiles(videoDir *os.File) (map[string]*models.FileData, error) {
	files, err := videoDir.ReadDir(-1)
	if err != nil {
		return nil, fmt.Errorf("error reading video directory: %w", err)
	}

	convertFrom := cmd.Get(keys.InputExtsEnum).([]enums.ConvertFromFiletype)
	videoExtensions := setExtensions(convertFrom)
	inputPrefixFilters := cmd.GetStringSlice(keys.FilePrefixes)
	inputPrefixes := setPrefixFilter(inputPrefixFilters)

	fmt.Printf(`

Filtering directory: %s:

File extensions: %v
File prefixes: %v

`, videoDir.Name(),
		videoExtensions,
		inputPrefixes)

	videoFiles := make(map[string]*models.FileData)

	for _, file := range files {
		if !file.IsDir() && hasFileExtension(file.Name(), videoExtensions) && hasPrefix(file.Name(), inputPrefixes) {
			filenameBase := filepath.Base(file.Name())

			fileData := models.NewFileData()
			fileData.OriginalVideoPath = filepath.Join(videoDir.Name(), file.Name())
			fileData.OriginalVideoBaseName = strings.TrimSuffix(filenameBase, filepath.Ext(file.Name()))
			fileData.VideoDirectory = videoDir.Name()

			if !strings.HasSuffix(fileData.OriginalVideoBaseName, consts.OldTag) {
				videoFiles[file.Name()] = fileData
			} else {
				logging.PrintI("Skipping file '%s' containing backup tag ('%s')", fileData.OriginalVideoBaseName, consts.OldTag)
			}

			logging.PrintI(`Added video to queue: %v`, filenameBase)
		}
	}

	if len(videoFiles) == 0 {
		return nil, fmt.Errorf("no video files with extensions: %v and prefixes: %v found in directory: %s", videoExtensions, inputPrefixes, videoDir.Name())
	}
	return videoFiles, nil
}

// GetMetadataFiles fetches metadata files from a directory
func GetMetadataFiles(metaDir *os.File) (map[string]*models.FileData, error) {
	files, err := metaDir.ReadDir(-1)
	if err != nil {
		return nil, fmt.Errorf("error reading metadata directory: %w", err)
	}

	metaExtensions := []string{".json"}
	inputPrefixFilters := cmd.GetStringSlice(keys.FilePrefixes)
	inputPrefixes := setPrefixFilter(inputPrefixFilters)

	metaFiles := make(map[string]*models.FileData)

	for _, file := range files {
		if !file.IsDir() && hasFileExtension(file.Name(), metaExtensions) && hasPrefix(file.Name(), inputPrefixes) {
			filenameBase := filepath.Base(file.Name())

			fileData := models.NewFileData()
			fileData.JSONFilePath = filepath.Join(metaDir.Name(), file.Name())
			fileData.JSONBaseName = strings.TrimSuffix(filenameBase, filepath.Ext(file.Name()))
			fileData.JSONDirectory = metaDir.Name()

			if !strings.Contains(fileData.JSONBaseName, consts.OldTag) {
				metaFiles[file.Name()] = fileData
			} else {
				logging.PrintI("Skipping file '%s' containing backup tag ('%s')", fileData.JSONBaseName, consts.OldTag)
			}
		}
	}

	if len(metaFiles) == 0 {
		return nil, fmt.Errorf("no meta files with extensions: %v and prefixes: %v found in directory: %s", metaExtensions, inputPrefixes, metaDir.Name())
	}
	return metaFiles, nil
}

// MatchVideoWithMetadata matches video files with their corresponding metadata files
func MatchVideoWithMetadata(videoFiles, metaFiles map[string]*models.FileData) (map[string]*models.FileData, error) {

	logging.PrintD(3, "Entering metadata and video file matching loop...")

	matchedFiles := make(map[string]*models.FileData)

	specialChars := regexp.MustCompile(`[^\w\s-]`)
	extraSpaces := regexp.MustCompile(`\s+`)

	for videoName, videoData := range videoFiles {

		// Normalize video name
		videoBase := strings.TrimSuffix(videoName, filepath.Ext(videoName))
		normalizedVideoBase := normalizeFilename(videoBase, specialChars, extraSpaces)
		logging.PrintD(3, "Normalized video base: %s", normalizedVideoBase)

		for metaName, metaData := range metaFiles {

			jsonBase := trimJsonSuffixes(metaName, videoBase)
			normalizedJsonBase := normalizeFilename(jsonBase, specialChars, extraSpaces)
			logging.PrintD(3, "Normalized metadata base: %s", normalizedJsonBase)

			if strings.Contains(normalizedJsonBase, normalizedVideoBase) {
				matchedFiles[videoName] = videoData
				matchedFiles[videoName].JSONFilePath = metaData.JSONFilePath
				matchedFiles[videoName].JSONBaseName = metaData.JSONBaseName
				matchedFiles[videoName].JSONDirectory = metaData.JSONDirectory
				break
			}
		}
	}

	if len(matchedFiles) == 0 {
		return nil, fmt.Errorf("no matching metadata files found for any videos")
	}

	return matchedFiles, nil
}

// hasVideoExtension checks if the file has a valid video extension
func hasFileExtension(fileName string, extensions []string) bool {

	if extensions == nil {
		extensions = append(extensions, ".*")
	}

	for _, ext := range extensions {
		if strings.HasSuffix(strings.ToLower(fileName), strings.ToLower(ext)) {
			return true
		}
	}

	// No matches
	return false
}

// hasPrefix determines if the input file has the desired prefix
func hasPrefix(fileName string, prefixes []string) bool {

	if prefixes == nil {
		prefixes = append(prefixes, "")
	}

	for _, data := range prefixes {
		if strings.HasPrefix(strings.ToLower(fileName), strings.ToLower(data)) {
			return true
		}
	}

	// No matches
	return false
}

// trimJsonSuffixes normalizes away common json string suffixes
// e.g. ".info" for yt-dlp outputted JSON files
func trimJsonSuffixes(jsonBase, videoBase string) string {

	switch {

	case strings.HasSuffix(jsonBase, ".info.json"): // FFmpeg
		if !strings.HasSuffix(videoBase, ".info") {
			jsonBase = strings.TrimSuffix(jsonBase, ".info.json")
		} else {
			jsonBase = strings.TrimSuffix(jsonBase, ".json")
		}

	case strings.HasSuffix(jsonBase, ".metadata.json"): // Angular
		if !strings.HasSuffix(videoBase, ".metadata") {
			jsonBase = strings.TrimSuffix(jsonBase, ".metadata.json")
		} else {
			jsonBase = strings.TrimSuffix(jsonBase, ".json")
		}

	case strings.HasSuffix(jsonBase, ".model.json"):
		if !strings.HasSuffix(videoBase, ".model") {
			jsonBase = strings.TrimSuffix(jsonBase, ".model.json")
		} else {
			jsonBase = strings.TrimSuffix(jsonBase, ".json")
		}

	case strings.HasSuffix(jsonBase, ".manifest.cdm.json"):
		if !strings.HasSuffix(videoBase, ".manifest.cdm") {
			jsonBase = strings.TrimSuffix(jsonBase, ".manifest.cdm.json")
		} else {
			jsonBase = strings.TrimSuffix(jsonBase, ".json")
		}

	default:
		if !strings.HasSuffix(videoBase, ".json") {
			jsonBase = strings.TrimSuffix(jsonBase, ".json")
		}
		logging.PrintD(1, "Common suffix not found for JSON (%s)", jsonBase)
	}

	return jsonBase
}

// setExtensions creates a list of extensions to filter
func setExtensions(convertFrom []enums.ConvertFromFiletype) []string {

	var videoExtensions []string

	for _, arg := range convertFrom {

		switch arg {
		case enums.IN_ALL_EXTENSIONS:
			videoExtensions = append(videoExtensions, ".mp4",
				".mkv",
				".avi",
				".wmv",
				".webm")

		case enums.IN_MKV:
			videoExtensions = append(videoExtensions, ".mkv")

		case enums.IN_MP4:
			videoExtensions = append(videoExtensions, ".mp4")

		case enums.IN_WEBM:
			videoExtensions = append(videoExtensions, ".webm")

		default:
			logging.PrintE(0, "Incorrect file format selected, reverting to default (convert from all)")
			videoExtensions = append(videoExtensions, ".mp4",
				".mkv",
				".avi",
				".wmv",
				".webm")
		}
	}

	return videoExtensions
}

// setPrefixFilter sets a list of prefixes to filter
func setPrefixFilter(inputPrefixFilters []string) []string {

	var prefixFilters []string

	prefixFilters = append(prefixFilters, inputPrefixFilters...)

	return prefixFilters
}

// normalizeFilename removes special characters and normalizes spacing
func normalizeFilename(filename string, specialChars, extraSpaces *regexp.Regexp) string {

	normalized := strings.ToLower(filename)
	normalized = specialChars.ReplaceAllString(normalized, "")
	normalized = extraSpaces.ReplaceAllString(normalized, " ")
	normalized = strings.TrimSpace(normalized)

	return normalized
}
