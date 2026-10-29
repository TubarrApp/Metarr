package parsing

import (
	"metarr/internal/abstractions"
	"metarr/internal/domain/keys"
	"metarr/internal/domain/logger"
	"net/url"
	"path/filepath"
	"strings"

	"github.com/TubarrApp/gocommon/sharedconsts"
	"github.com/TubarrApp/gocommon/sharedvalidation"
)

// ReadOnlyExt reports whether ext is a video extension FFmpeg reads but can't write playable files in (.rm, .rmvb).
func ReadOnlyExt(ext string) bool {
	if _, ok := sharedconsts.AllVidExtensions[strings.ToLower(ext)]; !ok {
		return false
	}
	_, err := sharedvalidation.ValidateFFmpegOutputExt(ext)
	return err != nil
}

// OutputExt returns the extension the video at path is written with: the output filetype if set, else its own, or
// .mkv (which holds the most codecs) if its own is read-only, reporting that swap with replaced.
func OutputExt(path string) (ext string, replaced bool) {
	if abstractions.IsSet(keys.OutputFiletype) {
		if ext = abstractions.GetString(keys.OutputFiletype); ext != "" {
			return ext, false
		}
	}
	if ext = filepath.Ext(path); ReadOnlyExt(ext) {
		return sharedconsts.ExtMKV, true
	}
	return ext, false
}

// IsHTTPURL reports whether s is an absolute http(s) URL.
func IsHTTPURL(s string) bool {
	u, err := url.Parse(s)
	if u == nil {
		logger.Pl.E("Failed to parse URL from %q", s)
		return false
	}
	return err == nil && (u.Scheme == "http" || u.Scheme == "https") && u.Host != ""
}

// GetBaseNameWithoutExt returns the base name (without extension) of any file path.
func GetBaseNameWithoutExt(path string) string {
	if path == "" {
		return ""
	}
	base := filepath.Base(path)
	return strings.TrimSuffix(base, filepath.Ext(base))
}

// GetBaseNameWithExt returns the base name (without extension) of any file path.
func GetBaseNameWithExt(path string) string {
	if path == "" {
		return ""
	}
	return filepath.Base(path)
}

// GetFilepathWithoutExt returns the base name (without extension) of any file path.
func GetFilepathWithoutExt(path string) string {
	if path == "" {
		return ""
	}
	return strings.TrimSuffix(path, filepath.Ext(path))
}
