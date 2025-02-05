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

	CookiePath string = "cookie-dir"

	InputMetaExts  string = "input-meta-exts"
	InputVideoExts string = "input-video-exts"
	FilePrefixes   string = "prefix"

	Concurrency     string = "concurrency"
	MaxCPU          string = "max-cpu"
	MinFreeMemInput string = "min-free-mem"

	InputFilenameReplaceSfx string = "filename-replace-suffix"
	InputFileDatePfx        string = "filename-date-tag"
	RenameStyle             string = "rename-style"
	MFilenamePfx            string = "metadata-filename-prefix"

	MetaOps      string = "meta-ops"
	MDescDatePfx string = "desc-date-prefix"
	MDescDateSfx string = "desc-date-suffix"

	DebugLevel      string = "debug"
	SkipVideos      string = "skip-videos"
	NoFileOverwrite string = "no-file-overwrite"

	Benchmarking        string = "benchmark"
	OutputFiletypeInput string = "ext"
	MoveOnComplete      string = "output-directory"
	InputPreset         string = "preset"

	UseGPU             string = "hwaccel"
	TranscodeDeviceDir string = "transcode-gpu-directory"
	TranscodeCodec     string = "transcode-codec"
	TranscodeQuality   string = "transcode-quality"

	TranscodeAudioCodec string = "transcode-audio-codec"
)

// Primary program
const (
	WaitGroup  string = "WaitGroup"
	SingleFile string = "SingleFile"
	MinFreeMem string = "minFreeMem"
)

// Files and directories
const (
	BatchPairs     string = "batchPairs"
	MetaPurgeEnum  string = "metaPurgeEnum"
	OutputFiletype string = "outputFiletype"
	BenchFiles     string = "benchFiles"
)

// Filter for files
const (
	InputVExtsEnum string = "inputVideoExtsEnum"
	InputMExtsEnum string = "inputMetaExtsEnum"
)

// Filename edits
const (
	Rename             string = "Rename"
	FileDateFmt        string = "filenameDateTag"
	FilenameReplaceSfx string = "filenameReplaceSfx"
)

// Meta edits
const (
	MOverwrite      string = "meta-overwrite"
	MPreserve       string = "meta-preserve"
	MCopyToField    string = "copyToField"
	MPasteFromField string = "pasteFromField"
	MAppend         string = "metaAppend"
	MNewField       string = "metaNewField"
	MPrefix         string = "metaPrefix"
	MReplaceText    string = "metaReplaceText"
	MTrimPrefix     string = "metaTrimPrefix"
	MTrimSuffix     string = "metaTrimSuffix"
	MDateTagMap     string = "metaDateTagMap"
	MDelDateTagMap  string = "metaDelDateTagMap"
)

// MultiEntryFields lists the fields containing multiple entries.
var MultiEntryFields = []string{
	InputVideoExts,
	InputMetaExts,
	FilePrefixes,
	FilenameReplaceSfx,
}
