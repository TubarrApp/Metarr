package cfg

import (
	"fmt"
	consts "metarr/internal/domain/constants"
	enums "metarr/internal/domain/enums"
	keys "metarr/internal/domain/keys"
	logging "metarr/internal/utils/logging"
	"os"
	"strings"

	"github.com/shirou/gopsutil/mem"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var rootCmd = &cobra.Command{
	Use:   "metarr",
	Short: "metarr is a video and metatagging tool",
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
		logging.E(0, "Failed to execute cobra")
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

	// Verify user metafile purge settings
	verifyPurgeMetafiles()

	// Ensure no video and metadata location conflicts
	if err := checkFileDirs(); err != nil {
		return err
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

	videoFile := strings.TrimSpace(viper.GetString(keys.VideoFile))
	videoFileSet := viper.IsSet(keys.VideoFile)

	videoDir := strings.TrimSpace(viper.GetString(keys.VideoDir))
	videoDirSet := viper.IsSet(keys.VideoDir)

	jsonFile := strings.TrimSpace(viper.GetString(keys.JsonFile))
	jsonFileSet := viper.IsSet(keys.JsonFile)

	jsonDir := strings.TrimSpace(viper.GetString(keys.JsonDir))
	jsonDirSet := viper.IsSet(keys.JsonDir)

	// Validate configuration
	if jsonFileSet {
		if jsonDirSet {
			return fmt.Errorf("cannot set both the JSON file and the JSON directory")
		}
		if videoDirSet {
			return fmt.Errorf("cannot set singular metadata file for whole video directory")
		}
	}
	if videoFileSet {
		if videoDirSet {
			return fmt.Errorf("cannot set singular video file AND video directory")
		}
		viper.Set(keys.SingleFile, true)
	}

	// Check files and dirs exist
	if viper.IsSet(keys.JsonFile) {
		if _, err := os.Stat(jsonFile); err != nil {
			return fmt.Errorf("file '%s' does not exist", jsonFile)
		}
		if fileInfo, _ := os.Stat(jsonFile); fileInfo.IsDir() {
			return fmt.Errorf("entered directory '%s' as a file", jsonFile)
		}
	}

	if viper.IsSet(keys.JsonDir) {
		if _, err := os.Stat(jsonDir); err != nil {
			return fmt.Errorf("directory '%s' does not exist", jsonDir)
		}
		if fileInfo, _ := os.Stat(jsonDir); !fileInfo.IsDir() {
			return fmt.Errorf("entered file '%s' as a directory", jsonDir)
		}
	}

	if viper.IsSet(keys.VideoFile) {
		if _, err := os.Stat(videoFile); err != nil {
			return fmt.Errorf("file '%s' does not exist", videoFile)
		}
		if fileInfo, _ := os.Stat(videoFile); fileInfo.IsDir() {
			return fmt.Errorf("entered directory '%s' as a file", videoFile)
		}
	}

	if viper.IsSet(keys.VideoDir) {
		if _, err := os.Stat(videoDir); err != nil {
			return fmt.Errorf("directory '%s' does not exist", videoDir)
		}
		if fileInfo, _ := os.Stat(videoDir); !fileInfo.IsDir() {
			return fmt.Errorf("entered file '%s' as a directory", videoDir)
		}
	}

	return nil
}

// verifyFilePrefixes checks and sets the file prefix filters
func verifyFilePrefixes() {
	if !viper.IsSet(keys.FilePrefixes) {
		return
	}

	argsInputPrefixes := viper.GetStringSlice(keys.FilePrefixes)
	filePrefixes := make([]string, 0, len(argsInputPrefixes))

	for _, arg := range argsInputPrefixes {
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
		logging.E(0, "Cannot enter both meta preserve AND meta overwrite, exiting...")
		os.Exit(1)
	}
}

// verifyDebugLevel checks and sets the debugging level to use
func verifyDebugLevel() {
	debugLevel := viper.GetUint16(keys.DebugLevel)
	if debugLevel > 5 {
		debugLevel = 5
	} else if debugLevel == 0 {
		logging.I("Debugging level: %v", debugLevel)
	}
	viper.Set(keys.DebugLevel, debugLevel)
	logging.Level = int(debugLevel)
}

// verifyInputFiletypes checks that the inputted filetypes are accepted
func verifyInputFiletypes() {
	argsVInputExts := viper.GetStringSlice(keys.InputVideoExts)
	inputVExts := make([]enums.ConvertFromFiletype, 0, len(argsVInputExts))

	for _, data := range argsVInputExts {
		switch data {
		case "mkv":
			inputVExts = append(inputVExts, enums.VID_EXTS_MKV)
		case "mp4":
			inputVExts = append(inputVExts, enums.VID_EXTS_MP4)
		case "webm":
			inputVExts = append(inputVExts, enums.VID_EXTS_WEBM)
		default:
			inputVExts = append(inputVExts, enums.VID_EXTS_ALL)
		}
	}
	if len(inputVExts) == 0 {
		inputVExts = append(inputVExts, enums.VID_EXTS_ALL)
	}
	logging.D(2, "Received video input extension filter: %v", inputVExts)
	viper.Set(keys.InputVExtsEnum, inputVExts)

	argsMInputExts := viper.GetStringSlice(keys.InputMetaExts)
	inputMExts := make([]enums.MetaFiletypeFilter, 0, len(argsMInputExts))

	for _, data := range argsMInputExts {
		switch data {
		case "json":
			inputMExts = append(inputMExts, enums.META_EXTS_JSON)
		case "nfo":
			inputMExts = append(inputMExts, enums.META_EXTS_NFO)
		default:
			inputMExts = append(inputMExts, enums.META_EXTS_ALL)
		}
	}
	if len(inputMExts) == 0 {
		inputMExts = append(inputMExts, enums.META_EXTS_ALL)
	}
	logging.D(2, "Received meta input extension filter: %v", inputMExts)
	viper.Set(keys.InputMExtsEnum, inputMExts)
}

// verifyHWAcceleration checks and sets HW acceleration to use
func verifyHWAcceleration() {
	switch viper.GetString(keys.GPU) {
	case "nvidia":
		viper.Set(keys.GPUEnum, enums.GPU_NVIDIA)
		logging.P("GPU acceleration selected by user: %v", keys.GPU)
	case "amd":
		viper.Set(keys.GPUEnum, enums.GPU_AMD)
		logging.P("GPU acceleration selected by user: %v", keys.GPU)
	case "intel":
		viper.Set(keys.GPUEnum, enums.GPU_INTEL)
		logging.P("GPU acceleration selected by user: %v", keys.GPU)
	default:
		viper.Set(keys.GPUEnum, enums.GPU_NO_HW_ACCEL)
	}
}

// verifyConcurrencyLimit checks and ensures correct concurrency limit input
func verifyConcurrencyLimit() {
	maxConcurrentProcesses := viper.GetInt(keys.Concurrency)

	switch {
	case maxConcurrentProcesses < 1:
		maxConcurrentProcesses = 1
		logging.E(2, "Max concurrency set too low, set to minimum value: %d", maxConcurrentProcesses)
	default:
		logging.I("Max concurrency: %d", maxConcurrentProcesses)
	}
	viper.Set(keys.Concurrency, maxConcurrentProcesses)
}

// verifyCPUUsage verifies the value used to limit the CPU needed to spawn a new routine
func verifyResourceLimits() {
	MinMemUsage := viper.GetUint64(keys.MinMem)
	MinMemUsage *= 1024 * 1024 // Convert input to MB

	currentAvailableMem, err := mem.VirtualMemory()
	if err != nil {
		logging.E(0, "Could not get system memory, using default max RAM requirements", err)
		currentAvailableMem.Available = 1024
	}
	if MinMemUsage > currentAvailableMem.Available {
		MinMemUsage = currentAvailableMem.Available
	}

	if MinMemUsage > 0 {
		logging.I("Min RAM to spawn process: %v", MinMemUsage)
	}
	viper.Set(keys.MinMemMB, MinMemUsage)

	maxCPUUsage := viper.GetFloat64(keys.MaxCPU)
	switch {
	case maxCPUUsage > 100.0:
		maxCPUUsage = 100.0
		logging.E(2, "Max CPU usage entered too high, setting to default max: %.2f%%", maxCPUUsage)

	case maxCPUUsage < 1.0:
		maxCPUUsage = 10.0
		logging.E(0, "Max CPU usage entered too low, setting to default low: %.2f%%", maxCPUUsage)
	}
	if maxCPUUsage != 100.0 {
		logging.I("Max CPU usage: %.2f%%", maxCPUUsage)
	}
	viper.Set(keys.MaxCPU, maxCPUUsage)
}

// Verify the output filetype is valid for FFmpeg
func verifyOutputFiletype() {
	if !viper.IsSet(keys.OutputFiletype) {
		return
	}

	o := GetString(keys.OutputFiletypeInput)
	o = strings.TrimSpace(o)

	if !strings.HasPrefix(o, ".") {
		o = "." + o
		viper.Set(keys.OutputFiletype, o)
	}

	valid := false
	for _, ext := range consts.AllVidExtensions {
		if o != ext {
			continue
		} else {
			valid = true
			break
		}
	}

	if valid {
		logging.I("Outputting files as %s", o)
	}
}

// verifyPurgeMetafiles checks and sets the type of metafile purge to perform
func verifyPurgeMetafiles() {
	if !viper.IsSet(keys.MetaPurge) {
		return
	}

	var e enums.PurgeMetafiles
	purgeType := viper.GetString(keys.MetaPurge)

	purgeType = strings.TrimSpace(purgeType)
	purgeType = strings.ToLower(purgeType)
	purgeType = strings.ReplaceAll(purgeType, ".", "")

	switch purgeType {
	case "all":
		e = enums.PURGEMETA_ALL
	case "json":
		e = enums.PURGEMETA_JSON
	case "nfo":
		e = enums.PURGEMETA_NFO
	default:
		e = enums.PURGEMETA_NONE
	}

	viper.Set(keys.MetaPurgeEnum, e)
}