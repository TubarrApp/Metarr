package ffprobe

import (
	"context"
	"encoding/json"
	"fmt"
	"metarr/internal/abstractions"
	"metarr/internal/domain/keys"
	"metarr/internal/domain/logger"
	"metarr/internal/models"
	"os/exec"

	"github.com/TubarrApp/gocommon/logging"
)

type tagDiff struct {
	existing string
	new      string
}

type tagDiffMap map[string]tagDiff

// CheckMetaMatches checks FFprobe captured metadata from the video against the metafile.
func CheckMetaMatches(ctx context.Context, extension string, fd *models.FileData) (allMetaMatches bool) {
	command := exec.CommandContext(
		ctx,
		"ffprobe",
		"-v", "quiet",
		"-print_format", "json",
		"-show_format", "-show_streams",
		fd.OriginalVideoPath,
	)

	logger.Pl.I("Made command for FFprobe:\n\n%v", command.String())
	output, err := command.Output()
	if err != nil {
		logger.Pl.E("Error running FFprobe command: %v. Will not process video.", err)
		return false
	}

	// Parse JSON output.
	var ffData ffprobeOutput
	if err := json.Unmarshal(output, &ffData); err != nil {
		logger.Pl.E("Error parsing FFprobe output: %v. Will not process video.", err)
		return false
	}

	// Check if thumbnail is already present in file.
	for _, s := range ffData.Streams {
		if s.Disposition.AttachedPic == 1 && s.CodecType == "video" {
			logger.Pl.I("Video %q has an embedded thumbnail", fd.OriginalVideoPath)
			fd.HasEmbeddedThumbnail = true

			// Thumbnail embedded in file, missing in metafile.
			if fd.MWebData.Thumbnail == "" {
				return false
			}
			break
		}
	}

	// Get strip thumbnail bool.
	stripThumbnail := false
	if abstractions.IsSet(keys.StripThumbnails) {
		stripThumbnail = abstractions.GetBool(keys.StripThumbnails)
	}

	// Thumbnail is in file but user wants to strip.
	if fd.HasEmbeddedThumbnail && stripThumbnail {
		logger.Pl.I("Thumbnail exists in video %q, set to be stripped", fd.OriginalVideoPath)
		return false
	}

	// No thumbnail in file but thumbnail exists in metadata.
	if !fd.HasEmbeddedThumbnail && fd.MWebData.Thumbnail != "" {
		logger.Pl.I("No thumbnail in video %q, found thumbnail %q", fd.OriginalVideoPath, fd.MWebData.Thumbnail)
		return false
	}

	// Map of metadata to check.
	metaCheckMap, exists := getDiffMapForFiletype(extension, fd, ffData)
	if !exists {
		logger.Pl.W("FFprobe metadata key map not available for filetype %s", extension)
		return false
	}

	// Collect all metadata for logging.
	ffContent := make([]string, 0, len(metaCheckMap))
	matches := true

	// Check each field.
	for key, values := range metaCheckMap {
		printVals := fmt.Sprintf("Currently in video: Key=%s, Value=%s, New Value=%s", key, values.existing, values.new)
		ffContent = append(ffContent, printVals)

		if values.new != values.existing { // Maintain case sensitivity (avoid strings.EqualFold).
			logger.Pl.D(2, "======== Mismatched meta in file: %q ========\nMismatch in key %q:\nNew value: %q\nIn video as: %q. Will process video.",
				fd.MetaFilePath, key, values.new, values.existing)
			matches = false
		} else {
			logger.Pl.D(2, "Detected key %q as being the same.\nFFprobe: %q\nMetafile: %q", key, values.existing, values.new)
		}
	}

	// Print all captured metadata.
	if logging.Level > 0 {
		printArray(ffContent)
	}
	return matches
}
