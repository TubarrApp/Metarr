package metadata

import (
	"Metarr/internal/config"
	consts "Metarr/internal/domain/constants"
	enums "Metarr/internal/domain/enums"
	keys "Metarr/internal/domain/keys"
	"Metarr/internal/models"
	backup "Metarr/internal/utils/fs/backup"
	logging "Metarr/internal/utils/logging"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// CommandBuilder handles FFmpeg command construction
type CommandBuilder struct {
	inputFile   string
	outputFile  string
	formatFlags []string
	gpuAccel    []string
	metadataMap map[string]string
}

// NewCommandBuilder creates a new FFmpeg command builder
func NewCommandBuilder(fd *models.FileData, outputFile string) *CommandBuilder {
	return &CommandBuilder{
		inputFile:   fd.OriginalVideoPath,
		outputFile:  outputFile,
		metadataMap: make(map[string]string),
	}
}

// buildCommand constructs the complete FFmpeg command
func buildCommand(fd *models.FileData, outputFile string) ([]string, error) {

	builder := NewCommandBuilder(fd, outputFile)

	builder.setGPUAcceleration()
	builder.addAllMetadata(fd)
	builder.setFormatFlags()

	// Return the fully appended argument string
	return builder.buildFinalCommand()
}

// ExecuteVideo writes metadata to a single video file
func ExecuteVideo(fd *models.FileData) error {

	if dontProcess(fd) {
		return nil
	}

	var tempOutputFilePath string

	dir := fd.VideoDirectory
	origPath := fd.OriginalVideoPath
	origExt := filepath.Ext(origPath)
	outExt := config.GetString(keys.OutputFiletype)

	fmt.Printf("\nWriting metadata for file: %s\n", origPath)

	// Make temp output path with .mp4 extension
	fileBase := strings.TrimSuffix(filepath.Base(origPath), filepath.Ext(origPath))

	if outExt == "" {
		// Set blank output file extension to just be the original file extension
		outExt = origExt
		config.Set(keys.OutputFiletype, outExt)
		tempOutputFilePath = filepath.Join(dir, consts.TempTag+fileBase+origExt+origExt)
	} else {
		tempOutputFilePath = filepath.Join(dir, consts.TempTag+fileBase+origExt+outExt)
	}

	logging.PrintD(3, "Orig ext: '%s', Out ext: '%s'", origExt, outExt)

	fd.TempOutputFilePath = tempOutputFilePath // Add to video file data struct

	defer func() {
		if _, err := os.Stat(tempOutputFilePath); err == nil {
			os.Remove(tempOutputFilePath)
		}
	}()

	// Build FFmpeg command
	args, err := buildCommand(fd, tempOutputFilePath)
	if err != nil {
		return err
	}

	command := exec.Command("ffmpeg", args...)
	logging.PrintI("%sConstructed FFmpeg command for%s '%s':\n\n%v\n", consts.ColorCyan, consts.ColorReset, fd.OriginalVideoPath, command.String())

	command.Stdout = os.Stdout
	command.Stderr = os.Stderr

	// Set final video path and base name in model
	fd.FinalVideoBaseName = strings.TrimSuffix(filepath.Base(origPath), filepath.Ext(origPath))
	fd.FinalVideoPath = filepath.Join(fd.VideoDirectory, fd.FinalVideoBaseName) + outExt

	logging.PrintI("Video file path data:\n\nOriginal Video Path: %s\nMetadata File Path: %s\nFinal Video Path: %s\n\nTemp Output Path: %s", origPath,
		fd.JSONFilePath,
		fd.FinalVideoPath,
		fd.TempOutputFilePath)

	// Run the ffmpeg command
	logging.Print("%s!!! Starting FFmpeg command for '%s'...\n%s", consts.ColorCyan, fd.FinalVideoBaseName, consts.ColorReset)
	if err := command.Run(); err != nil {
		logging.ErrorArray = append(logging.ErrorArray, err)
		return fmt.Errorf("failed to run FFmpeg command: %w", err)
	}

	// Rename temporary file to overwrite the original video file
	if filepath.Ext(origPath) != filepath.Ext(fd.FinalVideoPath) {
		logging.PrintI("Original file not type %s, removing '%s'", outExt, origPath)

	} else if config.GetBool(keys.NoFileOverwrite) && origPath == fd.FinalVideoPath {
		if err := makeBackup(origPath); err != nil {
			return err
		}
	}

	// Delete original after potential backup ops
	err = os.Remove(origPath)
	if err != nil {
		logging.ErrorArray = append(logging.ErrorArray, err)
		return fmt.Errorf("failed to remove original file (%s). Error: %v", origPath, err)
	}

	//
	err = os.Rename(tempOutputFilePath, fd.FinalVideoPath)
	if err != nil {
		return fmt.Errorf("failed to rename temp file: %w", err)
	}

	logging.PrintS(0, "Successfully processed video:\n\nOriginal file: %s\nNew file: %s\n\nTitle: %s", origPath,
		fd.FinalVideoPath,
		fd.MTitleDesc.Title)

	return nil
}

// dontProcess determines whether the program should process this video (meta already exists and file extensions are unchanged)
func dontProcess(fd *models.FileData) (dontProcess bool) {
	if fd.MetaAlreadyExists {

		logging.PrintI("Metadata already exists in the file, skipping processing...")
		origPath := fd.OriginalVideoPath
		fd.FinalVideoBaseName = strings.TrimSuffix(filepath.Base(origPath), filepath.Ext(origPath))

		// Set the final video path based on output extension
		outExt := config.GetString(keys.OutputFiletype)
		if outExt == "" {
			outExt = filepath.Ext(fd.OriginalVideoPath)
			config.Set(keys.OutputFiletype, outExt)
		}

		fd.FinalVideoPath = filepath.Join(fd.VideoDirectory, fd.FinalVideoBaseName) + outExt
		return true
	}
	return dontProcess
}

// makeBackup performs the backup
func makeBackup(origPath string) error {

	origInfo, err := os.Stat(origPath)
	if os.IsNotExist(err) {
		logging.PrintI("File does not exist, safe to proceed overwriting: %s", origPath)
		return nil
	}

	backupPath, err := backup.RenameToBackup(origPath)
	if err != nil {
		return fmt.Errorf("failed to rename original file and preserve file is on, aborting: %w", err)
	}

	backInfo, err := os.Stat(backupPath)
	if os.IsNotExist(err) {
		return fmt.Errorf("backup file was not created, aborting")
	}

	if origInfo.Size() != backInfo.Size() {
		return fmt.Errorf("backup file size does not match original, aborting")
	}

	return nil
}

// setGPUAcceleration sets appropriate GPU acceleration flags
func (b *CommandBuilder) setGPUAcceleration() {
	gpuFlag, ok := config.Get(keys.GPUEnum).(enums.SysGPU)
	if ok {
		switch gpuFlag {
		case enums.GPU_NVIDIA:
			b.gpuAccel = consts.NvidiaAccel[:]
		case enums.GPU_AMD:
			b.gpuAccel = consts.AMDAccel[:]
		case enums.GPU_INTEL:
			b.gpuAccel = consts.IntelAccel[:]
		}
	}
}

// addAllMetadata combines all metadata into a single map
func (b *CommandBuilder) addAllMetadata(fd *models.FileData) {

	b.addTitlesDescs(fd.MTitleDesc)
	b.addCredits(fd.MCredits)
	b.addDates(fd.MDates)
	b.addShowInfo(fd.MShowData)
	b.addOtherMetadata(fd.MOther)
}

// setFormatFlags adds commands specific for the extension input and output
func (b *CommandBuilder) setFormatFlags() {

	inExt := filepath.Ext(b.inputFile)
	outExt := config.GetString(keys.OutputFiletype)

	if outExt == "" {
		outExt = inExt
	}

	logging.PrintI("Input extension set '%s' and output extension '%s'. File: %s", inExt, outExt, b.inputFile)

	// Return early with straight copy if no extension change
	if strings.TrimPrefix(inExt, ".") == strings.TrimPrefix(outExt, ".") {
		b.formatFlags = consts.AVCodecCopy[:]
		return
	}

	flags := make([]string, 0)

	// Set flags based on output format requirements
	switch outExt {
	case ".mp4":
		flags = append(flags, "-f", outExt)
		flags = append(flags, consts.VideoToH264Balanced[:]...)
		flags = append(flags, consts.PixelFmtYuv420p[:]...)
		flags = append(flags, consts.AudioToAAC[:]...)
		flags = append(flags, consts.AudioBitrate[:]...)

	case ".mkv":
		flags = append(flags, "-f", outExt)
		// MKV is flexible, copy AV codec for supported formats
		if inExt == ".mp4" || inExt == ".m4v" {
			flags = append(flags, consts.VideoCodecCopy[:]...)
		} else {
			flags = append(flags, consts.VideoToH264Balanced[:]...)
		}
		flags = append(flags, consts.AudioToAAC[:]...)
		flags = append(flags, consts.AudioBitrate[:]...)

	case ".webm":
		flags = append(flags, "-f", outExt)
		flags = append(flags, consts.VideoToH264Balanced[:]...)
		flags = append(flags, consts.PixelFmtYuv420p[:]...)
		flags = append(flags, consts.KeyframeBalanced[:]...)
		flags = append(flags, consts.AudioToAAC[:]...)
		flags = append(flags, consts.AudioBitrate[:]...)

	default:
		// Safe defaults for any other output format
		flags = append(flags, "-f", outExt)
		flags = append(flags, consts.VideoToH264Balanced[:]...)
		flags = append(flags, consts.PixelFmtYuv420p[:]...)
		flags = append(flags, consts.AudioToAAC[:]...)
		flags = append(flags, consts.AudioBitrate[:]...)
	}

	b.formatFlags = flags
}

// buildFinalCommand assembles the final FFmpeg command
func (b *CommandBuilder) buildFinalCommand() ([]string, error) {

	// MAP LENGTH LOGIC:
	//
	// GPU acceleration flags
	// "-y", "i", input file, output file (+4)
	// Length of metadata map, then * 2 to prefix "-metadata" to each entry
	// Length of format flags
	// Output file (+1)

	args := make([]string, 0, len(b.gpuAccel)+4+len(b.metadataMap)*2+len(b.formatFlags)+1)

	// Add GPU acceleration if present
	args = append(args, b.gpuAccel...)

	// Add input file
	args = append(args, "-y", "-i", b.inputFile)

	// Add all -metadata commands
	for key, value := range b.metadataMap {
		args = append(args, "-metadata", fmt.Sprintf("%s=%s", key, strings.TrimSpace(value)))
	}

	// Add format flags
	args = append(args, b.formatFlags...)

	// Add output file
	args = append(args, b.outputFile)

	return args, nil
}
