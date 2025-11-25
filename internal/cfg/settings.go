package cfg

import (
	"fmt"
	"metarr/internal/domain/keys"
	"metarr/internal/validation"
	"os"
	"strings"

	"github.com/TubarrApp/gocommon/sharedtemplates"
	"github.com/TubarrApp/gocommon/sharedvalidation"
	"github.com/spf13/viper"
)

// init sets the initial Viper settings.
func init() {
	// Env vars.
	viper.AutomaticEnv()
	viper.SetEnvKeyReplacer(strings.NewReplacer("_", "-")) // Convert "video_directory" to "video-directory"

	// Config file.
	rootCmd.PersistentFlags().String(keys.ConfigPath, "", "Specify a path to your preset configuration file")
	if err := viper.BindPFlag(keys.ConfigPath, rootCmd.PersistentFlags().Lookup(keys.ConfigPath)); err != nil {
		fmt.Fprintf(os.Stderr, "config file path setting failure: %v\n", err)
		os.Exit(1)
	}

	// Files and directories.
	initOrExit(initFilesDirs(),
		"files & dirs initialization failure")

	// System resource related.
	initOrExit(initResourceRelated(),
		"config resource element initialization failure")

	// Filtering.
	initOrExit(initFiltering(),
		"config filtering initialization failure")

	// All file transformations.
	initOrExit(initAllFileTransformers(),
		"config file transformer initialization failure")

	// Filename transformations.
	initOrExit(initVideoTransformers(),
		"config video transformer initialization failure")

	// Metadata and metafile manipulation.
	initOrExit(initMetaTransformers(),
		"config meta transformer initialization failure")

	// Special functions.
	initOrExit(initProgramFunctions(),
		"config program function initialization failure")
}

// execute more thoroughly handles settings created in the Viper init.
func execute() (err error) {
	// Batch pairs.
	if viper.IsSet(keys.BatchPairsInput) {
		if err := validation.ValidateAndSetBatchPairs(viper.GetStringSlice(keys.BatchPairsInput)); err != nil {
			return err
		}
	}

	// Concurrency.
	validation.ValidateAndSetConcurrencyLimit(viper.GetInt(keys.Concurrency))

	// Resource usage limits (CPU and memory).
	validation.ValidateAndSetMinFreeMem(viper.GetString(keys.MinFreeMem))
	validation.ValidateAndSetMaxCPU(viper.GetFloat64(keys.MaxCPU))

	// File extension settings.
	validation.ValidateAndSetInputFiletypes(
		viper.GetStringSlice(keys.InputVideoExts),
		viper.GetStringSlice(keys.InputMetaExts),
	)

	// File filter settings.
	if viper.IsSet(keys.FilePrefixes) {
		validation.ValidateAndSetFileFilters(keys.FilePrefixes, viper.GetStringSlice(keys.FilePrefixes))
	}
	if viper.IsSet(keys.FileSuffixes) {
		validation.ValidateAndSetFileFilters(keys.FileSuffixes, viper.GetStringSlice(keys.FileSuffixes))
	}
	if viper.IsSet(keys.FileContains) {
		validation.ValidateAndSetFileFilters(keys.FileContains, viper.GetStringSlice(keys.FileContains))
	}
	if viper.IsSet(keys.FileOmits) {
		validation.ValidateAndSetFileFilters(keys.FileOmits, viper.GetStringSlice(keys.FileOmits))
	}

	// Output directory.
	if viper.IsSet(keys.OutputDirectory) {
		if _, _, err := sharedvalidation.ValidateDirectory(viper.GetString(keys.OutputDirectory), true, sharedtemplates.MetarrTemplateTags); err != nil {
			return err
		}
	}

	// Filetype to output as.
	if viper.IsSet(keys.OutputFiletype) {
		validation.ValidateAndSetOutputFiletype(viper.GetString(keys.OutputFiletype))
	}

	// Meta overwrite and preserve flags.
	validation.ValidateAndSetMetaOverwritePreserve(
		viper.GetBool(keys.MOverwrite),
		viper.GetBool(keys.MPreserve),
	)

	// Verify user metafile purge settings.
	if viper.IsSet(keys.MetaPurge) {
		validation.ValidateAndSetPurgeMetafiles(viper.GetString(keys.MetaPurge))
	}

	// Parse and verify the audio codec.
	if viper.IsSet(keys.TranscodeAudioCodecInput) {
		if err := validation.ValidateAndSetAudioCodec(viper.GetStringSlice(keys.TranscodeAudioCodecInput)); err != nil {
			return err
		}
	}

	// Parse GPU settings and set commands.
	if viper.IsSet(keys.TranscodeGPU) {
		// Retrieve Viper strings.
		accel := viper.GetString(keys.TranscodeGPU)
		nodePath := ""
		if viper.IsSet(keys.TranscodeDeviceDir) {
			nodePath = viper.GetString(keys.TranscodeDeviceDir)
		}

		// Validate GPU and node path.
		if a, err := validation.ValidateGPUAndNode(accel, nodePath); err != nil {
			return err
		} else if a != accel {
			viper.Set(keys.TranscodeGPU, a)
		}
	}
	if viper.IsSet(keys.TranscodeVideoCodecInput) {
		if err := validation.ValidateAndSetVideoCodec(viper.GetStringSlice((keys.TranscodeVideoCodecInput))); err != nil {
			return err
		}
	}
	if viper.IsSet(keys.TranscodeQuality) {
		if err := validation.ValidateAndSetTranscodeQuality(viper.GetString(keys.TranscodeQuality)); err != nil {
			return err
		}
	}

	// Get meta operations and other transformations.
	if err := initTransformations(); err != nil {
		return err
	}
	return nil
}

// initTransformations initializes text replacement flags.
func initTransformations() error {
	// Set rename flag.
	validation.ValidateAndSetRenameFlag(viper.GetString(keys.RenameStyle))

	// Validate filename operations.
	if viper.IsSet(keys.FilenameOpsInput) {
		if err := validation.ValidateAndSetFilenameOps(viper.GetStringSlice(keys.FilenameOpsInput)); err != nil {
			return err
		}
	}

	// Validate meta operations.
	if viper.IsSet(keys.MetaOpsInput) {
		if err := validation.ValidateAndSetMetaOps(viper.GetStringSlice(keys.MetaOpsInput)); err != nil {
			return err
		}
	}
	return nil
}
