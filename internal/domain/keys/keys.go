package domain

// Terminal keys
const (
	VideoDir                string = "video-dir"
	VideoFile               string = "video-file"
	JsonDir                 string = "json-dir"
	JsonFile                string = "json-file"
	InputExts               string = "input-exts"
	FilePrefixes            string = "prefix"
	Concurrency             string = "concurrency"
	MaxCPU                  string = "max-cpu"
	MinMem                  string = "min-mem"
	MinMemMB                string = "min-mem-mb"
	GPU                     string = "gpu"
	GetLatest               string = "get-latest"
	MFilenamePfx            string = "metadata-filename-prefix"
	InputFilenameReplaceSfx string = "filename-replace-suffix"
	InputFileDatePfx        string = "filename-date-tag"
	RenameStyle             string = "input-rename-style"
	MReplaceSfx             string = "meta-trim-suffix"
	MReplacePfx             string = "meta-trim-prefix"
	MNewField               string = "meta-add-field"
	MOverwrite              string = "meta-overwrite"
	MPreserve               string = "meta-preserve"
	DebugLevel              string = "debug-level"
	SkipVideos              string = "skip-videos"
	NoFileOverwrite         string = "no-file-overwrite"
	MDescDatePfx            string = "desc-date-prefix"
	MDescDateSfx            string = "desc-date-suffix"
	InputPreset             string = "preset"
	MoveOnComplete          string = "output-directory"
	OutputFiletype          string = "ext"
)

// Primary program
const (
	Context    string = "Context"
	WaitGroup  string = "WaitGroup"
	SingleFile string = "SingleFile"
)

// Files and directories
const (
	OpenVideo string = "openVideo"
	OpenJson  string = "openJson"
	VideoMap  string = "videoMap"
	MetaMap   string = "metaMap"
)

// Filter for files
const (
	InputExtsEnum string = "inputExtsEnum"
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

// Contains the fields which accept multiple entries as string arrays
var MultiEntryFields []string = []string{
	InputExts,
	FilePrefixes,
	MReplaceSfx,
	MReplacePfx,
	MNewField,
	FilenameReplaceSfx,
}
