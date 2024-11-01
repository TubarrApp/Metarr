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
	muPre  sync.Mutex
	muPost sync.Mutex
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

	// Set mutex until command execution
	muPre.Lock()

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

	args, err := buildCommand(m, tempOutputFilePath)
	if err != nil {
		// Unlock mutex
		muPre.Unlock()
		return err
	}

	command := exec.Command("ffmpeg", args...)

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
	muPre.Unlock()

	// Run the ffmpeg command
	logging.Print("%s!!! Starting FFmpeg command for '%s'...\n%s", consts.ColorCyan, m.FinalVideoBaseName, consts.ColorReset)
	if err := command.Run(); err != nil {
		logging.ErrorArray = append(logging.ErrorArray, err)
		return fmt.Errorf("failed to run ffmpeg command: %w", err)
	}

	// Lock second mutex after command executes
	muPost.Lock()
	defer muPost.Unlock()

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
