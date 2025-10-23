package transpresets

import (
	"metarr/internal/models"
	"metarr/internal/utils/logging"
)

// addFilenameReplacements adds suffix and prefix replacements to FileData without duplicates.
func addFilenameReplacements(fd *models.FileData, suffixes []models.FilenameReplaceSuffix, prefixes []models.FilenameReplacePrefix) {
	// Add suffixes
	for _, newSuffix := range suffixes {
		exists := false
		for _, existing := range fd.FilenameReplaceSuffix {
			if existing.Suffix == newSuffix.Suffix && existing.Replacement == newSuffix.Replacement {
				exists = true
				logging.D(3, "Suffix replacement %q -> %q already exists, skipping", newSuffix.Suffix, newSuffix.Replacement)
				break
			}
		}
		if !exists {
			fd.FilenameReplaceSuffix = append(fd.FilenameReplaceSuffix, newSuffix)
			logging.D(2, "Added suffix replacement: %q -> %q", newSuffix.Suffix, newSuffix.Replacement)
		}
	}

	// Add prefixes
	for _, newPrefix := range prefixes {
		exists := false
		for _, existing := range fd.FilenameReplacePrefix {
			if existing.Prefix == newPrefix.Prefix && existing.Replacement == newPrefix.Replacement {
				exists = true
				logging.D(3, "Prefix replacement %q -> %q already exists, skipping", newPrefix.Prefix, newPrefix.Replacement)
				break
			}
		}
		if !exists {
			fd.FilenameReplacePrefix = append(fd.FilenameReplacePrefix, newPrefix)
			logging.D(2, "Added prefix replacement: %q -> %q", newPrefix.Prefix, newPrefix.Replacement)
		}
	}
}

// addSuffix is a convenience function to add a single suffix replacement.
func addSuffix(fd *models.FileData, suffix, replacement string) {
	addFilenameReplacements(fd, []models.FilenameReplaceSuffix{
		{Suffix: suffix, Replacement: replacement},
	}, nil)
}

// addPrefix is a convenience function to add a single prefix replacement.
func addPrefix(fd *models.FileData, prefix, replacement string) {
	addFilenameReplacements(fd, nil, []models.FilenameReplacePrefix{
		{Prefix: prefix, Replacement: replacement},
	})
}
