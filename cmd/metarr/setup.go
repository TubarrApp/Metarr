package main

import (
	"fmt"
	"metarr/internal/domain/paths"
	"metarr/internal/utils/logging"
	"os"
	"path/filepath"
)

// initializeApplication sets up the application for the current run.
func initializeApplication() {
	// Setup files/dirs
	if err := paths.InitProgFilesDirs(); err != nil {
		fmt.Printf("Metarr exiting with error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("\nMain Metarr file/dir locations:\n\nMetarr Directories: %s\nLog file: %s\n\n",
		paths.HomeMetarrDir, paths.LogFilePath)

	// Start logging
	logDir := filepath.Dir(paths.LogFilePath)
	fmt.Printf("Setting log file at %q\n\n", logDir)

	if err := logging.SetupLogging(logDir); err != nil {
		fmt.Printf("\n\nWarning: Log file was not created\nReason: %s\n\n", err)
	}
}
