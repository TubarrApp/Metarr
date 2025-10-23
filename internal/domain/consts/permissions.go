package consts

// Recommended permissions for different types of files and directories Metarr might create.
const (
	// ** World Readable **
	// Media directories - world readable
	PermsGenericDir    = 0o755
	PermsJSONDir       = 0o755
	PermsVideoDir      = 0o755
	PermsHomeMetarrDir = 0o700

	// Media files - world readable
	PermsVideoFile = 0o644
	PermsJSONFile  = 0o644

	// Other files
	PermsLogFile = 0o644

	// ** Private **
	// Sensitive files - owner only
	PermsCookieDir  = 0o750 // Private auth directory
	PermsCookieFile = 0o600 // Private cookie files
	PermsConfigFile = 0o600 // Private config with passwords
)
