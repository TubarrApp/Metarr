package fsread

import (
	"errors"
	"metarr/internal/domain/consts"
	"metarr/internal/domain/enums"
	"metarr/internal/utils/logging"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// HasFileExtension checks if the file has a valid extension from a passed in map.
func HasFileExtension(filename string, extensions map[string]bool) bool {
	if extensions == nil {
		logging.E("Extensions sent in nil.")
		return false
	}
	ext := strings.ToLower(filepath.Ext(filename))
	if ext == "" {
		return false
	}
	if _, exists := extensions[ext]; exists {
		logging.I("File %q has valid extension %q, processing...", filename, ext)
		return true
	}
	logging.D(3, "File %q does not appear to have an extension contained in the extensions map", filename)
	return false
}

// matchesFileFilter determines if the input file has the desired suffix or prefix.
func matchesFileFilter(fileName string, slice []string, f func(string, string) bool) bool {
	if len(slice) == 0 {
		return false
	}
	for _, s := range slice {
		if f(fileName, s) {
			return true
		}
	}
	return false
}

// setVideoExtensions creates a list of extensions to filter.
func setVideoExtensions(exts []enums.ConvertFromFiletype) (map[string]bool, error) {
	videoExtensions := make(map[string]bool, len(consts.AllVidExtensions))

	for _, arg := range exts {
		switch arg {
		case enums.VidExtsMKV:
			videoExtensions[consts.ExtMKV] = true

		case enums.VidExtsMP4:
			videoExtensions[consts.ExtMP4] = true

		case enums.VidExtsWebM:
			videoExtensions[consts.ExtWEBM] = true

		case enums.VidExtsAll:
			for key := range consts.AllVidExtensions {
				videoExtensions[key] = true
			}
		}
	}

	if len(videoExtensions) == 0 {
		return nil, errors.New("failed to set video extensions")
	}

	return videoExtensions, nil
}

// setMetaExtensions creates a list of meta extensions to filter.
func setMetaExtensions(exts []enums.MetaFiletypeFilter) (map[string]bool, error) {
	metaExtensions := make(map[string]bool, len(consts.AllMetaExtensions))

	for _, arg := range exts {
		switch arg {
		case enums.MetaExtsJSON:
			metaExtensions[consts.MExtJSON] = true

		case enums.MetaExtsNFO:
			metaExtensions[consts.MExtNFO] = true

		case enums.MetaExtsAll:
			for key := range consts.AllMetaExtensions {
				metaExtensions[key] = true
			}
		}
	}

	if len(metaExtensions) == 0 {
		return nil, errors.New("failed to set meta extensions")
	}

	return metaExtensions, nil
}

// GetDirStats returns the number of video or metadata files in a directory, so maps/slices can be suitable sized.
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

// NormalizeFilename removes special characters and normalizes spacing.
func NormalizeFilename(filename string, specialChars, extraSpaces *regexp.Regexp) string {

	normalized := strings.ToLower(filename)
	normalized = specialChars.ReplaceAllString(normalized, "")
	normalized = extraSpaces.ReplaceAllString(normalized, " ")
	normalized = strings.TrimSpace(normalized)

	return normalized
}

// TrimMetafileSuffixes normalizes away common metafile string suffixes.
//
// E.g. ".info" for yt-dlp outputted JSON files
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
