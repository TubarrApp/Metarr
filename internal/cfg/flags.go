package cfg

import (
	"fmt"
	"metarr/internal/domain/keys"
	"metarr/internal/domain/logger"
	"os"

	"github.com/spf13/viper"
)

// initFilesDirs initializes user flag settings for input files and directories.
func initFilesDirs() error {
	// Batch.
	rootCmd.PersistentFlags().StringSliceP(keys.BatchPairsInput, "b", nil, "Pairs of video and JSON files/directories (e.g. '/videodir:/metadir')")
	if err := viper.BindPFlag(keys.BatchPairsInput, rootCmd.PersistentFlags().Lookup(keys.BatchPairsInput)); err != nil {
		return err
	}

	// Videos.
	rootCmd.PersistentFlags().StringSliceP(keys.VideoDirs, "v", nil, "A directory containing videos")
	if err := viper.BindPFlag(keys.VideoDirs, rootCmd.PersistentFlags().Lookup(keys.VideoDirs)); err != nil {
		return err
	}

	rootCmd.PersistentFlags().StringSliceP(keys.VideoFiles, "V", nil, "A video file")
	if err := viper.BindPFlag(keys.VideoFiles, rootCmd.PersistentFlags().Lookup(keys.VideoFiles)); err != nil {
		return err
	}

	// Metafiles.
	rootCmd.PersistentFlags().StringSliceP(keys.MetaDirs, "m", nil, "A directory containing metadata files")
	if err := viper.BindPFlag(keys.MetaDirs, rootCmd.PersistentFlags().Lookup(keys.MetaDirs)); err != nil {
		return err
	}

	rootCmd.PersistentFlags().StringSliceP(keys.MetaFiles, "M", nil, "A metadata file")
	if err := viper.BindPFlag(keys.MetaFiles, rootCmd.PersistentFlags().Lookup(keys.MetaFiles)); err != nil {
		return err
	}

	// Cookies.
	rootCmd.PersistentFlags().String(keys.CookiePath, "", "Specify cookie location")
	if err := viper.BindPFlag(keys.CookiePath, rootCmd.PersistentFlags().Lookup(keys.CookiePath)); err != nil {
		return err
	}
	return nil
}

// initResourceRelated initializes user flag settings for parameters related to system hardware.
func initResourceRelated() error {
	// Concurrency limit.
	rootCmd.PersistentFlags().IntP(keys.Concurrency, "l", 5, "Max concurrency limit")
	if err := viper.BindPFlag(keys.Concurrency, rootCmd.PersistentFlags().Lookup(keys.Concurrency)); err != nil {
		return err
	}

	// CPU usage.
	rootCmd.PersistentFlags().Float64P(keys.MaxCPU, "c", 101.0, "Max CPU usage %")
	if err := viper.BindPFlag(keys.MaxCPU, rootCmd.PersistentFlags().Lookup(keys.MaxCPU)); err != nil {
		return err
	}

	// Min memory.
	rootCmd.PersistentFlags().String(keys.MinFreeMemInput, "0", "Minimum free RAM to start process")
	if err := viper.BindPFlag(keys.MinFreeMemInput, rootCmd.PersistentFlags().Lookup(keys.MinFreeMemInput)); err != nil {
		return err
	}
	return nil
}

// initAllFileTransformers initializes user flag settings for transformations applying to all files.
func initAllFileTransformers() error {
	// Prefix files with date tag.
	rootCmd.PersistentFlags().StringSlice(keys.FilenameOpsInput, nil, "Filename operations for renaming files (e.g. 'prefix:[CATEGORY] ', 'date-tag:prefix:ymd')")
	if err := viper.BindPFlag(keys.FilenameOpsInput, rootCmd.PersistentFlags().Lookup(keys.FilenameOpsInput)); err != nil {
		return err
	}

	// Rename convention.
	rootCmd.PersistentFlags().StringP(keys.RenameStyle, "r", "skip", "Rename flag (spaces, underscores, fixes-only, or skip)")
	if err := viper.BindPFlag(keys.RenameStyle, rootCmd.PersistentFlags().Lookup(keys.RenameStyle)); err != nil {
		return err
	}

	// Backup files by renaming original files.
	rootCmd.PersistentFlags().BoolP(keys.NoFileOverwrite, "n", false, "Renames the original files to avoid overwriting")
	if err := viper.BindPFlag(keys.NoFileOverwrite, rootCmd.PersistentFlags().Lookup(keys.NoFileOverwrite)); err != nil {
		return err
	}

	// Output directory (can be external).
	rootCmd.PersistentFlags().StringP(keys.OutputDirectory, "o", "", "Move files to given directory on program completion")
	if err := viper.BindPFlag(keys.OutputDirectory, rootCmd.PersistentFlags().Lookup(keys.OutputDirectory)); err != nil {
		return err
	}
	return nil
}

// initMetaTransformers initializes user flag settings for manipulation of metadata.
func initMetaTransformers() error {
	// Metadata transformations
	rootCmd.PersistentFlags().StringSlice(keys.MetaOpsInput, nil, "Metadata operations (field:operation:value) - e.g. title:set:New Title, description:prefix:Draft-, tags:append:newtag")
	if err := viper.BindPFlag(keys.MetaOpsInput, rootCmd.PersistentFlags().Lookup(keys.MetaOpsInput)); err != nil {
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
	// Output extension type.
	rootCmd.PersistentFlags().StringP(keys.OutputFiletype, "e", "", "File extension to output files as (mp4 works best for most media servers)")
	if err := viper.BindPFlag(keys.OutputFiletype, rootCmd.PersistentFlags().Lookup(keys.OutputFiletype)); err != nil {
		return err
	}

	// GPU acceleration.
	rootCmd.PersistentFlags().String(keys.TranscodeGPU, "", "Use hardware for accelerated encoding/decoding")
	if err := viper.BindPFlag(keys.TranscodeGPU, rootCmd.PersistentFlags().Lookup(keys.TranscodeGPU)); err != nil {
		return err
	}

	// Codecs and quality.
	rootCmd.PersistentFlags().StringSlice(keys.TranscodeVideoCodecInput, nil, "Codec to use for encoding/decoding")
	if err := viper.BindPFlag(keys.TranscodeVideoCodecInput, rootCmd.PersistentFlags().Lookup(keys.TranscodeVideoCodecInput)); err != nil {
		return err
	}

	rootCmd.PersistentFlags().StringSlice(keys.TranscodeAudioCodecInput, nil, "Audio codec for encoding/decoding (e.g. 'aac', 'copy')")
	if err := viper.BindPFlag(keys.TranscodeAudioCodecInput, rootCmd.PersistentFlags().Lookup(keys.TranscodeAudioCodecInput)); err != nil {
		return err
	}

	rootCmd.PersistentFlags().String(keys.TranscodeQuality, "", "CRF value for ffmpeg")
	if err := viper.BindPFlag(keys.TranscodeQuality, rootCmd.PersistentFlags().Lookup(keys.TranscodeQuality)); err != nil {
		return err
	}

	rootCmd.PersistentFlags().String(keys.TranscodePreset, "", "Standard ffmpeg presets, for example: veryslow, medium, fast")
	if err := viper.BindPFlag(keys.TranscodePreset, rootCmd.PersistentFlags().Lookup(keys.TranscodePreset)); err != nil {
		return err
	}

	rootCmd.PersistentFlags().String(keys.TranscodeVideoFilter, "", "Transcoder video filter settings")
	if err := viper.BindPFlag(keys.TranscodeVideoFilter, rootCmd.PersistentFlags().Lookup(keys.TranscodeVideoFilter)); err != nil {
		return err
	}

	// Manual additional FFmpeg arguments.
	rootCmd.PersistentFlags().String(keys.ExtraFFmpegArgs, "", "Extra FFmpeg arguments to append to FFmpeg commands")
	if err := viper.BindPFlag(keys.ExtraFFmpegArgs, rootCmd.PersistentFlags().Lookup(keys.ExtraFFmpegArgs)); err != nil {
		return err
	}

	// Thumbnail handling.
	rootCmd.PersistentFlags().Bool(keys.ForceWriteThumbnails, false, "Force FFmpeg if a thumbnail exists, even when all metadata matches")
	if err := viper.BindPFlag(keys.ForceWriteThumbnails, rootCmd.PersistentFlags().Lookup(keys.ForceWriteThumbnails)); err != nil {
		return err
	}

	rootCmd.PersistentFlags().Bool(keys.StripThumbnails, false, "Strip thumbnails from a video")
	if err := viper.BindPFlag(keys.StripThumbnails, rootCmd.PersistentFlags().Lookup(keys.StripThumbnails)); err != nil {
		return err
	}

	return nil
}

// initFiltering initializes user flag settings for filtering files to work with.
func initFiltering() error {
	// Video file extensions to convert.
	rootCmd.PersistentFlags().StringSlice(keys.InputVideoExts, []string{"all"}, "File extensions to convert (all, mkv, mp4, webm)")
	if err := viper.BindPFlag(keys.InputVideoExts, rootCmd.PersistentFlags().Lookup(keys.InputVideoExts)); err != nil {
		return err
	}

	// Meta file extensions to convert.
	rootCmd.PersistentFlags().StringSlice(keys.InputMetaExts, []string{"all"}, "File extensions to convert (all, json, nfo)")
	if err := viper.BindPFlag(keys.InputMetaExts, rootCmd.PersistentFlags().Lookup(keys.InputMetaExts)); err != nil {
		return err
	}

	// Only convert files with these filters.
	rootCmd.PersistentFlags().StringSlice(keys.FilePrefixes, []string{""}, "Filters files to edit by prefixes")
	if err := viper.BindPFlag(keys.FilePrefixes, rootCmd.PersistentFlags().Lookup(keys.FilePrefixes)); err != nil {
		return err
	}

	rootCmd.PersistentFlags().StringSlice(keys.FileSuffixes, []string{""}, "Filters files to edit by suffixes")
	if err := viper.BindPFlag(keys.FileSuffixes, rootCmd.PersistentFlags().Lookup(keys.FileSuffixes)); err != nil {
		return err
	}

	rootCmd.PersistentFlags().StringSlice(keys.FileContains, []string{""}, "Filters files to edit by strings contained")
	if err := viper.BindPFlag(keys.FileContains, rootCmd.PersistentFlags().Lookup(keys.FileContains)); err != nil {
		return err
	}

	rootCmd.PersistentFlags().StringSlice(keys.FileOmits, []string{""}, "Filters files to edit by strings omitted")
	if err := viper.BindPFlag(keys.FileOmits, rootCmd.PersistentFlags().Lookup(keys.FileOmits)); err != nil {
		return err
	}
	return nil
}

// initProgramFunctions initializes user flag settings for miscellaneous program features such as debug level.
func initProgramFunctions() error {
	// Debugging level.
	rootCmd.PersistentFlags().Int(keys.DebugLevel, 0, "Level of debugging (0 - 5)")
	if err := viper.BindPFlag(keys.DebugLevel, rootCmd.PersistentFlags().Lookup(keys.DebugLevel)); err != nil {
		return err
	}

	// Skip videos, only alter metafiles.
	rootCmd.PersistentFlags().Bool(keys.SkipVideos, false, "Skips compiling/transcoding the videos and just edits the file names/JSON file fields")
	if err := viper.BindPFlag(keys.SkipVideos, rootCmd.PersistentFlags().Lookup(keys.SkipVideos)); err != nil {
		return err
	}

	// Output benchmarking files.
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
	logger.Pl.I("Using configuration file %q", file)
	viper.SetConfigFile(file)
	if err := viper.ReadInConfig(); err != nil {
		return err
	}

	return nil
}
