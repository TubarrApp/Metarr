package enums

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

// OverrideMetaType holds the value for the type of metafield to override all values of (e.g. "credits")
type OverrideMetaType int

const (
	OVERRIDE_META_NONE OverrideMetaType = iota
	OVERRIDE_META_CREDITS
)

type MetaOpType int

const (
	METAOPS_NONE MetaOpType = iota
	METAOPS_SET
)

// User system graphics hardware for transcoding
type SysGPU int

const (
	GPU_NO_HW_ACCEL SysGPU = iota
	GPU_AMD
	GPU_INTEL
	GPU_NVIDIA
)

// Naming syle
type ReplaceToStyle int

const (
	RENAMING_SKIP ReplaceToStyle = iota
	RENAMING_UNDERSCORES
	RENAMING_FIXES_ONLY
	RENAMING_SPACES
)

// Date formats
type DateFormat int

const (
	DATEFMT_SKIP DateFormat = iota
	DATEFMT_YYYY_MM_DD
	DATEFMT_YY_MM_DD
	DATEFMT_YYYY_DD_MM
	DATEFMT_YY_DD_MM
	DATEFMT_DD_MM_YYYY
	DATEFMT_DD_MM_YY
	DATEFMT_MM_DD_YYYY
	DATEFMT_MM_DD_YY
	DATEFMT_DD_MM
	DATEFMT_MM_DD
)

// Date tag location
type MetaDateTagLocation int

const (
	DATE_TAG_LOC_PFX MetaDateTagLocation = iota
	DATE_TAG_LOC_SFX
)

type MetaDateTaggingType int

const (
	DATE_TAG_ADD_OP MetaDateTaggingType = iota
	DATE_TAG_DEL_OP
)

// Web tags
type MetaFiletypeFound int

const (
	METAFILE_JSON MetaFiletypeFound = iota
	METAFILE_NFO
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

// Purge metafile types
type PurgeMetafiles int

const (
	PURGEMETA_ALL PurgeMetafiles = iota
	PURGEMETA_JSON
	PURGEMETA_NFO
	PURGEMETA_NONE
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
