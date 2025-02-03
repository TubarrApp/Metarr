// Package enums holds enumerated variables.
package enums

// ConvertFromFiletype is the type of filename to perform conversions on.
type ConvertFromFiletype int

const (
	VidExtsAll ConvertFromFiletype = iota
	VidExtsMKV
	VidExtsMP4
	VidExtsWebM
)

// MetaFiletypeFilter filters the metadata files to read from.
type MetaFiletypeFilter int

const (
	MetaExtsAll MetaFiletypeFilter = iota
	MetaExtsJSON
	MetaExtsNFO
)

// OverrideMetaType holds the value for the type of metafield to override all values of (e.g. "credits").
type OverrideMetaType int

const (
	OverrideMetaNone OverrideMetaType = iota
	OverrideMetaCredits
)

// MetaOpType stores the meta operation type directive (e.g. set).
type MetaOpType int

const (
	MetaOpsNone MetaOpType = iota
	MetaOpsSet
)

// ReplaceToStyle dictates a naming convention to use, e.g. spaces or underscores.
//
// RenamingFixesOnly only fixes things like contractions, without changing the style.
type ReplaceToStyle int

const (
	RenamingSkip ReplaceToStyle = iota
	RenamingFixesOnly
	RenamingSpaces
	RenamingUnderscores
)

// DateFormat holds the date format directive (e.g. yyyy-mm-dd).
type DateFormat int

const (
	DateFmtSkip DateFormat = iota
	DateYyyyMmDd
	DateYyMmDd
	DateYyyyDdMm
	DateYyDdMm
	DateDdMmYyyy
	DateDdMmYy
	DateMmDdYyyy
	DateMmDdYy
	DateDdMm
	DateMmDd
)

// MetaDateTagLocation determines where a date tag should be added in a string.
type MetaDateTagLocation int

const (
	DatetagLocPrefix MetaDateTagLocation = iota
	DatetagLocSuffix
)

// MetaDateTaggingType determines the type of operation to perform for date tags.
type MetaDateTaggingType int

const (
	DatetagAddOp MetaDateTaggingType = iota
	DatetagDelOp
)

// MetaFiletype is the type of meta file, e.g. JSON or NFO.
type MetaFiletype int

const (
	MetaFiletypeJSON MetaFiletype = iota
	MetaFiletypeNFO
)

// ViperVarTypes relates to Viper configuration variable types.
type ViperVarTypes int

const (
	ViperVarAny ViperVarTypes = iota
	ViperVarBool
	ViperVarInt
	ViperVarString
	ViperVarStringSlice
)

// PurgeMetafiles sets directives for the deletion of metafiles upon process completion.
type PurgeMetafiles int

const (
	PurgeMetaAll PurgeMetafiles = iota
	PurgeMetaJSON
	PurgeMetaNFO
	PurgeMetaNone
)

// WebClassTags relates to the type of data to grab from a web page.
type WebClassTags int

const (
	WebclassDate WebClassTags = iota
	WebclassTitle
	WebclassDescription
	WebclassCredits
	WebclassWebInfo
)

// SitePresets holds presets for different video sites.
type SitePresets int

const (
	PresetCensoredTV SitePresets = iota
)
