package domain

// User selection of filetypes to convert from
type ConvertFromFiletype int

const (
	VID_EXTS_ALL ConvertFromFiletype = iota
	VID_EXTS_MKV
	VID_EXTS_MP4
	VID_EXTS_WEBM
)

// MetaFiletypeFilter filters the metadata files to read from
type MetaFiletypeFilter int

const (
	META_EXTS_ALL MetaFiletypeFilter = iota
	META_EXTS_JSON
	META_EXTS_NFO
)

// User system graphics hardware for transcoding
type SysGPU int

const (
	GPU_NVIDIA SysGPU = iota
	GPU_AMD
	GPU_INTEL
	GPU_NO_HW_ACCEL
)

// Naming syle
type ReplaceToStyle int

const (
	RENAMING_SPACES ReplaceToStyle = iota
	RENAMING_UNDERSCORES
	RENAMING_FIXES_ONLY
	RENAMING_SKIP
)

// Date formats
type FilenameDateFormat int

const (
	FILEDATE_YYYY_MM_DD FilenameDateFormat = iota
	FILEDATE_YY_MM_DD
	FILEDATE_YYYY_DD_MM
	FILEDATE_YY_DD_MM
	FILEDATE_DD_MM_YYYY
	FILEDATE_DD_MM_YY
	FILEDATE_MM_DD_YYYY
	FILEDATE_MM_DD_YY
	FILEDATE_SKIP
)

// Web tags
type MetaFiletypeFound int

const (
	METAFILE_JSON MetaFiletypeFound = iota
	METAFILE_NFO
	WEBCLASS_XML
)

// Viper variable types
type ViperVarTypes int

const (
	VIPER_ANY ViperVarTypes = iota
	VIPER_BOOL
	VIPER_INT
	VIPER_STRING
	VIPER_STRING_SLICE
)

// Web tags
type WebClassTags int

const (
	WEBCLASS_DATE WebClassTags = iota
	WEBCLASS_TITLE
	WEBCLASS_DESCRIPTION
	WEBCLASS_CREDITS
	WEBCLASS_WEBINFO
)

// Presets
type SitePresets int

const (
	PRESET_CENSOREDTV SitePresets = iota
)
