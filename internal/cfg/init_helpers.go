package cfg

import (
	keys "metarr/internal/domain/keys"

	"github.com/spf13/viper"
)

// initFilesDirs initializes user flag settings for input files and directories
func initFilesDirs() {

	// Batch
	rootCmd.PersistentFlags().StringSlice(keys.BatchPairsInput, nil, "Pairs of video and JSON directories (e.g. '/videodir:/metadir')")
	viper.BindPFlag(keys.BatchPairsInput, rootCmd.PersistentFlags().Lookup(keys.BatchPairsInput))

	// Videos
	rootCmd.PersistentFlags().StringSliceP(keys.VideoDirs, "v", nil, "A directory containing videos")
	viper.BindPFlag(keys.VideoDirs, rootCmd.PersistentFlags().Lookup(keys.VideoDirs))

	rootCmd.PersistentFlags().StringSliceP(keys.VideoFiles, "V", nil, "A video file")
	viper.BindPFlag(keys.VideoFiles, rootCmd.PersistentFlags().Lookup(keys.VideoFiles))

	// JSON
	rootCmd.PersistentFlags().StringSliceP(keys.JsonDirs, "j", nil, "A directory containing videos")
	viper.BindPFlag(keys.JsonDirs, rootCmd.PersistentFlags().Lookup(keys.JsonDirs))

	rootCmd.PersistentFlags().StringSliceP(keys.JsonFiles, "J", nil, "A directory containing videos")
	viper.BindPFlag(keys.JsonFiles, rootCmd.PersistentFlags().Lookup(keys.JsonFiles))

	// Cookies
	rootCmd.PersistentFlags().String(keys.CookiePath, "", "Specify cookie location")
	viper.BindPFlag(keys.CookiePath, rootCmd.PersistentFlags().Lookup(keys.CookiePath))
}

// initResourceRelated initializes user flag settings for parameters related to system hardware
func initResourceRelated() {

	// Concurrency limit
	rootCmd.PersistentFlags().IntP(keys.Concurrency, "l", 5, "Max concurrency limit")
	viper.BindPFlag(keys.Concurrency, rootCmd.PersistentFlags().Lookup(keys.Concurrency))

	// CPU usage
	rootCmd.PersistentFlags().Float64P(keys.MaxCPU, "c", 100.0, "Max CPU usage")
	viper.BindPFlag(keys.MaxCPU, rootCmd.PersistentFlags().Lookup(keys.MaxCPU))

	// Min memory
	rootCmd.PersistentFlags().Uint64P(keys.MinMem, "m", 0, "Minimum RAM to start process")
	viper.BindPFlag(keys.MinMem, rootCmd.PersistentFlags().Lookup(keys.MinMem))

	// Hardware accelerated transcoding
	rootCmd.PersistentFlags().StringP(keys.GPU, "g", "none", "GPU acceleration type (nvidia, amd, intel, none)")
	viper.BindPFlag(keys.GPU, rootCmd.PersistentFlags().Lookup(keys.GPU))
}

// initAllFileTransformers initializes user flag settings for transformations applying to all files
func initAllFileTransformers() {

	// Prefix file with metafield
	rootCmd.PersistentFlags().StringSlice(keys.MFilenamePfx, nil, "Adds a specified metatag's value onto the start of the filename")
	viper.BindPFlag(keys.MFilenamePfx, rootCmd.PersistentFlags().Lookup(keys.MFilenamePfx))

	// Prefix files with date tag
	rootCmd.PersistentFlags().String(keys.InputFileDatePfx, "", "Looks for dates in metadata to prefix the video with. (date:format [e.g. Ymd for yyyy-mm-dd])")
	viper.BindPFlag(keys.InputFileDatePfx, rootCmd.PersistentFlags().Lookup(keys.InputFileDatePfx))

	// Rename convention
	rootCmd.PersistentFlags().StringP(keys.RenameStyle, "r", "fixes-only", "Rename flag (spaces, underscores, fixes-only, or skip)")
	viper.BindPFlag(keys.RenameStyle, rootCmd.PersistentFlags().Lookup(keys.RenameStyle))

	// Replace filename suffix
	rootCmd.PersistentFlags().StringSliceVar(&filenameReplaceSuffixInput, keys.InputFilenameReplaceSfx, nil, "Replaces a specified suffix on filenames. (suffix:replacement)")
	viper.BindPFlag(keys.InputFilenameReplaceSfx, rootCmd.PersistentFlags().Lookup(keys.InputFilenameReplaceSfx))

	// Backup files by renaming original files
	rootCmd.PersistentFlags().BoolP(keys.NoFileOverwrite, "n", false, "Renames the original files to avoid overwriting")
	viper.BindPFlag(keys.NoFileOverwrite, rootCmd.PersistentFlags().Lookup(keys.NoFileOverwrite))

	// Output directory (can be external)
	rootCmd.PersistentFlags().StringP(keys.MoveOnComplete, "o", "", "Move files to given directory on program completion")
	viper.BindPFlag(keys.MoveOnComplete, rootCmd.PersistentFlags().Lookup(keys.MoveOnComplete))
}

// initMetaTransformers initializes user flag settings for manipulation of metadata
func initMetaTransformers() {

	// Metadata transformations
	rootCmd.PersistentFlags().StringSlice(keys.MetaOps, nil, "Metadata operations (field:operation:value) - e.g. title:set:New Title, description:prefix:Draft-, tags:append:newtag")
	viper.BindPFlag(keys.MetaOps, rootCmd.PersistentFlags().Lookup(keys.MetaOps))

	// Prefix or append description fields with dates
	rootCmd.PersistentFlags().Bool(keys.MDescDatePfx, false, "Adds the date to the start of the description field.")
	viper.BindPFlag(keys.MDescDatePfx, rootCmd.PersistentFlags().Lookup(keys.MDescDatePfx))

	rootCmd.PersistentFlags().Bool(keys.MDescDateSfx, false, "Adds the date to the end of the description field.")
	viper.BindPFlag(keys.MDescDateSfx, rootCmd.PersistentFlags().Lookup(keys.MDescDateSfx))

	// Overwrite or preserve metafields
	rootCmd.PersistentFlags().Bool(keys.MOverwrite, false, "When adding new metadata fields, automatically overwrite existing fields with your new values")
	viper.BindPFlag(keys.MOverwrite, rootCmd.PersistentFlags().Lookup(keys.MOverwrite))

	rootCmd.PersistentFlags().Bool(keys.MPreserve, false, "When adding new metadata fields, skip already existent fields")
	viper.BindPFlag(keys.MPreserve, rootCmd.PersistentFlags().Lookup(keys.MPreserve))

	rootCmd.PersistentFlags().String(keys.MetaPurge, "", "Delete metadata files (e.g. .json, .nfo) after the video is successfully processed")
	viper.BindPFlag(keys.MetaPurge, rootCmd.PersistentFlags().Lookup(keys.MetaPurge))
}

// initVideoTransformers initializes user flag settings for transformation of video files
func initVideoTransformers() {

	// Output extension type
	rootCmd.PersistentFlags().String(keys.OutputFiletypeInput, "", "File extension to output files as (mp4 works best for most media servers)")
	viper.BindPFlag(keys.OutputFiletypeInput, rootCmd.PersistentFlags().Lookup(keys.OutputFiletypeInput))
}

// initFiltering initializes user flag settings for filtering files to work with
func initFiltering() {

	// Video file extensions to convert
	rootCmd.PersistentFlags().StringSliceP(keys.InputVideoExts, "e", []string{"all"}, "File extensions to convert (all, mkv, mp4, webm)")
	viper.BindPFlag(keys.InputVideoExts, rootCmd.PersistentFlags().Lookup(keys.InputVideoExts))

	// Meta file extensions to convert
	rootCmd.PersistentFlags().StringSlice(keys.InputMetaExts, []string{"all"}, "File extensions to convert (all, json, nfo)")
	viper.BindPFlag(keys.InputMetaExts, rootCmd.PersistentFlags().Lookup(keys.InputMetaExts))

	// Only convert files with prefix
	rootCmd.PersistentFlags().StringSliceP(keys.FilePrefixes, "p", []string{""}, "Filters files by prefixes")
	viper.BindPFlag(keys.FilePrefixes, rootCmd.PersistentFlags().Lookup(keys.FilePrefixes))
}

// initProgramFunctions initializes user flag settings for miscellaneous program features such as debug level
func initProgramFunctions() {

	// Debugging level
	rootCmd.PersistentFlags().IntP(keys.DebugLevel, "d", 0, "Level of debugging (0 - 3)")
	viper.BindPFlag(keys.DebugLevel, rootCmd.PersistentFlags().Lookup(keys.DebugLevel))

	// Skip videos, only alter metafiles
	rootCmd.PersistentFlags().Bool(keys.SkipVideos, false, "Skips compiling/transcoding the videos and just edits the file names/JSON file fields")
	viper.BindPFlag(keys.SkipVideos, rootCmd.PersistentFlags().Lookup(keys.SkipVideos))

	// Preset configurations for sites
	rootCmd.PersistentFlags().String(keys.InputPreset, "", "Use a preset configuration (e.g. censoredtv)")
	viper.BindPFlag(keys.InputPreset, rootCmd.PersistentFlags().Lookup(keys.InputPreset))

	// Output benchmarking files
	rootCmd.PersistentFlags().Bool(keys.Benchmarking, false, "Benchmarks the program")
	viper.BindPFlag(keys.Benchmarking, rootCmd.PersistentFlags().Lookup(keys.Benchmarking))
}
