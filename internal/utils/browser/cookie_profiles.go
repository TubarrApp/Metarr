package utils

import (
	consts "metarr/internal/domain/constants"
)

// cookieFilePatterns holds the various filenames for cookie/auth related files by browser
var cookieFilePatterns = map[string][]string{

	consts.BrowserFirefox: {
		// Core cookie and storage files
		"cookies.sqlite",
		"cookies.sqlite-shm",
		"cookies.sqlite-wal",
		"storage.sqlite",
		"storage.sqlite-shm",
		"storage.sqlite-wal",
		"webappsstore.sqlite",

		// Encryption and security
		"key*.db",
		"key*.db-journal",
		"cert9.db",
		"cert8.db",
		"pkcs11.txt",
		"secmod.db",
		"logins.json",
		"passwords.json",
		"signons.sqlite",

		// History and places
		"places.sqlite",
		"places.sqlite-shm",
		"places.sqlite-wal",
		"favicons.sqlite",
		"favicons.sqlite-shm",
		"favicons.sqlite-wal",

		// Session and preferences
		"prefs.js",
		"user.js",
		"sessionstore.js",
		"sessionstore.jsonlz4",
		"sessionCheckpoints.json",
		"permissions.sqlite",
		"extensions.json",
		"containers.json",
	},

	consts.BrowserChrome: {
		// Core files
		"Cookies",
		"Cookies-journal",
		"Login Data",
		"Login Data-journal",
		"Web Data",
		"Web Data-journal",

		// Local storage
		"Local Storage/*",
		"IndexedDB/*",

		// Preferences and sync
		"Preferences",
		"Secure Preferences",
		"Session Storage/*",

		// History
		"History",
		"History-journal",
		"Visited Links",

		// Authentication
		"Origin Bound Certs",
		"*.jwt", // Auth token
		"Network/*",
		"Extension Cookies",
		"Extension Cookies-journal",
	},

	consts.BrowserEdge: { // Similar to Chrome as it's Chromium-based
		"Cookies",
		"Cookies-journal",
		"Login Data",
		"Login Data-journal",
		"Web Data",
		"Web Data-journal",
		"Local Storage/*",
		"IndexedDB/*",
		"Preferences",
		"Secure Preferences",
		"Session Storage/*",
		"History",
		"History-journal",
		"Network/*",
		"Extension Cookies",
		"Extension Cookies-journal",
		"*.jwt",           // Edge-specific
		"Microsoft Edge*", // Edge-specific files
	},

	consts.BrowserSafari: { // macOS only
		"Cookies.binarycookies",
		"Cookies.plist",
		"LastSession.plist",
		"History.db",
		"History.db-shm",
		"History.db-wal",
		"Downloads.plist",
		"Extensions.plist",
		"Databases/*",
		"LocalStorage/*",
		"WebKit/WebsiteData/*",
		"Preferences.plist",
		"TopSites.plist",
		"ApplicationCache.db",
		"Cache.db",
		"PerSite/*",
		"SafeBrowsing/*",
	},
}
