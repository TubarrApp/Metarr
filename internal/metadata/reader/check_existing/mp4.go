package metadata

import (
	"Metarr/internal/models"
	"fmt"
	"os/exec"
	"strings"

	consts "Metarr/internal/domain/constants"
	logging "Metarr/internal/utils/logging"
)

func MP4MetaMatches(fd *models.FileData) (same bool) {

	c := fd.MCredits
	d := fd.MDates
	t := fd.MTitleDesc

	fieldMap := map[string]string{
		// Titles/descs
		consts.JDescription:   t.Description,
		consts.JSynopsis:      t.Synopsis,
		consts.JFallbackTitle: t.Title,

		// Dates/times
		consts.JCreationTime: d.Creation_Time,
		consts.JDate:         d.Date,

		// Credits
		consts.JArtist:   c.Artist,
		consts.JComposer: c.Composer,
	}

	ffContent := make([]string, 0, len(fieldMap))
	for key, value := range fieldMap {

		command := exec.Command("ffprobe", "-v", "quiet", "-show_entries", "format_tags="+key, "-of", "default=noprint_wrappers=1:nokey=1", fd.OriginalVideoPath)
		result, err := command.Output()
		if err != nil {
			logging.PrintE(0, "Error in ffprobe command: %v. Will process video.", err.Error())
			return false
		}
		strResult := string(result)

		if key == consts.JCreationTime {
			value, _, _ = strings.Cut(value, "T")
			strResult, _, _ = strings.Cut(strResult, "T")
		}

		value = strings.TrimSpace(value)
		strResult = strings.TrimSpace(strResult)

		if value != strResult {
			logging.PrintD(2, "======== Mismatched meta in file: '%s' ========\nMismatch in key '%s':\nNew value: '%s'\nAlready in video as: '%s'. Will process video.", fd.OriginalVideoBaseName, key, value, strResult)
			pair := fmt.Sprintf("Key: %s, Value %s", key, strResult)
			ffContent = append(ffContent, pair)
			printArray(ffContent)
			return false
		}
		pair := fmt.Sprintf("Key: %s, Value %s", key, strResult)
		ffContent = append(ffContent, pair)
	}
	printArray(ffContent)
	return true
}

func printArray(s []string) {
	str := strings.Join(s, ", ")
	logging.PrintI("FFprobe captured %s", str)
}
