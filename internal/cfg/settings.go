package cfg

import (
	"fmt"
	"metarr/internal/domain/keys"
	"metarr/internal/utils/validation"
	"os"
	"strings"

	"github.com/spf13/viper"
)

// init sets the initial Viper settings.
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

// execute more thoroughly handles settings created in the Viper init.
func execute() error {
	// Batch pairs
	if viper.IsSet(keys.BatchPairsInput) {
		if err := validation.ValidateBatchPairs(viper.GetStringSlice(keys.BatchPairsInput)); err != nil {
			return err
		}
	}

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

	// File filter settings
	if viper.IsSet(keys.FilePrefixes) {
		validation.ValidateSetFileFilters(keys.FilePrefixes, viper.GetStringSlice(keys.FilePrefixes))
	}
	if viper.IsSet(keys.FileSuffixes) {
		validation.ValidateSetFileFilters(keys.FileSuffixes, viper.GetStringSlice(keys.FileSuffixes))
	}
	if viper.IsSet(keys.FileContains) {
		validation.ValidateSetFileFilters(keys.FileContains, viper.GetStringSlice(keys.FileContains))
	}
	if viper.IsSet(keys.FileOmits) {
		validation.ValidateSetFileFilters(keys.FileOmits, viper.GetStringSlice(keys.FileOmits))
	}

	// Filetype to output as
	if viper.IsSet(keys.OutputDirectory) {
		validation.ValidateOutputFiletype(viper.GetString(keys.OutputDirectory))
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
	return nil
}

// initTransformations initializes text replacement flags.
func initTransformations() error {
	// Set rename flag
	validation.ValidateRenameFlag(viper.GetString(keys.RenameStyle))

	// Validate filename operations
	if viper.IsSet(keys.FilenameOpsInput) {
		if err := validation.ValidateSetFilenameOps(viper.GetStringSlice(keys.FilenameOpsInput)); err != nil {
			return err
		}
	}

	// Validate meta operations
	if viper.IsSet(keys.MetaOpsInput) {
		if err := validation.ValidateSetMetaOps(viper.GetStringSlice(keys.MetaOpsInput)); err != nil {
			return err
		}
	}
	return nil
}
