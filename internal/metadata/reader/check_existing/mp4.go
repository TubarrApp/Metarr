package metadata

import (
	consts "Metarr/internal/domain/constants"
	"Metarr/internal/models"
	logging "Metarr/internal/utils/logging"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
)

// MP4MetaMatches checks FFprobe captured metadata from the video against the metafile
func MP4MetaMatches(fd *models.FileData) bool {
	// Run ffprobe once to get all metadata
	cmd := exec.Command(
		"ffprobe",
		"-v", "quiet",
		"-print_format", "json",
		"-show_format",
		fd.OriginalVideoPath,
	)

	output, err := cmd.Output()
	if err != nil {
		logging.PrintE(0, "Error running ffprobe command: %v. Will process video.", err)
		return false
	}

	// Parse JSON output
	var ffData ffprobeOutput
	if err := json.Unmarshal(output, &ffData); err != nil {
		logging.PrintE(0, "Error parsing ffprobe output: %v. Will process video.", err)
		return false
	}

	// Create map of metadata to check
	fieldMap := map[string]struct {
		existing string
		new      string
	}{
		consts.JDescription: {
			existing: strings.TrimSpace(ffData.Format.Tags.Description),
			new:      strings.TrimSpace(fd.MTitleDesc.Description),
		},
		consts.JSynopsis: {
			existing: strings.TrimSpace(ffData.Format.Tags.Synopsis),
			new:      strings.TrimSpace(fd.MTitleDesc.Synopsis),
		},
		consts.JFallbackTitle: {
			existing: strings.TrimSpace(ffData.Format.Tags.Title),
			new:      strings.TrimSpace(fd.MTitleDesc.Title),
		},
		consts.JCreationTime: {
			existing: safeGetDatePart(ffData.Format.Tags.CreationTime),
			new:      safeGetDatePart(fd.MDates.Creation_Time),
		},
		consts.JDate: {
			existing: strings.TrimSpace(ffData.Format.Tags.Date),
			new:      strings.TrimSpace(fd.MDates.Date),
		},
		consts.JArtist: {
			existing: strings.TrimSpace(ffData.Format.Tags.Artist),
			new:      strings.TrimSpace(fd.MCredits.Artist),
		},
		consts.JComposer: {
			existing: strings.TrimSpace(ffData.Format.Tags.Composer),
			new:      strings.TrimSpace(fd.MCredits.Composer),
		},
	}

	// Collect all metadata for logging
	var ffContent []string
	matches := true

	// Check each field
	for key, values := range fieldMap {
		pair := fmt.Sprintf("Key: %s, Value: %s", key, values.existing)
		ffContent = append(ffContent, pair)

		if values.new != values.existing {
			logging.PrintD(2, "======== Mismatched meta in file: '%s' ========\nMismatch in key '%s':\nNew value: '%s'\nAlready in video as: '%s'. Will process video.",
				fd.OriginalVideoBaseName, key, values.new, values.existing)
			matches = false
		}
	}

	// Print all captured metadata
	printArray(ffContent)

	return matches
}

func printArray(s []string) {
	str := strings.Join(s, ", ")
	logging.PrintI("FFprobe captured %s", str)
}
