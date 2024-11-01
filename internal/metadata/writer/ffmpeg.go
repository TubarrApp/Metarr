package metadata

import (
	"Metarr/internal/config"
	consts "Metarr/internal/domain/constants"
	enums "Metarr/internal/domain/enums"
	keys "Metarr/internal/domain/keys"
	"Metarr/internal/types"
	backup "Metarr/internal/utils/fs/backup"
	logging "Metarr/internal/utils/logging"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
)

var (
	muWriteCommand sync.Mutex
)

// MetadataMap holds all metadata key-value pairs
type MetadataMap map[string]string

// CommandBuilder handles FFmpeg command construction
type CommandBuilder struct {
	inputFile   string
	outputFile  string
	gpuAccel    []string
	metadataMap MetadataMap
	formatFlags []string
}

// NewCommandBuilder creates a new FFmpeg command builder
func NewCommandBuilder(m *types.FileData, outputFile string) *CommandBuilder {
	return &CommandBuilder{
		inputFile:   m.OriginalVideoPath,
		outputFile:  outputFile,
		metadataMap: make(MetadataMap),
	}
}

// buildCommand constructs the complete FFmpeg command
func buildCommand(m *types.FileData, outputFile string) ([]string, error) {

	builder := NewCommandBuilder(m, outputFile)

	builder.setGPUAcceleration()

	builder.addAllMetadata(m)

	builder.setFormatFlags()

	// Return the fully appended argument string
	return builder.buildFinalCommand()
}

// WriteMetadata writes metadata to a single video file
func WriteMetadata(m *types.FileData) error {

	var originalVPath string = m.OriginalVideoPath
	dir := m.VideoDirectory

	fmt.Printf("\nWriting metadata for file: %s\n", originalVPath)

	// Make temp output path with .mp4 extension
	fileBase := strings.TrimSuffix(filepath.Base(originalVPath), filepath.Ext(originalVPath))

	var tempOutputFilePath string
	originalExt := filepath.Ext(originalVPath)
	outputExt := config.GetString(keys.OutputFiletype)
	switch {
	case outputExt != "":
		tempOutputFilePath = filepath.Join(dir, consts.TempTag+fileBase+originalExt+outputExt)
	default:
		tempOutputFilePath = filepath.Join(dir, consts.TempTag+fileBase+originalExt+originalExt)
	}

	m.TempOutputFilePath = tempOutputFilePath // Add to video file data struct

	defer func() {
		if _, err := os.Stat(tempOutputFilePath); err == nil {
			os.Remove(tempOutputFilePath)
		}
	}()

	muWriteCommand.Lock()
	args, err := buildCommand(m, tempOutputFilePath)
	if err != nil {
		muWriteCommand.Unlock()
		return err
	}

	command := exec.Command("ffmpeg", args...)
	muWriteCommand.Unlock()

	logging.PrintI("\n%sConstructed FFmpeg command for%s '%s':\n\n%v\n", consts.ColorCyan, consts.ColorReset, m.OriginalVideoPath, command.String())

	command.Stdout = os.Stdout
	command.Stderr = os.Stderr

	origPath := originalVPath
	m.FinalVideoBaseName = strings.TrimSuffix(filepath.Base(origPath), filepath.Ext(origPath))

	switch {
	case outputExt != "":
		m.FinalVideoPath = filepath.Join(m.VideoDirectory, m.FinalVideoBaseName) + outputExt
	default:
		m.FinalVideoPath = filepath.Join(m.VideoDirectory, m.FinalVideoBaseName) + originalExt
	}

	fmt.Printf("\n\nVideo file path data:\n\nOriginal Video Path: %s\nMetadata File Path: %s\nFinal Video Path: %s\n\nTemp Output Path: %s\n\n", originalVPath,
		m.JSONFilePath,
		m.FinalVideoPath,
		m.TempOutputFilePath)

	// Unlock mutex

	// Run the ffmpeg command
	logging.Print("%s!!! Starting FFmpeg command for '%s'...\n%s", consts.ColorCyan, m.FinalVideoBaseName, consts.ColorReset)
	if err := command.Run(); err != nil {
		logging.ErrorArray = append(logging.ErrorArray, err)
		return fmt.Errorf("failed to run ffmpeg command: %w", err)
	}

	// Rename temporary file to overwrite the original video file:
	// First check overwrite rules
	if config.GetBool(keys.NoFileOverwrite) && originalVPath == m.FinalVideoPath {
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

		if config.GetBool(keys.NoFileOverwrite) {
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

	logging.PrintS(0, "\nSuccessfully processed video:\n\nOriginal file: %s\nNew file: %s\n\nTitle: %s\n\n", originalVPath,
		m.FinalVideoPath,
		m.MTitleDesc.Title)

	return nil
}

// setGPUAcceleration sets appropriate GPU acceleration flags
func (b *CommandBuilder) setGPUAcceleration() {
	gpuFlag, ok := config.Get(keys.GPUEnum).(enums.SysGPU)
	if ok {
		switch gpuFlag {
		case enums.NVIDIA:
			b.gpuAccel = consts.NvidiaAccel
		case enums.AMD:
			b.gpuAccel = consts.AMDAccel
		case enums.INTEL:
			b.gpuAccel = consts.IntelAccel
		}
	}
}

// addAllMetadata combines all metadata into a single map
func (b *CommandBuilder) addAllMetadata(m *types.FileData) {

	b.addTitlesDescs(m.MTitleDesc)
	b.addCredits(m.MCredits)
	b.addDates(m.MDates)
	b.addShowInfo(m.MShowData)
	b.addOtherMetadata(m.MOther)
}

// setFormatFlags sets format-specific encoding flags
func (b *CommandBuilder) setFormatFlags() {
	ext := filepath.Ext(b.inputFile)
	switch ext {
	case ".mp4":
		b.formatFlags = consts.AVCodecCopy
	case ".mkv":
		flags := make([]string, 0)
		flags = append(flags, consts.OutputExt...)
		flags = append(flags, consts.VideoCodecCopy...)
		flags = append(flags, consts.AudioToAAC...)
		flags = append(flags, consts.AudioBitrate...)
		b.formatFlags = flags
	case ".webm":
		flags := make([]string, 0)
		flags = append(flags, consts.OutputExt...)
		flags = append(flags, consts.VideoToH264Balanced...)
		flags = append(flags, consts.PixelFmtYuv420p...)
		flags = append(flags, consts.KeyframeBalanced...)
		flags = append(flags, consts.AudioToAAC...)
		flags = append(flags, consts.AudioBitrate...)
		b.formatFlags = flags
	}
}

// buildFinalCommand assembles the final FFmpeg command
func (b *CommandBuilder) buildFinalCommand() ([]string, error) {
	args := make([]string, 0, len(b.gpuAccel)+4+len(b.metadataMap)*2+len(b.formatFlags)+1)

	// Add GPU acceleration if present
	args = append(args, b.gpuAccel...)

	// Add input file
	args = append(args, "-y", "-i", b.inputFile)

	// Add all metadata in a single batch
	for key, value := range b.metadataMap {
		args = append(args, "-metadata", fmt.Sprintf("%s=%s", key, fieldFormatter(value)))
	}

	// Add format flags
	args = append(args, b.formatFlags...)

	// Add output file
	args = append(args, b.outputFile)

	return args, nil
}

// fieldFormatter cleans field values
func fieldFormatter(value string) string {
	return strings.TrimSpace(value)
}
