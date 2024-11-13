package utils

import (
	"fmt"
	consts "metarr/internal/domain/constants"
	enums "metarr/internal/domain/enums"
	logging "metarr/internal/utils/logging"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// hasVideoExtension checks if the file has a valid video extension
func HasFileExtension(filename string, extensions map[string]bool) bool {
	if extensions == nil {
		logging.E(0, "Extensions sent in nil.")
		return false
	}

	ext := strings.ToLower(filepath.Ext(filename))
	if ext == "" {
		return false
	}

	if _, exists := extensions[ext]; exists {
		logging.I("File '%s' has valid extension '%s', processing...", filename, ext)
		return true
	}
	logging.D(3, "File '%s' does not appear to have an extension contained in the extensions map", filename)
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
	return false
}

// setVideoExtensions creates a list of extensions to filter
func setVideoExtensions(exts []enums.ConvertFromFiletype) (map[string]bool, error) {

	videoExtensions := make(map[string]bool, len(consts.AllVidExtensions))

	for _, arg := range exts {
		switch arg {
		case enums.VID_EXTS_MKV:
			videoExtensions[consts.ExtMKV] = true

		case enums.VID_EXTS_MP4:
			videoExtensions[consts.ExtMP4] = true

		case enums.VID_EXTS_WEBM:
			videoExtensions[consts.ExtWEBM] = true

		case enums.VID_EXTS_ALL:
			for key := range consts.AllVidExtensions {
				videoExtensions[key] = true
			}
		}
	}

	if len(videoExtensions) == 0 {
		return nil, fmt.Errorf("failed to set video extensions")
	}

	return videoExtensions, nil
}

// setMetaExtensions creates a lists of meta extensions to filter
func setMetaExtensions(exts []enums.MetaFiletypeFilter) (map[string]bool, error) {

	metaExtensions := make(map[string]bool, len(consts.AllMetaExtensions))

	for _, arg := range exts {
		switch arg {
		case enums.META_EXTS_JSON:
			metaExtensions[consts.MExtJSON] = true

		case enums.META_EXTS_NFO:
			metaExtensions[consts.MExtNFO] = true

		case enums.META_EXTS_ALL:
			for key := range consts.AllMetaExtensions {
				metaExtensions[key] = true
			}
		}
	}

	if len(metaExtensions) == 0 {
		return nil, fmt.Errorf("failed to set meta extensions")
	}

	return metaExtensions, nil
}

// setPrefixFilter sets a list of prefixes to filter
func SetPrefixFilter(inputPrefixFilters []string) []string {

	prefixFilters := make([]string, 0, len(inputPrefixFilters))
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

			for key := range consts.AllVidExtensions {
				if ext == key {
					vidCount++
					continue
				}
				switch ext {
				case consts.MExtJSON, consts.MExtNFO:
					metaCount++
					continue
				}
			}
		}
	}
	return vidCount, metaCount
}

// normalizeFilename removes special characters and normalizes spacing
func NormalizeFilename(filename string, specialChars, extraSpaces *regexp.Regexp) string {

	normalized := strings.ToLower(filename)
	normalized = specialChars.ReplaceAllString(normalized, "")
	normalized = extraSpaces.ReplaceAllString(normalized, " ")
	normalized = strings.TrimSpace(normalized)

	return normalized
}

// trimJsonSuffixes normalizes away common json string suffixes
// e.g. ".info" for yt-dlp outputted JSON files
func TrimMetafileSuffixes(metaBase, videoBase string) string {

	switch {

	case strings.HasSuffix(metaBase, ".info.json"): // FFmpeg
		if !strings.HasSuffix(videoBase, ".info") {
			metaBase = strings.TrimSuffix(metaBase, ".info.json")
		} else {
			metaBase = strings.TrimSuffix(metaBase, consts.MExtJSON)
		}

	case strings.HasSuffix(metaBase, ".metadata.json"): // Angular
		if !strings.HasSuffix(videoBase, ".metadata") {
			metaBase = strings.TrimSuffix(metaBase, ".metadata.json")
		} else {
			metaBase = strings.TrimSuffix(metaBase, consts.MExtJSON)
		}

	case strings.HasSuffix(metaBase, ".model.json"):
		if !strings.HasSuffix(videoBase, ".model") {
			metaBase = strings.TrimSuffix(metaBase, ".model.json")
		} else {
			metaBase = strings.TrimSuffix(metaBase, consts.MExtJSON)
		}

	case strings.HasSuffix(metaBase, ".manifest.cdfd.json"):
		if !strings.HasSuffix(videoBase, ".manifest.cdm") {
			metaBase = strings.TrimSuffix(metaBase, ".manifest.cdfd.json")
		} else {
			metaBase = strings.TrimSuffix(metaBase, consts.MExtJSON)
		}

	default:
		switch {
		case !strings.HasSuffix(videoBase, consts.MExtJSON): // Edge cases where metafile extension is in the suffix of the video file
			metaBase = strings.TrimSuffix(metaBase, consts.MExtJSON)

		case !strings.HasSuffix(videoBase, consts.MExtNFO):
			metaBase = strings.TrimSuffix(metaBase, consts.MExtNFO)

		default:
			logging.D(1, "Common suffix not found for metafile (%s)", metaBase)
		}
	}
	return metaBase
}
