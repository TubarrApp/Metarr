// Package metaconversion converts metadata held in a FileData model between metafile formats.
package metaconversion

import (
	"fmt"
	"metarr/internal/abstractions"
	"metarr/internal/domain/keys"
	"metarr/internal/domain/logger"
	"metarr/internal/models"
	"os"
	"path/filepath"
	"strings"

	"github.com/TubarrApp/gocommon/sharedconsts"
)

const nfoFilePerms = 0o644

// WriteNFOs writes an NFO file for each model.
func WriteNFOs(fdArray []*models.FileData) {
	if !abstractions.GetBool(keys.WriteNFO) {
		logger.Pl.D(2, "NFO output not requested, skipping NFO generation")
		return
	}

	var written int
	for _, fd := range fdArray {
		if fd == nil {
			continue
		}

		outPath, err := WriteNFO(fd)
		if err != nil {
			logger.Pl.E("Failed to write NFO for %q: %v", nfoSourcePath(fd), err)
			continue
		}
		if outPath != "" {
			written++
		}
	}

	if written > 0 {
		logger.Pl.S("Wrote %d NFO file(s)", written)
	}
}

// WriteNFO builds and writes the NFO file for the provided FileData model.
//
// Returns the path of the written NFO file, or an empty string if no NFO was written.
func WriteNFO(fd *models.FileData) (string, error) {
	if fd == nil {
		return "", fmt.Errorf("FileData is nil for input, cannot write NFO")
	}

	source := nfoSourcePath(fd)
	if source == "" {
		logger.Pl.W("No final path available for NFO output, skipping NFO generation for this file")
		return "", nil
	}

	contents, err := BuildNFOContents(fd)
	if err != nil {
		return "", err
	}
	if contents == "" {
		return "", nil
	}

	outPath := strings.TrimSuffix(source, filepath.Ext(source)) + sharedconsts.MExtNFO

	// Do not clobber an existing NFO unless the user allows overwriting.
	if _, statErr := os.Stat(outPath); statErr == nil {
		if abstractions.GetBool(keys.NoFileOverwrite) {
			logger.Pl.I("NFO file %q already exists and overwriting is disabled, skipping", outPath)
			return "", nil
		}
		logger.Pl.I("Overwriting existing NFO file %q", outPath)
	}

	if err := os.WriteFile(outPath, []byte(contents), nfoFilePerms); err != nil {
		return "", fmt.Errorf("failed to write NFO file %q: %w", outPath, err)
	}

	fd.NFOFileContents = contents
	logger.Pl.S("Wrote NFO file %q", outPath)

	return outPath, nil
}

// nfoSourcePath returns the path the NFO filename should be derived from.
func nfoSourcePath(fd *models.FileData) string {
	return firstNonEmpty(
		fd.FinalVideoPath,
		fd.FinalMetaPath,
		fd.RenamedVideoPath,
		fd.PostFFmpegVideoPath,
		fd.OriginalVideoPath,
		fd.MetaFilePath,
	)
}
