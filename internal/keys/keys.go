package keys

// Terminal keys
const (
	VideoDir                string = "video-dir"
	JsonDir                 string = "json-dir"
	InputExts               string = "input-exts"
	FilePrefixes            string = "prefix"
	Concurrency             string = "concurrency"
	MaxCPU                  string = "max-cpu"
	MinMem                  string = "min-mem"
	MinMemMB                string = "min-mem-mb"
	GPU                     string = "gpu"
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
)

// Primary program
const (
	Context   string = "Context"
	WaitGroup string = "WaitGroup"
)

// Files and directories
const (
	OpenVideoDir string = "openVideoDir"
	OpenJsonDir  string = "openJsonDir"
	VideoMap     string = "videoMap"
	MetaMap      string = "metaMap"
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
