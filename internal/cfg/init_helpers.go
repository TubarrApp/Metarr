package cfg

import (
	"fmt"
	keys "metarr/internal/domain/keys"
	"metarr/internal/utils/logging"
	"os"

	"github.com/spf13/viper"
)

// initFilesDirs initializes user flag settings for input files and directories.
func initFilesDirs() error {

	// Batch
	rootCmd.PersistentFlags().StringSlice(keys.BatchPairsInput, nil, "Pairs of video and JSON directories (e.g. '/videodir:/metadir')")
	if err := viper.BindPFlag(keys.BatchPairsInput, rootCmd.PersistentFlags().Lookup(keys.BatchPairsInput)); err != nil {
		return err
	}

	// Videos
	rootCmd.PersistentFlags().StringSliceP(keys.VideoDirs, "v", nil, "A directory containing videos")
	if err := viper.BindPFlag(keys.VideoDirs, rootCmd.PersistentFlags().Lookup(keys.VideoDirs)); err != nil {
		return err
	}

	rootCmd.PersistentFlags().StringSliceP(keys.VideoFiles, "V", nil, "A video file")
	if err := viper.BindPFlag(keys.VideoFiles, rootCmd.PersistentFlags().Lookup(keys.VideoFiles)); err != nil {
		return err
	}

	// JSON
	rootCmd.PersistentFlags().StringSliceP(keys.JSONDirs, "j", nil, "A directory containing videos")
	if err := viper.BindPFlag(keys.JSONDirs, rootCmd.PersistentFlags().Lookup(keys.JSONDirs)); err != nil {
		return err
	}

	rootCmd.PersistentFlags().StringSliceP(keys.JSONFiles, "J", nil, "A directory containing videos")
	if err := viper.BindPFlag(keys.JSONFiles, rootCmd.PersistentFlags().Lookup(keys.JSONFiles)); err != nil {
		return err
	}

	// Cookies
	rootCmd.PersistentFlags().String(keys.CookiePath, "", "Specify cookie location")
	if err := viper.BindPFlag(keys.CookiePath, rootCmd.PersistentFlags().Lookup(keys.CookiePath)); err != nil {
		return err
	}
	return nil
}

// initResourceRelated initializes user flag settings for parameters related to system hardware.
func initResourceRelated() error {

	// Concurrency limit
	rootCmd.PersistentFlags().IntP(keys.Concurrency, "l", 5, "Max concurrency limit")
	if err := viper.BindPFlag(keys.Concurrency, rootCmd.PersistentFlags().Lookup(keys.Concurrency)); err != nil {
		return err
	}

	// CPU usage
	rootCmd.PersistentFlags().Float64P(keys.MaxCPU, "c", 101.0, "Max CPU usage")
	if err := viper.BindPFlag(keys.MaxCPU, rootCmd.PersistentFlags().Lookup(keys.MaxCPU)); err != nil {
		return err
	}

	// Min memory
	rootCmd.PersistentFlags().StringP(keys.MinFreeMemInput, "m", "0", "Minimum free RAM to start process")
	if err := viper.BindPFlag(keys.MinFreeMemInput, rootCmd.PersistentFlags().Lookup(keys.MinFreeMemInput)); err != nil {
		return err
	}
	return nil
}

// initAllFileTransformers initializes user flag settings for transformations applying to all files.
func initAllFileTransformers() error {

	// Prefix file with metafield
	rootCmd.PersistentFlags().StringSlice(keys.MFilenamePfx, nil, "Adds a specified metatag's value onto the start of the filename")
	if err := viper.BindPFlag(keys.MFilenamePfx, rootCmd.PersistentFlags().Lookup(keys.MFilenamePfx)); err != nil {
		return err
	}

	// Prefix files with date tag
	rootCmd.PersistentFlags().String(keys.InputFileDatePfx, "", "Looks for dates in metadata to prefix the video with. (date:format [e.g. Ymd for yyyy-mm-dd])")
	if err := viper.BindPFlag(keys.InputFileDatePfx, rootCmd.PersistentFlags().Lookup(keys.InputFileDatePfx)); err != nil {
		return err
	}

	// Strip existing date tag
	rootCmd.PersistentFlags().Bool(keys.DeleteDateTagPfx, false, "Delete existing date tags from files")
	if err := viper.BindPFlag(keys.DeleteDateTagPfx, rootCmd.PersistentFlags().Lookup(keys.DeleteDateTagPfx)); err != nil {
		return err
	}

	// Rename convention
	rootCmd.PersistentFlags().StringP(keys.RenameStyle, "r", "skip", "Rename flag (spaces, underscores, fixes-only, or skip)")
	if err := viper.BindPFlag(keys.RenameStyle, rootCmd.PersistentFlags().Lookup(keys.RenameStyle)); err != nil {
		return err
	}

	// Replace filename suffix
	rootCmd.PersistentFlags().StringSliceVar(&filenameReplaceSuffixInput, keys.InputFilenameReplaceSfx, nil, "Replaces a specified suffix on filenames. (suffix:replacement)")
	if err := viper.BindPFlag(keys.InputFilenameReplaceSfx, rootCmd.PersistentFlags().Lookup(keys.InputFilenameReplaceSfx)); err != nil {
		return err
	}

	// Backup files by renaming original files
	rootCmd.PersistentFlags().BoolP(keys.NoFileOverwrite, "n", false, "Renames the original files to avoid overwriting")
	if err := viper.BindPFlag(keys.NoFileOverwrite, rootCmd.PersistentFlags().Lookup(keys.NoFileOverwrite)); err != nil {
		return err
	}

	// Output directory (can be external)
	rootCmd.PersistentFlags().StringP(keys.MoveOnComplete, "o", "", "Move files to given directory on program completion")
	if err := viper.BindPFlag(keys.MoveOnComplete, rootCmd.PersistentFlags().Lookup(keys.MoveOnComplete)); err != nil {
		return err
	}
	return nil
}

// initMetaTransformers initializes user flag settings for manipulation of metadata.
func initMetaTransformers() error {

	// Metadata transformations
	rootCmd.PersistentFlags().StringSlice(keys.MetaOps, nil, "Metadata operations (field:operation:value) - e.g. title:set:New Title, description:prefix:Draft-, tags:append:newtag")
	if err := viper.BindPFlag(keys.MetaOps, rootCmd.PersistentFlags().Lookup(keys.MetaOps)); err != nil {
		return err
	}

	// Prefix or append description fields with dates
	rootCmd.PersistentFlags().Bool(keys.MDescDatePfx, false, "Adds the date to the start of the description field.")
	if err := viper.BindPFlag(keys.MDescDatePfx, rootCmd.PersistentFlags().Lookup(keys.MDescDatePfx)); err != nil {
		return err
	}

	rootCmd.PersistentFlags().Bool(keys.MDescDateSfx, false, "Adds the date to the end of the description field.")
	if err := viper.BindPFlag(keys.MDescDateSfx, rootCmd.PersistentFlags().Lookup(keys.MDescDateSfx)); err != nil {
		return err
	}

	// Overwrite or preserve metafields
	rootCmd.PersistentFlags().Bool(keys.MOverwrite, false, "When adding new metadata fields, automatically overwrite existing fields with your new values")
	if err := viper.BindPFlag(keys.MOverwrite, rootCmd.PersistentFlags().Lookup(keys.MOverwrite)); err != nil {
		return err
	}

	rootCmd.PersistentFlags().Bool(keys.MPreserve, false, "When adding new metadata fields, skip already existent fields")
	if err := viper.BindPFlag(keys.MPreserve, rootCmd.PersistentFlags().Lookup(keys.MPreserve)); err != nil {
		return err
	}

	rootCmd.PersistentFlags().String(keys.MetaPurge, "", "Delete metadata files (e.g. .json, .nfo) after the video is successfully processed")
	if err := viper.BindPFlag(keys.MetaPurge, rootCmd.PersistentFlags().Lookup(keys.MetaPurge)); err != nil {
		return err
	}
	return nil
}

// initVideoTransformers initializes user flag settings for transformation of video files.
func initVideoTransformers() error {

	// Output extension type
	rootCmd.PersistentFlags().String(keys.OutputFiletypeInput, "", "File extension to output files as (mp4 works best for most media servers)")
	if err := viper.BindPFlag(keys.OutputFiletypeInput, rootCmd.PersistentFlags().Lookup(keys.OutputFiletypeInput)); err != nil {
		return err
	}

	// HW acceleration
	rootCmd.PersistentFlags().String(keys.UseGPU, "", "Use hardware for accelerated encoding/decoding")
	if err := viper.BindPFlag(keys.UseGPU, rootCmd.PersistentFlags().Lookup(keys.UseGPU)); err != nil {
		return err
	}

	rootCmd.PersistentFlags().String(keys.TranscodeDeviceDir, "", "Directory for the transcoding GPU (e.g. '/dev/dri/renderD128')")
	if err := viper.BindPFlag(keys.TranscodeDeviceDir, rootCmd.PersistentFlags().Lookup(keys.TranscodeDeviceDir)); err != nil {
		return err
	}

	rootCmd.PersistentFlags().String(keys.TranscodeCodec, "", "Codec to use for encoding/decoding")
	if err := viper.BindPFlag(keys.TranscodeCodec, rootCmd.PersistentFlags().Lookup(keys.TranscodeCodec)); err != nil {
		return err
	}

	rootCmd.PersistentFlags().String(keys.TranscodeQuality, "", "Quality profile for encoding/decoding (p1 [worst] to p7 [best])")
	if err := viper.BindPFlag(keys.TranscodeQuality, rootCmd.PersistentFlags().Lookup(keys.TranscodeQuality)); err != nil {
		return err
	}

	rootCmd.PersistentFlags().String(keys.TranscodeVideoFilter, "", "Transcoder video filter settings")
	if err := viper.BindPFlag(keys.TranscodeVideoFilter, rootCmd.PersistentFlags().Lookup(keys.TranscodeVideoFilter)); err != nil {
		return err
	}

	rootCmd.PersistentFlags().String(keys.TranscodeAudioCodec, "", "Audio codec for encoding/decoding (e.g. 'aac', 'copy')")
	if err := viper.BindPFlag(keys.TranscodeAudioCodec, rootCmd.PersistentFlags().Lookup(keys.TranscodeAudioCodec)); err != nil {
		return err
	}

	rootCmd.PersistentFlags().String(keys.ExtraFFmpegArgs, "", "Extra FFmpeg arguments to append to FFmpeg commands")
	if err := viper.BindPFlag(keys.ExtraFFmpegArgs, rootCmd.PersistentFlags().Lookup(keys.ExtraFFmpegArgs)); err != nil {
		return err
	}

	return nil
}

// initFiltering initializes user flag settings for filtering files to work with.
func initFiltering() error {

	// Video file extensions to convert
	rootCmd.PersistentFlags().StringSliceP(keys.InputVideoExts, "e", []string{"all"}, "File extensions to convert (all, mkv, mp4, webm)")
	if err := viper.BindPFlag(keys.InputVideoExts, rootCmd.PersistentFlags().Lookup(keys.InputVideoExts)); err != nil {
		return err
	}

	// Meta file extensions to convert
	rootCmd.PersistentFlags().StringSlice(keys.InputMetaExts, []string{"all"}, "File extensions to convert (all, json, nfo)")
	if err := viper.BindPFlag(keys.InputMetaExts, rootCmd.PersistentFlags().Lookup(keys.InputMetaExts)); err != nil {
		return err
	}

	// Only convert files with prefix
	rootCmd.PersistentFlags().StringSliceP(keys.FilePrefixes, "p", []string{""}, "Filters files by prefixes")
	if err := viper.BindPFlag(keys.FilePrefixes, rootCmd.PersistentFlags().Lookup(keys.FilePrefixes)); err != nil {
		return err
	}
	return nil
}

// initProgramFunctions initializes user flag settings for miscellaneous program features such as debug level.
func initProgramFunctions() error {

	// Debugging level
	rootCmd.PersistentFlags().Int(keys.DebugLevel, 0, "Level of debugging (0 - 5)")
	if err := viper.BindPFlag(keys.DebugLevel, rootCmd.PersistentFlags().Lookup(keys.DebugLevel)); err != nil {
		return err
	}

	// Skip videos, only alter metafiles
	rootCmd.PersistentFlags().Bool(keys.SkipVideos, false, "Skips compiling/transcoding the videos and just edits the file names/JSON file fields")
	if err := viper.BindPFlag(keys.SkipVideos, rootCmd.PersistentFlags().Lookup(keys.SkipVideos)); err != nil {
		return err
	}

	// Preset configurations for sites
	rootCmd.PersistentFlags().String(keys.InputPreset, "", "Use a preset configuration (e.g. censoredtv)")
	if err := viper.BindPFlag(keys.InputPreset, rootCmd.PersistentFlags().Lookup(keys.InputPreset)); err != nil {
		return err
	}

	// Output benchmarking files
	rootCmd.PersistentFlags().Bool(keys.Benchmarking, false, "Benchmarks the program")
	if err := viper.BindPFlag(keys.Benchmarking, rootCmd.PersistentFlags().Lookup(keys.Benchmarking)); err != nil {
		return err
	}
	return nil
}

// initOrExit attempts to run the function and exits the program on failure.
func initOrExit(err error, failMsg string) {
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: %v\n", failMsg, err)
		os.Exit(1)
	}
}

// loadConfigFile loads in the preset configuration file.
func loadConfigFile(file string) error {
	logging.I("Using configuration file %q", file)
	viper.SetConfigFile(file)
	if err := viper.ReadInConfig(); err != nil {
		return err
	}

	return nil
}
