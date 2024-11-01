package config

import (
	enums "Metarr/internal/domain/enums"
	keys "Metarr/internal/domain/keys"
	logging "Metarr/internal/utils/logging"
	"fmt"
	"strings"

	"github.com/shirou/gopsutil/mem"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var metaReplaceSuffixInput []string
var metaReplacePrefixInput []string
var metaNewFieldInput []string
var filenameReplaceSuffixInput []string

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

	// Video directory
	rootCmd.PersistentFlags().StringP(keys.VideoDir, "v", ".", "Video directory")
	viper.BindPFlag(keys.VideoDir, rootCmd.PersistentFlags().Lookup(keys.VideoDir))
	// Video file
	rootCmd.PersistentFlags().StringP(keys.VideoFile, "V", ".", "Video file")
	viper.BindPFlag(keys.VideoFile, rootCmd.PersistentFlags().Lookup(keys.VideoFile))

	// JSON directory
	rootCmd.PersistentFlags().StringP(keys.JsonDir, "j", ".", "JSON directory")
	viper.BindPFlag(keys.JsonDir, rootCmd.PersistentFlags().Lookup(keys.JsonDir))
	// JSON file
	rootCmd.PersistentFlags().StringP(keys.JsonFile, "J", ".", "JSON file")
	viper.BindPFlag(keys.JsonFile, rootCmd.PersistentFlags().Lookup(keys.JsonFile))

	// Rename choice
	rootCmd.PersistentFlags().StringP(keys.RenameStyle, "r", "skip", "Rename flag (spaces, underscores, or skip)")
	viper.BindPFlag(keys.RenameStyle, rootCmd.PersistentFlags().Lookup(keys.RenameStyle))

	// Concurrency limit
	rootCmd.PersistentFlags().IntP(keys.Concurrency, "l", 5, "Max concurrency limit")
	viper.BindPFlag(keys.Concurrency, rootCmd.PersistentFlags().Lookup(keys.Concurrency))

	// CPU usage
	rootCmd.PersistentFlags().Float64P(keys.MaxCPU, "c", 100.0, "Max CPU usage")
	viper.BindPFlag(keys.MaxCPU, rootCmd.PersistentFlags().Lookup(keys.MaxCPU))

	// Min memory
	rootCmd.PersistentFlags().Uint64P(keys.MinMem, "m", 0, "Minimum RAM to start process")
	viper.BindPFlag(keys.MinMem, rootCmd.PersistentFlags().Lookup(keys.MinMem))

	// File extensions to convert
	rootCmd.PersistentFlags().StringSliceP(keys.InputExts, "e", []string{"all"}, "File extensions to convert (all, mkv, mp4, webm)")
	viper.BindPFlag(keys.InputExts, rootCmd.PersistentFlags().Lookup(keys.InputExts))

	// Only convert files with prefix
	rootCmd.PersistentFlags().StringSliceP(keys.FilePrefixes, "p", []string{""}, "Filters files by prefixes")
	viper.BindPFlag(keys.FilePrefixes, rootCmd.PersistentFlags().Lookup(keys.FilePrefixes))

	// Hardware acceleration
	rootCmd.PersistentFlags().StringP(keys.GPU, "g", "none", "GPU acceleration type (nvidia, amd, intel, none)")
	viper.BindPFlag(keys.GPU, rootCmd.PersistentFlags().Lookup(keys.GPU))

	// Debugging level
	rootCmd.PersistentFlags().Uint16P(keys.DebugLevel, "d", 0, "Level of debugging (0 - 3)")
	viper.BindPFlag(keys.DebugLevel, rootCmd.PersistentFlags().Lookup(keys.DebugLevel))

	// Metadata replacement
	rootCmd.PersistentFlags().StringSliceVar(&metaReplaceSuffixInput, "meta-replace-suffix", nil, "Trim suffixes from metadata fields (metatag:fieldsuffix:replacement)")
	rootCmd.PersistentFlags().StringSliceVar(&metaReplacePrefixInput, "meta-replace-prefix", nil, "Trim prefixes from metadata fields (metatag:fieldprefix:replacement)")
	rootCmd.PersistentFlags().StringSliceVar(&metaNewFieldInput, "meta-add-field", nil, "Add new fields into metadata files (metatag:value)")

	rootCmd.PersistentFlags().Bool(keys.MDescDatePfx, false, "Adds the date to the start of the description field.")
	viper.BindPFlag(keys.MDescDatePfx, rootCmd.PersistentFlags().Lookup(keys.MDescDatePfx))
	rootCmd.PersistentFlags().Bool(keys.MDescDateSfx, false, "Adds the date to the end of the description field.")
	viper.BindPFlag(keys.MDescDateSfx, rootCmd.PersistentFlags().Lookup(keys.MDescDateSfx))

	// Filename transformations
	rootCmd.PersistentFlags().StringSlice(keys.MFilenamePfx, nil, "Adds a specified metatag's value onto the start of the filename")
	viper.BindPFlag(keys.MFilenamePfx, rootCmd.PersistentFlags().Lookup(keys.MFilenamePfx))

	rootCmd.PersistentFlags().StringSliceVar(&filenameReplaceSuffixInput, keys.InputFilenameReplaceSfx, nil, "Replaces a specified suffix on filenames. (suffix:replacement)")
	viper.BindPFlag(keys.InputFilenameReplaceSfx, rootCmd.PersistentFlags().Lookup(keys.InputFilenameReplaceSfx))

	rootCmd.PersistentFlags().String(keys.InputFileDatePfx, "", "Looks for dates in metadata to prefix the video with. (date:format [e.g. Ymd for yyyy-mm-dd])")
	viper.BindPFlag(keys.InputFileDatePfx, rootCmd.PersistentFlags().Lookup(keys.InputFileDatePfx))

	rootCmd.PersistentFlags().StringP(keys.OutputFiletype, "o", "", "File extension to output as (mp4 works best for most media servers)")
	viper.BindPFlag(keys.OutputFiletype, rootCmd.PersistentFlags().Lookup(keys.OutputFiletype))

	// Special functions
	rootCmd.PersistentFlags().Bool(keys.SkipVideos, false, "Skips compiling/transcoding the videos and just edits the file names/JSON file fields")
	viper.BindPFlag(keys.SkipVideos, rootCmd.PersistentFlags().Lookup(keys.SkipVideos))

	rootCmd.PersistentFlags().Bool(keys.MOverwrite, false, "When adding new metadata fields, automatically overwrite existing fields with your new values")
	viper.BindPFlag(keys.MOverwrite, rootCmd.PersistentFlags().Lookup(keys.MOverwrite))

	rootCmd.PersistentFlags().Bool(keys.MPreserve, false, "When adding new metadata fields, skip already existent fields")
	viper.BindPFlag(keys.MPreserve, rootCmd.PersistentFlags().Lookup(keys.MPreserve))

	rootCmd.PersistentFlags().BoolP(keys.NoFileOverwrite, "n", false, "Renames the original files to avoid overwriting")
	viper.BindPFlag(keys.NoFileOverwrite, rootCmd.PersistentFlags().Lookup(keys.NoFileOverwrite))

	rootCmd.PersistentFlags().String(keys.GetLatest, "", "Grabs new videos released since the last stored URL in the grabbed-urls.txt folder (parameter should be the channel URL)")
	viper.BindPFlag(keys.GetLatest, rootCmd.PersistentFlags().Lookup(keys.GetLatest))

	rootCmd.PersistentFlags().String(keys.InputPreset, "", "Use a preset configuration (e.g. censoredtv)")
	viper.BindPFlag(keys.InputPreset, rootCmd.PersistentFlags().Lookup(keys.InputPreset))

	rootCmd.PersistentFlags().String(keys.MoveOnComplete, "", "Move files to given directory on program completion")
	viper.BindPFlag(keys.MoveOnComplete, rootCmd.PersistentFlags().Lookup(keys.MoveOnComplete))
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
	case "none":
		viper.Set(keys.GPUEnum, enums.NO_HW_ACCEL)
	default:
		return fmt.Errorf("invalid hardware acceleration option")
	}

	// Concurrency
	maxConcurrentProcesses := viper.GetInt(keys.Concurrency)

	switch {
	case maxConcurrentProcesses < 1:
		maxConcurrentProcesses = 1
		logging.PrintE(2, "Max concurrency set too low, set to minimum value: %d", maxConcurrentProcesses)
	default:
		logging.PrintI("Max concurrency: %d", maxConcurrentProcesses)
	}
	viper.Set(keys.Concurrency, maxConcurrentProcesses)

	// CPU Usage
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

	// Minimum memory
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

	// File extension settings
	var inputExts []enums.ConvertFromFiletype

	argsInputExts := viper.GetStringSlice(keys.InputExts)

	for _, data := range argsInputExts {
		switch data {
		case "all":
			inputExts = append(inputExts, enums.IN_ALL_EXTENSIONS)
		case "mkv":
			inputExts = append(inputExts, enums.IN_MKV)
		case "mp4":
			inputExts = append(inputExts, enums.IN_MP4)
		case "webm":
			inputExts = append(inputExts, enums.IN_WEBM)
		default:
			return fmt.Errorf("invalid input file extension filters selected")
		}
	}

	if len(inputExts) == 0 {
		inputExts = append(inputExts, enums.IN_ALL_EXTENSIONS)
	}
	viper.Set(keys.InputExtsEnum, inputExts)

	// File prefix filter settings
	var filePrefixes []string

	argInputPrefixes := viper.GetStringSlice(keys.FilePrefixes)
	filePrefixes = append(filePrefixes, argInputPrefixes...)

	viper.Set(keys.FilePrefixes, filePrefixes)

	// Debugging level
	debugLevel := viper.GetUint16(keys.DebugLevel)
	if debugLevel > 3 {
		debugLevel = 3
	} else if debugLevel == 0 {
		logging.PrintI("Debugging level: %v", debugLevel)
	}
	viper.Set(keys.DebugLevel, debugLevel)

	// Ensure no video and metadata location conflicts
	jsonFileSet := viper.IsSet(keys.JsonFile)
	jsonDirSet := viper.IsSet(keys.JsonDir)
	videoFileSet := viper.IsSet(keys.VideoFile)
	videoDirSet := viper.IsSet(keys.VideoDir)

	if jsonFileSet && jsonDirSet || jsonFileSet && videoDirSet {
		return fmt.Errorf("cannot set singular metadata file for whole video directory")
	}
	if videoFileSet && videoDirSet {
		return fmt.Errorf("cannot set singular video file AND video directory")
	}

	if videoFileSet {
		viper.Set(keys.SingleFile, true)
	}

	// Get presets

	switch viper.GetString(keys.InputPreset) {
	case "censoredtv":
		logging.PrintI("Setting preset settings for videos retrieved from Censored.tv")
		censoredTvPreset()
	default:
		// Do nothing
	}

	verifyOutputFiletype()

	err = initTextReplace()
	if err != nil {
		return err
	}

	err = initDateReplaceFormat()
	if err != nil {
		return err
	}

	return nil
}

// Verify the output filetype is valid for FFmpeg
func verifyOutputFiletype() {
	o := GetString(keys.OutputFiletype)
	if !strings.HasPrefix(o, ".") {
		o = "." + o
		Set(keys.OutputFiletype, o)
	}
	switch o {
	case ".3gp", ".avi", ".f4v", ".flv", ".m4v", ".mkv",
		".mov", ".mp4", ".mpeg", ".mpg", ".ogm", ".ogv",
		".ts", ".vob", ".webm", ".wmv":
		logging.PrintI("Outputting files as %s", o)
	default:
		Set(keys.OutputFiletype, "")
	}
}
