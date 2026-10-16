package consts

// Permissions requested when creating files and directories Metarr might create.
//
// Shared modes leave the final permissions to the process umask (e.g. 0o755/0o644 with the usual 022).
// Private modes are fixed, so a loose umask can't expose them.
const (
	// ** Shared **
	// Media directories
	PermsGenericDir = 0o777
	PermsJSONDir    = 0o777
	PermsVideoDir   = 0o777

	// Media files
	PermsVideoFile = 0o666
	PermsJSONFile  = 0o666

	// Other files
	PermsLogFile = 0o666

	// ** Private **
	// Sensitive files - owner only
	PermsHomeMetarrDir = 0o700
	PermsCookieDir     = 0o750 // Private auth directory
	PermsCookieFile    = 0o600 // Private cookie files
	PermsConfigFile    = 0o600 // Private config with passwords
)
