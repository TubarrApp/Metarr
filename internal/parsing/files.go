package parsing

import (
	"path/filepath"
	"strings"
)

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
