package existingmeta

import (
	"context"
	"encoding/json"
	"fmt"
	"metarr/internal/domain/consts"
	"metarr/internal/models"
	"metarr/internal/utils/logging"
	"os/exec"
	"strings"
)

// MP4MetaMatches checks FFprobe captured metadata from the video against the metafile.
func MP4MetaMatches(ctx context.Context, fd *models.FileData) bool {

	c := fd.MCredits
	d := fd.MDates
	t := fd.MTitleDesc

	// FFprobe command fetches metadata from the video file
	command := exec.CommandContext(
		ctx,
		"ffprobe",
		"-v", "quiet",
		"-print_format", "json",
		"-show_format",
		fd.OriginalVideoPath,
	)

	logging.I("Made command for FFprobe:\n\n%v", command.String())

	output, err := command.Output()
	if err != nil {
		logging.E(0, "Error running FFprobe command: %v. Will not process video.", err)
		return false
	}

	// Parse JSON output
	var ffData ffprobeOutput

	if err := json.Unmarshal(output, &ffData); err != nil {
		logging.E(0, "Error parsing FFprobe output: %v. Will not process video.", err)
		return false
	}

	// Map of metadata to check
	metaCheckMap := map[string]struct {
		existing string
		new      string
	}{
		consts.JDescription: {
			existing: strings.TrimSpace(ffData.Format.Tags.Description),
			new:      strings.TrimSpace(t.Description),
		},
		consts.JSynopsis: {
			existing: strings.TrimSpace(ffData.Format.Tags.Synopsis),
			new:      strings.TrimSpace(t.Synopsis),
		},
		consts.JTitle: {
			existing: strings.TrimSpace(ffData.Format.Tags.Title),
			new:      strings.TrimSpace(t.Title),
		},
		consts.JCreationTime: {
			existing: safeGetDatePart(ffData.Format.Tags.CreationTime),
			new:      safeGetDatePart(d.CreationTime),
		},
		consts.JDate: {
			existing: strings.TrimSpace(ffData.Format.Tags.Date),
			new:      strings.TrimSpace(d.Date),
		},
		consts.JArtist: {
			existing: strings.TrimSpace(ffData.Format.Tags.Artist),
			new:      strings.TrimSpace(c.Artist),
		},
		consts.JComposer: {
			existing: strings.TrimSpace(ffData.Format.Tags.Composer),
			new:      strings.TrimSpace(c.Composer),
		},
	}

	// Collect all metadata for logging
	var ffContent []string
	matches := true

	// Check each field
	for key, values := range metaCheckMap {
		printVals := fmt.Sprintf("Currently in video: Key=%s, Value=%s, New Value=%s", key, values.existing, values.new)
		ffContent = append(ffContent, printVals)

		if values.new != values.existing {
			logging.D(2, "======== Mismatched meta in file: %q ========\nMismatch in key %q:\nNew value: %q\nIn video as: %q. Will process video.",
				fd.OriginalVideoBaseName, key, values.new, values.existing)
			matches = false
		} else {
			logging.D(2, "Detected key %q as being the same.\nFFprobe: %q\nMetafile: %q", key, values.existing, values.new)
		}
	}

	// Print all captured metadata
	if logging.Level > 0 {
		printArray(ffContent)
	}

	return matches
}
