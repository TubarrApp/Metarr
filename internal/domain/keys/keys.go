package domain

// Terminal keys
const (
	VideoDir  string = "video-dir"
	VideoFile string = "video-file"

	JsonDir   string = "json-dir"
	JsonFile  string = "json-file"
	MetaPurge string = "purge-metafile"

	CookiePath string = "cookie-dir"

	InputMetaExts  string = "input-meta-exts"
	InputVideoExts string = "input-video-exts"
	FilePrefixes   string = "prefix"

	Concurrency string = "concurrency"
	GPU         string = "gpu"
	MaxCPU      string = "max-cpu"
	MinMem      string = "min-mem"
	MinMemMB    string = "min-mem-mb"

	InputFilenameReplaceSfx string = "filename-replace-suffix"
	InputFileDatePfx        string = "filename-date-tag"
	RenameStyle             string = "input-rename-style"
	MFilenamePfx            string = "metadata-filename-prefix"

	MetaOps      string = "meta-ops"
	MDescDatePfx string = "desc-date-prefix"
	MDescDateSfx string = "desc-date-suffix"

	DebugLevel      string = "debug-level"
	SkipVideos      string = "skip-videos"
	NoFileOverwrite string = "no-file-overwrite"

	Benchmarking        string = "benchmark"
	OutputFiletypeInput string = "ext"
	MoveOnComplete      string = "output-directory"
	InputPreset         string = "preset"
)

// Primary program
const (
	Context    string = "Context"
	WaitGroup  string = "WaitGroup"
	SingleFile string = "SingleFile"
)

// Files and directories
const (
	OpenVideo      string = "openVideo"
	OpenJson       string = "openJson"
	VideoMap       string = "videoMap"
	MetaMap        string = "metaMap"
	MetaPurgeEnum  string = "metaPurgeEnum"
	OutputFiletype string = "outputFiletype"
)

// Filter for files
const (
	InputVExtsEnum string = "inputVideoExtsEnum"
	InputMExtsEnum string = "inputMetaExtsEnum"
)

// Performance
const (
	GPUEnum string = "gpuEnum"
)

// Filename edits
const (
	Rename             string = "Rename"
	FileDateFmt        string = "filenameDateTag"
	FilenameReplaceSfx string = "filenameReplaceSfx"
)

// Meta edits
const (
	MOverwrite string = "meta-overwrite"
	MPreserve  string = "meta-preserve"

	MAppend      string = "metaAppend"
	MNewField    string = "metaNewField"
	MPrefix      string = "metaPrefix"
	MReplaceText string = "metaReplaceText"
	MTrimPrefix  string = "metaTrimPrefix"
	MTrimSuffix  string = "metaTrimSuffix"

	MDateTagMap string = "metaDateTagMap"
)

// Contains the fields which accept multiple entries as string arrays
var MultiEntryFields = []string{
	InputVideoExts,
	InputMetaExts,
	FilePrefixes,
	FilenameReplaceSfx,
}
