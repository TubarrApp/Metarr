package parsing

import (
	"path/filepath"
	"strings"
	"unicode/utf8"
)

// MaxFilenameBytes is the longest file name most filesystems allow.
const MaxFilenameBytes = 255

// pathSeparatorReplacer replaces characters that would split a name into directories.
var pathSeparatorReplacer = strings.NewReplacer("/", "-", "\\", "-", "\x00", "")

// SanitizePathComponent makes s safe as a single file or directory name, cut to at most maxBytes.
func SanitizePathComponent(s string, maxBytes int) string {
	s = strings.TrimSpace(pathSeparatorReplacer.Replace(s))
	if s == "." || s == ".." {
		s = strings.ReplaceAll(s, ".", "-")
	}
	if len(s) <= maxBytes {
		return s
	}
	cut := maxBytes
	for cut > 0 && !utf8.RuneStart(s[cut]) {
		cut--
	}
	return strings.TrimSpace(s[:cut])
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
