package cfg

import (
	"fmt"
	"metarr/internal/domain/keys"
	"metarr/internal/utils/validation"
	"os"
	"strings"

	"github.com/spf13/viper"
)

// init sets the initial Viper settings
func init() {
	// Env vars
	viper.AutomaticEnv()
	viper.SetEnvKeyReplacer(strings.NewReplacer("_", "-")) // Convert "video_directory" to "video-directory"

	// Config file
	rootCmd.PersistentFlags().String(keys.ConfigPath, "", "Specify a path to your preset configuration file")
	if err := viper.BindPFlag(keys.ConfigPath, rootCmd.PersistentFlags().Lookup(keys.ConfigPath)); err != nil {
		fmt.Fprintf(os.Stderr, "config file path setting failure: %v\n", err)
		os.Exit(1)
	}

	// Files and directories
	initOrExit(initFilesDirs(),
		"files & dirs initialization failure")

	// System resource related
	initOrExit(initResourceRelated(),
		"config resource element initialization failure")

	// Filtering
	initOrExit(initFiltering(),
		"config filtering initialization failure")

	// All file transformations
	initOrExit(initAllFileTransformers(),
		"config file transformer initialization failure")

	// Filename transformations
	initOrExit(initVideoTransformers(),
		"config video transformer initialization failure")

	// Metadata and metafile manipulation
	initOrExit(initMetaTransformers(),
		"config meta transformer initialization failure")

	// Special functions
	initOrExit(initProgramFunctions(),
		"config program function initialization failure")
}

// execute more thoroughly handles settings created in the Viper init
func execute() error {

	// Concurrency
	validation.ValidateConcurrencyLimit(viper.GetInt(keys.Concurrency))

	// Resource usage limits (CPU and memory)
	validation.ValidateMinFreeMem(viper.GetString(keys.MinFreeMem))
	validation.ValidateMaxCPU(viper.GetFloat64(keys.MaxCPU))

	// File extension settings
	validation.ValidateInputFiletypes(
		viper.GetStringSlice(keys.InputVideoExts),
		viper.GetStringSlice(keys.InputMetaExts),
	)

	// File prefix filter settings
	if viper.IsSet(keys.FilePrefixes) {
		validation.ValidateFilePrefixes(viper.GetStringSlice(keys.FilePrefixes))
	}

	// Debugging level
	validation.ValidateDebugLevel(viper.GetInt(keys.DebugLevel))

	// Filetype to output as
	if viper.IsSet(keys.OutputFiletypeInput) {
		validation.ValidateOutputFiletype(viper.GetString(keys.OutputFiletypeInput))
	}

	// Meta overwrite and preserve flags
	validation.ValidateMetaOverwritePreserve(
		viper.GetBool(keys.MOverwrite),
		viper.GetBool(keys.MPreserve),
	)

	// Verify user metafile purge settings
	if viper.IsSet(keys.MetaPurge) {
		validation.ValidatePurgeMetafiles(viper.GetString(keys.MetaPurge))
	}

	// Parse and verify the audio codec
	if viper.IsSet(keys.TranscodeAudioCodec) {
		if err := validation.ValidateAudioCodec(viper.GetString(keys.TranscodeAudioCodec)); err != nil {
			return err
		}
	}

	// Parse GPU settings and set commands
	if viper.IsSet(keys.UseGPU) {
		if err := validation.ValidateGPU(viper.GetString(keys.UseGPU)); err != nil {
			return err
		}
	}
	if viper.IsSet(keys.TranscodeCodec) {
		if err := validation.ValidateTranscodeCodec(viper.GetString((keys.TranscodeCodec))); err != nil {
			return err
		}
	}
	if viper.IsSet(keys.TranscodeQuality) {
		if err := validation.ValidateTranscodeQuality(viper.GetString(keys.TranscodeQuality)); err != nil {
			return err
		}
	}

	// Get meta operations and other transformations
	if err := initTransformations(); err != nil {
		return err
	}

	if viper.IsSet(keys.InputFileDatePfx) {
		if err := validation.ValidateDateReplaceFormat(viper.GetString(keys.InputFileDatePfx)); err != nil {
			return err
		}
	}
	return nil
}

// initTransformations initializes text replacement flags.
func initTransformations() error {
	// Set rename flag
	validation.ValidateRenameFlag(viper.GetString(keys.RenameStyle))

	// Filename suffix replacements
	if err := validation.ValidateFilenameSuffixReplace(viper.GetStringSlice(keys.InputFilenameReplaceSfx)); err != nil {
		return err
	}
	return nil
}
