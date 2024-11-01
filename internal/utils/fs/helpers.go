package utils

import (
	enums "Metarr/internal/domain/enums"
	logging "Metarr/internal/utils/logging"
	"os"
	"path/filepath"
	"strings"
)

// hasVideoExtension checks if the file has a valid video extension
func HasFileExtension(fileName string, extensions []string) bool {

	if extensions == nil {
		logging.PrintE(0, "NO EXTENSIONS PICKED.")
		return false
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
func HasPrefix(fileName string, prefixes []string) bool {

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

// setExtensions creates a list of extensions to filter
func SetExtensions(convertFrom []enums.ConvertFromFiletype) []string {

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
func SetPrefixFilter(inputPrefixFilters []string) []string {

	var prefixFilters []string

	prefixFilters = append(prefixFilters, inputPrefixFilters...)

	return prefixFilters
}

// GetDirStats returns the number of video or metadata files in a directory, so maps/slices can be suitable sized
func GetDirStats(dir string) (vidCount, metaCount int) {
	// Quick initial scan just counting files, not storing anything
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0, 0
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			ext := strings.ToLower(filepath.Ext(entry.Name()))
			switch ext {
			case ".3gp", ".avi", ".f4v", ".flv", ".m4v", ".mkv",
				".mov", ".mp4", ".mpeg", ".mpg", ".ogm", ".ogv",
				".ts", ".vob", ".webm", ".wmv":
				vidCount++
			case ".json", ".nfo":
				metaCount++
			}
		}
	}
	return vidCount, metaCount
}
