package metadata

import (
	"Metarr/internal/backup"
	"Metarr/internal/cmd"
	"Metarr/internal/commandvars"
	"Metarr/internal/consts"
	"Metarr/internal/enums"
	"Metarr/internal/keys"
	"Metarr/internal/logging"
	"Metarr/internal/models"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
)

var (
	preCommandMutex  sync.RWMutex
	postCommandMutex sync.RWMutex
)

// WriteMetadata writes metadata to a single video file
func WriteMetadata(m *models.FileData) error {

	// Set mutex until command execution
	preCommandMutex.Lock()

	var originalVPath string = m.OriginalVideoPath
	dir := m.VideoDirectory

	fmt.Printf("\nWriting metadata for file: %s\n", originalVPath)

	// Make temp output path with .mp4 extension
	fileBase := strings.TrimSuffix(filepath.Base(originalVPath), filepath.Ext(originalVPath))

	tempOutputFilePath := filepath.Join(dir, consts.TempTag+fileBase+filepath.Ext(originalVPath)+".mp4")
	m.TempOutputFilePath = tempOutputFilePath // Add to video file data struct

	defer func() {
		if _, err := os.Stat(tempOutputFilePath); err == nil {
			os.Remove(tempOutputFilePath)
		}
	}()

	args, err := buildCommand(m, tempOutputFilePath)
	if err != nil {
		// Unlock mutex
		preCommandMutex.Unlock()
		return err
	}

	command := exec.Command("ffmpeg", args...)

	logging.PrintI("\n%sConstructed FFmpeg command for%s '%s':\n\n%v\n", consts.ColorCyan, consts.ColorReset, m.OriginalVideoPath, command.String())

	command.Stdout = os.Stdout
	command.Stderr = os.Stderr

	origPath := originalVPath
	m.FinalVideoBaseName = strings.TrimSuffix(filepath.Base(origPath), filepath.Ext(origPath))
	m.FinalVideoPath = filepath.Join(m.VideoDirectory, m.FinalVideoBaseName) + ".mp4"

	fmt.Printf(`

Video file path data:
	
Original Video Path: %s
Metadata File Path: %s
Final Video Path: %s

Temp Output Path: %s
	
`, originalVPath,
		m.JSONFilePath,
		m.FinalVideoPath,
		m.TempOutputFilePath)

	// Unlock mutex
	preCommandMutex.Unlock()

	// Run the ffmpeg command
	logging.Print("%s!!! Starting FFmpeg command for '%s'...\n%s", consts.ColorCyan, m.FinalVideoBaseName, consts.ColorReset)
	if err := command.Run(); err != nil {
		logging.ErrorArray = append(logging.ErrorArray, err)
		return fmt.Errorf("failed to run ffmpeg command: %w", err)
	}

	// Lock second mutex after command executes
	postCommandMutex.Lock()
	defer postCommandMutex.Unlock()

	// Rename temporary file to overwrite the original video file:
	// First check overwrite rules
	if cmd.GetBool(keys.NoFileOverwrite) && originalVPath == m.FinalVideoPath {
		if err := backup.RenameToBackup(originalVPath); err != nil {
			return fmt.Errorf("failed to rename original file and preserve file is on, aborting: %w", err)
		}
	}
	err = os.Rename(tempOutputFilePath, m.FinalVideoPath)
	if err != nil {
		return fmt.Errorf("failed to overwrite original file: %w", err)
	}

	fmt.Printf("Successfully renamed video from %s to %s\n", tempOutputFilePath, m.FinalVideoPath)

	if filepath.Ext(originalVPath) != ".mp4" {
		logging.PrintI("Removing original non-MP4 file: %s", originalVPath)

		if cmd.GetBool(keys.NoFileOverwrite) {
			if _, err := os.Stat(originalVPath); os.IsNotExist(err) {
				logging.PrintI("File does not exist, safe to proceed overwriting: %s", originalVPath)
			} else {
				if err := backup.RenameToBackup(originalVPath); err != nil {
					return fmt.Errorf("failed to rename original file and preserve file is on, aborting: %w", err)
				}
			}
			err = os.Remove(originalVPath)
			if err != nil {
				logging.ErrorArray = append(logging.ErrorArray, err)
				return fmt.Errorf("failed to remove original file (%s). Error: %v", originalVPath, err)
			}
		}
	}

	fmt.Println()
	logging.PrintS(0, `Successfully processed video:

Original file: %s
New file: %s

Title: %s

`, originalVPath,
		m.FinalVideoPath,
		m.MTitleDesc.Title)

	return nil
}

// buildCommand is the function to create the final FFmpeg output command
func buildCommand(m *models.FileData, outputFile string) ([]string, error) {
	var originalVPath string = m.OriginalVideoPath
	var args []string

	// Determine GPU Acceleration
	gpuFlag := cmd.Get(keys.GPUEnum).(enums.SysGPU)
	switch gpuFlag {
	case enums.NVIDIA:
		args = append(args, commandvars.NvidiaAccel...)
	case enums.AMD:
		args = append(args, commandvars.AMDAccel...)
	case enums.INTEL:
		args = append(args, commandvars.IntelAccel...)
	}

	// Input file argument
	args = append(args, "-y", "-i", originalVPath)

	// Metadata titles
	if m.MTitleDesc.Title != "" {
		args = append(args, "-metadata", fmt.Sprintf("title=%s", fieldFormatter(m.MTitleDesc.Title)))
	} else if m.MTitleDesc.FallbackTitle != "" {
		args = append(args, "-metadata", fmt.Sprintf("title=%s", fieldFormatter(m.MTitleDesc.FallbackTitle)))
	}

	if m.MTitleDesc.Subtitle != "" {
		args = append(args, "-metadata", fmt.Sprintf("subtitle=%s", fieldFormatter(m.MTitleDesc.Subtitle)))
	}
	if m.MTitleDesc.Description != "" {
		args = append(args, "-metadata", fmt.Sprintf("description=%s", fieldFormatter(m.MTitleDesc.Description)))
	}
	if m.MTitleDesc.LongDescription != "" {
		args = append(args, "-metadata", fmt.Sprintf("longdescription=%s", fieldFormatter(m.MTitleDesc.LongDescription)))
	}
	if m.MTitleDesc.Synopsis != "" {
		args = append(args, "-metadata", fmt.Sprintf("synopsis=%s", fieldFormatter(m.MTitleDesc.Synopsis)))
	}
	if m.MTitleDesc.Comment != "" {
		args = append(args, "-metadata", fmt.Sprintf("comment=%s", fieldFormatter(m.MTitleDesc.Comment)))
	}
	logging.PrintD(1, "Adding title metadata: %v", m.MTitleDesc)

	// Metadata credits
	if m.MCredits.Actor != "" {
		args = append(args, "-metadata", fmt.Sprintf("actor=%s", fieldFormatter(m.MCredits.Actor)))
	}
	if m.MCredits.Author != "" {
		args = append(args, "-metadata", fmt.Sprintf("author=%s", fieldFormatter(m.MCredits.Author)))
	}
	if m.MCredits.Artist != "" {
		args = append(args, "-metadata", fmt.Sprintf("artist=%s", fieldFormatter(m.MCredits.Artist)))
	}
	if m.MCredits.Creator != "" {
		args = append(args, "-metadata", fmt.Sprintf("creator=%s", fieldFormatter(m.MCredits.Creator)))
	}
	if m.MCredits.Studio != "" {
		args = append(args, "-metadata", fmt.Sprintf("studio=%s", fieldFormatter(m.MCredits.Studio)))
	}
	if m.MCredits.Publisher != "" {
		args = append(args, "-metadata", fmt.Sprintf("publisher=%s", fieldFormatter(m.MCredits.Publisher)))
	}
	if m.MCredits.Producer != "" {
		args = append(args, "-metadata", fmt.Sprintf("producer=%s", fieldFormatter(m.MCredits.Producer)))
	}
	if m.MCredits.Performer != "" {
		args = append(args, "-metadata", fmt.Sprintf("performer=%s", fieldFormatter(m.MCredits.Performer)))
	}
	if m.MCredits.Uploader != "" {
		args = append(args, "-metadata", fmt.Sprintf("uploader=%s", fieldFormatter(m.MCredits.Uploader)))
	}
	if m.MCredits.Composer != "" {
		args = append(args, "-metadata", fmt.Sprintf("composer=%s", fieldFormatter(m.MCredits.Composer)))
	}
	if m.MCredits.Director != "" {
		args = append(args, "-metadata", fmt.Sprintf("director=%s", fieldFormatter(m.MCredits.Director)))
	}
	logging.PrintD(1, "Adding credits metadata: %v", m.MCredits)

	// Metadata dates
	if m.MDates.UploadDate != "" {
		args = append(args, "-metadata", fmt.Sprintf("upload_date=%s", fieldFormatter(m.MDates.UploadDate)))
	}
	if m.MDates.ReleaseDate != "" {
		args = append(args, "-metadata", fmt.Sprintf("release_date=%s", fieldFormatter(m.MDates.ReleaseDate)))
	}
	if m.MDates.Date != "" {
		args = append(args, "-metadata", fmt.Sprintf("date=%s", fieldFormatter(m.MDates.Date)))
	}
	if m.MDates.Year != "" {
		args = append(args, "-metadata", fmt.Sprintf("year=%s", fieldFormatter(m.MDates.Year)))
	}
	if m.MDates.Originally_Available_At != "" {
		args = append(args, "-metadata", fmt.Sprintf("originally_available_at=%s", fieldFormatter(m.MDates.Originally_Available_At)))
	}
	if m.MDates.Creation_Time != "" {
		args = append(args, "-metadata", fmt.Sprintf("creation_time=%s", fieldFormatter(m.MDates.Creation_Time)))
	}
	logging.PrintD(1, "Adding date metadata: %v", m.MDates)

	// Metadata show info
	if m.MShowData.Show != "" {
		args = append(args, "-metadata", fmt.Sprintf("show=%s", fieldFormatter(m.MShowData.Show)))
	}
	if m.MShowData.Episode_ID != "" {
		args = append(args, "-metadata", fmt.Sprintf("episode_id=%s", fieldFormatter(m.MShowData.Episode_ID)))
	}
	if m.MShowData.Episode_Sort != "" {
		args = append(args, "-metadata", fmt.Sprintf("episode_sort=%s", fieldFormatter(m.MShowData.Episode_Sort)))
	}
	if m.MShowData.Season_Number != "" {
		args = append(args, "-metadata", fmt.Sprintf("season_number=%s", fieldFormatter(m.MShowData.Season_Number)))
	}
	logging.PrintD(1, "Adding show info metadata: %v", m.MShowData)

	// Other metadata
	if m.MOther.Language != "" {
		args = append(args, "-metadata", fmt.Sprintf("language=%s", fieldFormatter(m.MOther.Language)))
	}
	if m.MOther.Genre != "" {
		args = append(args, "-metadata", fmt.Sprintf("genre=%s", fieldFormatter(m.MOther.Genre)))
	}
	if m.MOther.HD_Video != "" {
		args = append(args, "-metadata", fmt.Sprintf("hd_video=%s", fieldFormatter(m.MOther.HD_Video)))
	}
	logging.PrintD(1, "Adding other metadata: %v", m.MOther)

	// Output format specific arguments
	fileExtension := filepath.Ext(originalVPath)
	switch fileExtension {
	case ".mp4":
		args = append(args, commandvars.AVCodecCopy...)
	case ".mkv":
		args = append(args, commandvars.OutputExt...)
		args = append(args, commandvars.VideoCodecCopy...)
		args = append(args, commandvars.AudioToAAC...)
		args = append(args, commandvars.AudioBitrate...)
	case ".webm":
		args = append(args, commandvars.OutputExt...)
		args = append(args, commandvars.VideoToH264Balanced...)
		args = append(args, commandvars.PixelFmtYuv420p...)
		args = append(args, commandvars.KeyframeBalanced...)
		args = append(args, commandvars.AudioToAAC...)
		args = append(args, commandvars.AudioBitrate...)
	}

	// Output file
	args = append(args, outputFile)

	// Debug print the final command
	logging.PrintD(1, "Metadata arguments for %s:\n", m.OriginalVideoBaseName)
	for i, arg := range args {
		if i > 0 && args[i-1] == "-metadata" {
			fmt.Printf("  %s\n", arg)
		}
	}
	fmt.Println()

	return args, nil
}

// formatter formats field values
func fieldFormatter(fieldValue string) string {

	if fieldValue != "" {
		fieldValue = strings.TrimSpace(fieldValue)
	}

	return fieldValue
}

// makeDateTag creates the date tag to affix to the filename
func makeDateTag(jsonData map[string]interface{}, fileName string) (string, error) {

	preferredDateFields := []string{"release_date", "creation_time", "upload_date", "date", "year"}

	dateFmt, ok := cmd.Get(keys.FileDateFmt).(enums.FilenameDateFormat)
	if !ok {
		return "", fmt.Errorf("grabbed value from Viper in makeDateTag but var type was not correct")
	}

	var gotDate bool = false
	var date string = ""

	for _, field := range preferredDateFields {
		if value, found := jsonData[field]; found && !gotDate {
			date, ok = value.(string)
			if !ok {
				dateInt, ok := value.(int)
				if !ok {
					continue
				}
				date = strconv.Itoa(dateInt)
			}
			gotDate = true
			break
		}
	}

	if !gotDate {
		logging.PrintE(0, "No dates found in JSON file")
		return "", nil
	}

	alreadyFormatted := strings.Contains(date, "-")

	if !alreadyFormatted {
		logging.PrintD(1, "Entering case check for %v", fileName)

		if len(date) >= 8 {
			logging.PrintD(3, "Input date length >= 8: %d", date)
			switch dateFmt {
			case enums.FILEDATE_YYYY_MM_DD:
				logging.PrintD(3, "Formatting as: yyyy-mm-dd")
				date = date[:4] + "-" + date[4:6] + "-" + date[6:8]

			case enums.FILEDATE_YY_MM_DD:
				logging.PrintD(3, "Formatting as: yy-mm-dd")
				date = date[2:4] + "-" + date[4:6] + "-" + date[6:8]

			case enums.FILEDATE_YYYY_DD_MM:
				logging.PrintD(3, "Formatting as: yyyy-dd-mm")
				date = date[:4] + "-" + date[6:8] + "-" + date[4:6]

			case enums.FILEDATE_YY_DD_MM:
				logging.PrintD(3, "Formatting as: yy-dd-mm")
				date = date[2:4] + "-" + date[6:8] + "-" + date[4:6]

			case enums.FILEDATE_DD_MM_YYYY:
				logging.PrintD(3, "Formatting as: dd-mm-yyyy")
				date = date[6:8] + "-" + date[4:6] + "-" + date[:4]

			case enums.FILEDATE_DD_MM_YY:
				logging.PrintD(3, "Formatting as: dd-mm-yy")
				date = date[6:8] + "-" + date[4:6] + "-" + date[2:4]

			case enums.FILEDATE_MM_DD_YYYY:
				logging.PrintD(3, "Formatting as: mm-dd-yyyy")
				date = date[4:6] + "-" + date[6:8] + "-" + date[:4]

			case enums.FILEDATE_MM_DD_YY:
				logging.PrintD(3, "Formatting as: mm-dd-yy")
				date = date[4:6] + "-" + date[6:8] + "-" + date[2:4]
			}
		} else if len(date) >= 6 {
			logging.PrintD(3, "Input date length >= 6 and < 8: %d", date)
			switch dateFmt {
			case enums.FILEDATE_YYYY_MM_DD, enums.FILEDATE_YY_MM_DD:
				date = date[:2] + "-" + date[2:4] + "-" + date[4:6]

			case enums.FILEDATE_YYYY_DD_MM, enums.FILEDATE_YY_DD_MM:
				date = date[:2] + "-" + date[4:6] + "-" + date[2:4]

			case enums.FILEDATE_DD_MM_YYYY, enums.FILEDATE_DD_MM_YY:
				date = date[4:6] + "-" + date[2:4] + "-" + date[:2]

			case enums.FILEDATE_MM_DD_YYYY, enums.FILEDATE_MM_DD_YY:
				date = date[2:4] + "-" + date[4:6] + "-" + date[:2]
			}
		} else if len(date) >= 4 {
			logging.PrintD(3, "Input date length >= 4 and < 6: %d", date)
			switch dateFmt {
			case enums.FILEDATE_YYYY_MM_DD,
				enums.FILEDATE_YY_MM_DD,
				enums.FILEDATE_MM_DD_YYYY,
				enums.FILEDATE_MM_DD_YY:

				date = date[:2] + "-" + date[2:4]

			case enums.FILEDATE_YYYY_DD_MM,
				enums.FILEDATE_YY_DD_MM,
				enums.FILEDATE_DD_MM_YYYY,
				enums.FILEDATE_DD_MM_YY:

				date = date[2:4] + "-" + date[:2]
			}
		}
	}
	dateTag := "[" + date + "]"

	logging.PrintD(1, "Made date tag '%s' from file '%v'", dateTag, filepath.Base(fileName))

	if dateTag != "[]" {
		if checkTagExists(dateTag, filepath.Base(fileName)) {
			logging.PrintD(2, "Tag '%s' already detected in name, skipping...", dateTag)
			dateTag = "[]"
		}
	} else {
		logging.PrintD(3, "Constructed empty tag, skipping tag exists check for file '%s'", filepath.Base(fileName))
	}

	return dateTag, nil
}

// makeFilenameTag creates the metatag string to prefix filenames with
func makeFilenameTag(jsonData map[string]interface{}, file *os.File) string {

	logging.PrintD(3, "Entering makeFilenameTag with data@ %v", jsonData)

	tagArray := cmd.GetStringSlice(keys.MFilenamePfx)
	tag := "["

	for field, value := range jsonData {
		for i, data := range tagArray {

			if field == data {
				tag += fmt.Sprintf(value.(string))
				logging.PrintD(3, "Added metafield %v data %v to prefix tag (Tag so far: %s)", field, data, tag)

				if i != len(tagArray)-1 {
					tag += "_"
				}
			}
		}
	}
	tag += "]"
	tag = strings.TrimSpace(tag)
	tag = strings.ToValidUTF8(tag, "")

	invalidChars := regexp.MustCompile(`[<>:"/\\|?*\x00-\x1F]`)
	tag = invalidChars.ReplaceAllString(tag, "")

	logging.PrintD(1, "Made metatag '%s' from file '%s'", tag, file.Name())

	if tag != "[]" {
		if checkTagExists(tag, filepath.Base(file.Name())) {
			logging.PrintD(2, "Tag '%s' already detected in name, skipping...", tag)
			tag = "[]"
		}
	}

	return tag
}

// checkTagExists checks if the constructed tag already exists in the filename
func checkTagExists(tag, filename string) bool {

	logging.PrintD(3, "Checking if tag '%s' exists in filename '%s'", tag, filename)

	return strings.Contains(filename, tag)
}
