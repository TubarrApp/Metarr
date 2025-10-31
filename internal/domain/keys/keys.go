// Package keys holds keys relating to terminal commands, and internal Viper/Cobra keys.
package keys

// Terminal keys
const (
	BatchPairsInput string = "batch-pairs"
	VideoDirs       string = "video-directory"
	VideoFiles      string = "video-file"
	JSONDirs        string = "json-directory"
	JSONFiles       string = "json-file"
	MetaPurge       string = "purge-metafile"

	ConfigPath string = "config-file"
	CookiePath string = "cookie-dir"

	InputMetaExts  string = "input-meta-exts"
	InputVideoExts string = "input-video-exts"
	FilePrefixes   string = "filter-prefix"
	FileSuffixes   string = "filter-suffix"
	FileContains   string = "filter-contains"
	FileOmits      string = "filter-omits"

	Concurrency     string = "concurrency"
	MaxCPU          string = "max-cpu"
	MinFreeMemInput string = "min-free-mem"

	FilenameOpsInput string = "filename-ops"
	RenameStyle      string = "rename-style"

	MetaOpsInput string = "meta-ops"
	MDescDatePfx string = "desc-date-prefix"
	MDescDateSfx string = "desc-date-suffix"

	DebugLevel      string = "debug"
	SkipVideos      string = "skip-videos"
	NoFileOverwrite string = "no-file-overwrite"

	Benchmarking    string = "benchmark"
	OutputFiletype  string = "output-ext"
	OutputDirectory string = "output-directory"
	InputPreset     string = "preset"

	UseGPU               string = "hwaccel"
	TranscodeDeviceDir   string = "transcode-gpu-directory"
	TranscodeAudioCodec  string = "transcode-audio-codec"
	TranscodeVideoCodec  string = "transcode-video-codec"
	TranscodeQuality     string = "transcode-quality"
	TranscodeVideoFilter string = "transcode-video-filter"

	ExtraFFmpegArgs string = "extra-ffmpeg-args"
)

// Primary program
const (
	MinFreeMem string = "minFreeMem"
)

// Files and directories
const (
	MetaPurgeEnum string = "metaPurgeEnum"
)

// Filter for files
const (
	InputVExts string = "inputVideoExtsEnum"
	InputMExts string = "inputMetaExtsEnum"
)

// Filename edits
const (
	Rename string = "Rename"
)

// Meta edits
const (
	MOverwrite      string = "meta-overwrite"
	MPreserve       string = "meta-preserve"
	MCopyToField    string = "copyToField"
	MPasteFromField string = "pasteFromField"
	MAppend         string = "metaAppend"
	MNewField       string = "MetaSetField"
	MPrefix         string = "metaPrefix"
	MReplaceText    string = "metaReplaceText"
	MTrimPrefix     string = "metaTrimPrefix"
	MTrimSuffix     string = "metaTrimSuffix"
	MDateTagMap     string = "metaDateTagMap"
	MDelDateTagMap  string = "metaDelDateTagMap"
)

// Internal filename operation keys. Not exposed to end user.
const (
	BatchPairs        string = "INTERNAL-batch-files"
	FilenameOpsModels string = "INTERNAL-filename-ops"
	MetaOpsModels     string = "INTERNAL-meta-ops"
)
