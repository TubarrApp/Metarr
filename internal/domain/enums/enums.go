// Package enums holds enumerated variables.
package enums

// OverrideMetaType holds the value for the type of metafield to override all values of (e.g. "credits").
type OverrideMetaType int

// OverrideMetaType definitions.
const (
	OverrideMetaNone OverrideMetaType = iota
	OverrideMetaCredits
	OverrideMetaDates
	OverrideMetaDescriptions
	OverrideMetaTitles
)

// MetaOpType stores the meta operation type directive (e.g. set).
type MetaOpType int

// MetaOpType definitions.
const (
	MetaOpsNone MetaOpType = iota
	MetaOpsSet
)

// ReplaceToStyle dictates a naming convention to use, e.g. spaces or underscores.
//
// RenamingFixesOnly only fixes things like contractions, without changing the style.
type ReplaceToStyle int

// ReplaceToStyle definitions.
const (
	RenamingSkip ReplaceToStyle = iota
	RenamingFixesOnly
	RenamingSpaces
	RenamingUnderscores
)

// MetaFiletype is the type of meta file, e.g. JSON or NFO.
type MetaFiletype int

// MetaFiletype definitions.
const (
	MetaFiletypeJSON MetaFiletype = iota
	MetaFiletypeNFO
)

// ViperVarTypes relates to Viper configuration variable types.
type ViperVarTypes int

// ViperVarTypes definitions.
const (
	ViperVarAny ViperVarTypes = iota
	ViperVarBool
	ViperVarInt
	ViperVarString
	ViperVarStringSlice
)

// PurgeMetafiles sets directives for the deletion of metafiles upon process completion.
type PurgeMetafiles int

// PurgeMetafiles definitions.
const (
	PurgeMetaAll PurgeMetafiles = iota
	PurgeMetaJSON
	PurgeMetaNFO
	PurgeMetaNone
)
