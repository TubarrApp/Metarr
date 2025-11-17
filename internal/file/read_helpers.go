package file

import (
	"metarr/internal/domain/logger"
	"metarr/internal/domain/lookupmaps"
	"metarr/internal/domain/regex"
	"path/filepath"
	"strings"
)

// HasFileExtension checks if the file has a valid extension from a passed in map.
func hasFileExtension(filename string, extensions map[string]bool) bool {
	if extensions == nil {
		logger.Pl.E("Extensions sent in nil.")
		return false
	}
	ext := strings.ToLower(filepath.Ext(filename))
	if ext == "" {
		return false
	}
	if isSet := extensions[ext]; isSet {
		logger.Pl.I("File %q has valid extension %q, processing...", filename, ext)
		return true
	}
	logger.Pl.D(3, "File %q does not appear to have an extension contained in the extensions map", filename)
	return false
}

// matchesFilenameFilter determines if the input file has the desired suffix or prefix.
func matchesFilenameFilter(fileName string, slice []string, f func(string, string) bool) bool {
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

// NormalizeFilename removes special characters and normalizes spacing.
func NormalizeFilename(filename string) string {
	normalized := strings.ToLower(filename)
	normalized = regex.ExtraSpacesCompile().ReplaceAllString(normalized, "")
	normalized = regex.ExtraSpacesCompile().ReplaceAllString(normalized, " ")
	normalized = strings.TrimSpace(normalized)

	return normalized
}

// TrimMetafileSuffixes normalizes away common metafile string suffixes.
//
// E.g. ".info" for yt-dlp outputted JSON files
func TrimMetafileSuffixes(metaBase, videoBase string) string {
	patterns := []struct {
		full  string
		noExt string
	}{
		// JSON
		{".info.json", ".info"},
		{".metadata.json", ".metadata"},
		{".model.json", ".model"},
		{".manifest.cdm.json", ".manifest.cdm"},

		// NFO
		{".movie.nfo", ".movie"},
		{".tvshow.nfo", ".tvshow"},
		{".episode.nfo", ".episode"},
		{".disc.nfo", ".disc"},
		{".release.nfo", ".release"},
		{".bdinfo.nfo", ".bdinfo"},
		{".mediainfo.nfo", ".mediainfo"},
	}
	// Trims suffix from metafiles. Handles cases where a video was filename.metadata.mp4
	// and metafile was filename.metadata.json, so both become filename.metadata and match.
	for _, pattern := range patterns {
		if strings.HasSuffix(metaBase, pattern.full) {
			if !strings.HasSuffix(videoBase, pattern.noExt) {
				return strings.TrimSuffix(metaBase, pattern.full)
			}
		}
	}
	// Same as above but directly strips the metafile extension. Handles edge cases where
	// video is file.json.mp4 and metafile is file.json, so they both become file.json.
	for k := range lookupmaps.AllMetaExtensions {
		if strings.HasSuffix(metaBase, k) && !strings.HasSuffix(videoBase, k) {
			return strings.TrimSuffix(metaBase, k)
		}
	}
	return metaBase
}
