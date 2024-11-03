package config

import (
	consts "Metarr/internal/domain/constants"
	enums "Metarr/internal/domain/enums"
	keys "Metarr/internal/domain/keys"
	logging "Metarr/internal/utils/logging"
	"fmt"
	"os"
	"strings"

	"github.com/shirou/gopsutil/mem"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var rootCmd = &cobra.Command{
	Use:   "metarr",
	Short: "Metarr is a video and metatagging tool",
	RunE: func(cmd *cobra.Command, args []string) error {
		if cmd.Flags().Lookup("help").Changed {
			return nil // Stop further execution if help is invoked
		}
		viper.Set("execute", true)
		return execute()
	},
}

// init sets the initial Viper settings
func init() {

	// Files and directories
	initFilesDirs()

	// System resource related
	initResourceRelated()

	// Filtering
	initFiltering()

	// All file transformations
	initAllFileTransformers()

	// Filename transformations
	initVideoTransformers()

	// Metadata and metafile manipulation
	initMetaTransformers()

	// Special functions
	initProgramFunctions()
}

// Execute is the primary initializer of Viper
func Execute() error {

	fmt.Println()

	err := rootCmd.Execute()
	if err != nil {
		logging.PrintE(0, "Failed to execute cobra")
		return err

	}
	return nil
}

// execute more thoroughly handles settings created in the Viper init
func execute() error {

	// Parse GPU settings and set commands
	verifyHWAcceleration()

	// Concurrency
	verifyConcurrencyLimit()

	// Resource usage limits (CPU and memory)
	verifyResourceLimits()

	// File extension settings
	verifyInputFiletypes()

	// File prefix filter settings
	verifyFilePrefixes()

	// Debugging level
	verifyDebugLevel()

	// Filetype to output as
	verifyOutputFiletype()

	// Meta overwrite and preserve flags
	verifyMetaOverwritePreserve()

	// Ensure no video and metadata location conflicts
	if err := checkFileDirs(); err != nil {
		return err
	}

	// Get presets
	switch viper.GetString(keys.InputPreset) {
	case "censoredtv":
		logging.PrintI("Setting preset settings for videos retrieved from Censored.tv")
		censoredTvPreset()
	default:
		// Do nothing
	}

	if err := initTextReplace(); err != nil {
		return err
	}

	if err := initDateReplaceFormat(); err != nil {
		return err
	}

	return nil
}

// checkFileDirConflicts ensures no conflicts in the file and directories entered by the user
func checkFileDirs() error {
	jsonFileSet := viper.IsSet(keys.JsonFile)
	jsonDirSet := viper.IsSet(keys.JsonDir)
	videoFileSet := viper.IsSet(keys.VideoFile)
	videoDirSet := viper.IsSet(keys.VideoDir)

	if jsonFileSet {
		if file, err := os.Stat(viper.GetString(keys.JsonFile)); err != nil {
			switch {
			case file.IsDir():
				return fmt.Errorf("entered directory '%s' as a file", viper.GetString(keys.JsonFile))
			case os.IsNotExist(err):
				return fmt.Errorf("file '%s' does not exist", viper.GetString(keys.JsonFile))
			}
		}
	}
	if jsonDirSet {
		if dir, err := os.Stat(viper.GetString(keys.JsonDir)); err != nil {
			switch {
			case !dir.IsDir():
				return fmt.Errorf("entered file '%s' as a directory", viper.GetString(keys.JsonDir))
			case os.IsNotExist(err):
				return fmt.Errorf("directory '%s' does not exist", viper.GetString(keys.JsonDir))
			}
		}
	}
	if videoFileSet {
		if file, err := os.Stat(viper.GetString(keys.VideoFile)); err != nil {
			switch {
			case file.IsDir():
				return fmt.Errorf("entered directory '%s' as a file", viper.GetString(keys.VideoFile))
			case os.IsNotExist(err):
				return fmt.Errorf("file '%s' does not exist", viper.GetString(keys.VideoFile))
			}
		}
	}
	if videoDirSet {
		if dir, err := os.Stat(viper.GetString(keys.VideoDir)); err != nil {
			switch {
			case !dir.IsDir():
				return fmt.Errorf("entered file '%s' as a directory", viper.GetString(keys.VideoDir))
			case os.IsNotExist(err):
				return fmt.Errorf("directory '%s' does not exist", viper.GetString(keys.VideoDir))
			}
		}
	}

	if jsonFileSet && jsonDirSet {
		return fmt.Errorf("cannot set both the JSON file and the JSON directory")
	}
	if jsonFileSet && videoDirSet {
		return fmt.Errorf("cannot set singular metadata file for whole video directory")
	}
	if videoFileSet && videoDirSet {
		return fmt.Errorf("cannot set singular video file AND video directory")
	}

	if videoFileSet {
		viper.Set(keys.SingleFile, true)
	}
	return nil
}

// verifyFilePrefixes checks and sets the file prefix filters
func verifyFilePrefixes() {
	var filePrefixes []string

	argInputPrefixes := viper.GetStringSlice(keys.FilePrefixes)
	for _, arg := range argInputPrefixes {
		if arg != "" {
			filePrefixes = append(filePrefixes, arg)
		}
	}
	if len(filePrefixes) > 0 {
		viper.Set(keys.FilePrefixes, filePrefixes)
	}
}

// verifyMetaOverwritePreserve checks if the entered meta overwrite and preserve flags are valid
func verifyMetaOverwritePreserve() {
	if GetBool(keys.MOverwrite) && GetBool(keys.MPreserve) {
		logging.PrintE(0, "Cannot enter both meta preserve AND meta overwrite, exiting...")
		os.Exit(1)
	}
}

// verifyDebugLevel checks and sets the debugging level to use
func verifyDebugLevel() {
	debugLevel := viper.GetUint16(keys.DebugLevel)
	if debugLevel > 3 {
		debugLevel = 3
	} else if debugLevel == 0 {
		logging.PrintI("Debugging level: %v", debugLevel)
	}
	viper.Set(keys.DebugLevel, debugLevel)
}

// verifyInputFiletypes checks that the inputted filetypes are accepted
func verifyInputFiletypes() {
	var inputExts []enums.ConvertFromFiletype

	argsInputExts := viper.GetStringSlice(keys.InputExts)

	for _, data := range argsInputExts {
		switch data {
		case "mkv":
			inputExts = append(inputExts, enums.IN_MKV)
		case "mp4":
			inputExts = append(inputExts, enums.IN_MP4)
		case "webm":
			inputExts = append(inputExts, enums.IN_WEBM)
		default:
			inputExts = append(inputExts, enums.IN_ALL_EXTENSIONS)
		}
	}
	if len(inputExts) == 0 {
		inputExts = append(inputExts, enums.IN_ALL_EXTENSIONS)
	}
	viper.Set(keys.InputExtsEnum, inputExts)
}

// verifyHWAcceleration checks and sets HW acceleration to use
func verifyHWAcceleration() {
	switch viper.GetString(keys.GPU) {
	case "nvidia":
		viper.Set(keys.GPUEnum, enums.NVIDIA)
		logging.Print("GPU acceleration selected by user: %v", keys.GPU)
	case "amd":
		viper.Set(keys.GPUEnum, enums.AMD)
		logging.Print("GPU acceleration selected by user: %v", keys.GPU)
	case "intel":
		viper.Set(keys.GPUEnum, enums.INTEL)
		logging.Print("GPU acceleration selected by user: %v", keys.GPU)
	default:
		viper.Set(keys.GPUEnum, enums.NO_HW_ACCEL)
	}
}

// verifyConcurrencyLimit checks and ensures correct concurrency limit input
func verifyConcurrencyLimit() {
	maxConcurrentProcesses := viper.GetInt(keys.Concurrency)

	switch {
	case maxConcurrentProcesses < 1:
		maxConcurrentProcesses = 1
		logging.PrintE(2, "Max concurrency set too low, set to minimum value: %d", maxConcurrentProcesses)
	default:
		logging.PrintI("Max concurrency: %d", maxConcurrentProcesses)
	}
	viper.Set(keys.Concurrency, maxConcurrentProcesses)
}

// verifyCPUUsage verifies the value used to limit the CPU needed to spawn a new routine
func verifyResourceLimits() {
	MinMemUsage := viper.GetUint64(keys.MinMem)
	MinMemUsage *= 1024 * 1024 // Convert input to MB

	currentAvailableMem, err := mem.VirtualMemory()
	if err != nil {
		logging.PrintE(0, "Could not get system memory, using default max RAM requirements", err)
		currentAvailableMem.Available = 1024
	}
	if MinMemUsage > currentAvailableMem.Available {
		MinMemUsage = currentAvailableMem.Available
	}

	if MinMemUsage > 0 {
		logging.PrintI("Min RAM to spawn process: %v", MinMemUsage)
	}
	viper.Set(keys.MinMemMB, MinMemUsage)

	maxCPUUsage := viper.GetFloat64(keys.MaxCPU)
	switch {
	case maxCPUUsage > 100.0:
		maxCPUUsage = 100.0
		logging.PrintE(2, "Max CPU usage entered too high, setting to default max: %.2f%%", maxCPUUsage)

	case maxCPUUsage < 1.0:
		maxCPUUsage = 10.0
		logging.PrintE(0, "Max CPU usage entered too low, setting to default low: %.2f%%", maxCPUUsage)
	}
	if maxCPUUsage != 100.0 {
		logging.PrintI("Max CPU usage: %.2f%%", maxCPUUsage)
	}
	viper.Set(keys.MaxCPU, maxCPUUsage)
}

// Verify the output filetype is valid for FFmpeg
func verifyOutputFiletype() {
	o := GetString(keys.OutputFiletype)
	o = strings.TrimSpace(o)

	if !strings.HasPrefix(o, ".") {
		o = "." + o
		Set(keys.OutputFiletype, o)
	}

	valid := false
	for _, ext := range consts.AllVidExtensions {
		if o != ext {
			continue
		}
		valid = true
		break
	}

	switch valid {
	case true:
		logging.PrintI("Outputting files as %s", o)
	default:
		Set(keys.OutputFiletype, "")
	}
}
