package main

import (
	"errors"
	"fmt"
	"metarr/internal/abstractions"
	"metarr/internal/domain/keys"
	"metarr/internal/domain/paths"
	"metarr/internal/models"
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
	fmt.Printf("Setting log file at %q", logDir)

	if err := logging.SetupLogging(logDir); err != nil {
		fmt.Printf("\n\nWarning: Log file was not created\nReason: %s\n\n", err)
	}
}

// initializeBatchConfigs ensures the entered files and directories are valid and creates batch pairs.
func initializeBatchConfigs(metaOps *models.MetaOps) (batchConfig []models.BatchConfig, err error) {
	metaOps = models.EnsureMetaOps(metaOps)

	var videoDirs, videoFiles, jsonDirs, jsonFiles []string
	if abstractions.IsSet(keys.VideoFiles) {
		videoFiles = abstractions.GetStringSlice(keys.VideoFiles)
	}

	if abstractions.IsSet(keys.VideoDirs) {
		videoDirs = abstractions.GetStringSlice(keys.VideoDirs)
	}

	if abstractions.IsSet(keys.JSONFiles) {
		jsonFiles = abstractions.GetStringSlice(keys.JSONFiles)
	}

	if abstractions.IsSet(keys.JSONDirs) {
		jsonDirs = abstractions.GetStringSlice(keys.JSONDirs)
	}

	videoDirs, videoFiles, jsonDirs, jsonFiles = getValidFileDirs(videoDirs, videoFiles, jsonDirs, jsonFiles)

	if len(videoDirs) > len(jsonDirs) || len(videoFiles) > len(jsonFiles) {
		return nil, errors.New("invalid configuration, please enter a meta directory/file for each video directory/file")
	}

	var tasks = make([]models.BatchConfig, 0, (len(jsonDirs) + len(videoDirs) + len(jsonFiles) + len(videoFiles)))

	logging.I("Finding video and JSON directories...")

	// Make directory batches
	vDirCount := 0
	vFileCount := 0
	if len(videoDirs) > 0 {
		for i := range videoDirs {
			var newBatchConfig = models.BatchConfig{
				MetaOps: models.EnsureMetaOps(metaOps),
			}

			// Video directories

			newBatchConfig.Video = videoDirs[i]

			// JSON directories
			if len(jsonDirs) > i {
				jInfo, err := os.Stat(jsonDirs[i])
				if err != nil {
					return nil, err
				}
				if !jInfo.IsDir() {
					return nil, fmt.Errorf("file %q entered instead of directory", jInfo.Name())
				}
				newBatchConfig.JSON = jsonDirs[i]
			}

			// IsDirs
			newBatchConfig.IsDirs = true

			tasks = append(tasks, newBatchConfig)
			vDirCount++
		}
	}
	logging.I("Got %d directory pairs to process, %d singular JSON directories", vDirCount, len(jsonDirs)-vDirCount)

	// Remnant JSON directories
	if len(jsonDirs) > vDirCount {
		remnantJSONDirs := jsonDirs[vDirCount:]

		for i := range remnantJSONDirs {
			var newBatchConfig = models.BatchConfig{
				MetaOps: models.EnsureMetaOps(metaOps),
			}

			// JSON directories
			jInfo, err := os.Stat(remnantJSONDirs[i])
			if err != nil {
				return nil, err
			}
			if !jInfo.IsDir() {
				return nil, fmt.Errorf("file %q entered instead of directory", jInfo.Name())
			}

			// BatchConfig model settings:
			newBatchConfig.JSON = remnantJSONDirs[i]
			newBatchConfig.IsDirs = true
			newBatchConfig.SkipVideos = true

			tasks = append(tasks, newBatchConfig)
		}
	}

	logging.I("Finding video and JSON files...")

	// Make file batches
	if len(videoFiles) > 0 {
		for i := range videoFiles {
			var newBatchConfig = models.BatchConfig{
				MetaOps: models.EnsureMetaOps(metaOps),
			}
			logging.D(3, "Checking video file %q ...", videoFiles[i])

			// Video files
			vInfo, err := os.Stat(videoFiles[i])
			if err != nil {
				return nil, err
			}
			if vInfo.IsDir() {
				return nil, fmt.Errorf("directory %q entered instead of file", vInfo.Name())
			}
			newBatchConfig.Video = videoFiles[i]

			// JSON files
			if len(jsonFiles) > i {
				logging.D(3, "Checking JSON file %q ...", jsonFiles[i])
				jInfo, err := os.Stat(jsonFiles[i])
				if err != nil {
					return nil, err
				}
				if jInfo.IsDir() {
					return nil, fmt.Errorf("directory %q entered instead of file", jInfo.Name())
				}
				newBatchConfig.JSON = jsonFiles[i]
			}
			// NOT Dirs
			newBatchConfig.IsDirs = false

			tasks = append(tasks, newBatchConfig)
			vFileCount++
		}

		logging.I("Got %d file pairs to process, %d singular JSON files", vFileCount, len(jsonFiles)-len(videoFiles))

		// Remnant JSON files
		if len(jsonFiles) > vFileCount {
			remnantJSONFiles := jsonFiles[vFileCount:]
			var newBatchConfig = models.BatchConfig{
				MetaOps: models.EnsureMetaOps(metaOps),
			}

			// JSON Files
			for i := range remnantJSONFiles {
				jInfo, err := os.Stat(remnantJSONFiles[i])
				if err != nil {
					return nil, err
				}
				if jInfo.IsDir() {
					return nil, fmt.Errorf("directory %q entered instead of file", jInfo.Name())
				}

				// BatchConfig model settings:
				newBatchConfig.JSON = remnantJSONFiles[i]
				newBatchConfig.IsDirs = false
				newBatchConfig.SkipVideos = true

				tasks = append(tasks, newBatchConfig)
			}
		}
	}

	logging.I("Got %d batch jobs to perform.", len(tasks))
	return tasks, nil
}

// getValidFileDirs checks for validity of files and directories, with fallback handling.
func getValidFileDirs(videoDirs, videoFiles, jsonDirs, jsonFiles []string) (vDirs, vFiles, jDirs, jFiles []string) {
	vDirs, misplacedVFiles := validatePaths("video directory", videoDirs)
	misplacedVDirs, vFiles := validatePaths("video file", videoFiles)
	jDirs, misplacedJFiles := validatePaths("JSON directory", jsonDirs)
	misplacedJDirs, jFiles := validatePaths("JSON file", jsonFiles)

	// Log and reassign misplaced entries
	for _, f := range misplacedVFiles {
		logging.W("User entered file %q as directory, appending to video files", f)
		vFiles = append(vFiles, f)
	}
	for _, d := range misplacedVDirs {
		logging.W("User entered directory %q as file, appending to video directories", d)
		vDirs = append(vDirs, d)
	}
	for _, f := range misplacedJFiles {
		logging.W("User entered file %q as directory, appending to valid JSON files", f)
		jFiles = append(jFiles, f)
	}
	for _, d := range misplacedJDirs {
		logging.W("User entered directory %q as file, appending to valid JSON directories", d)
		jDirs = append(jDirs, d)
	}

	return vDirs, vFiles, jDirs, jFiles
}

// validatePaths checks whether each path in 'paths' is a directory or file.
//
// It classifies them into dirs and files while logging consistent warnings.
func validatePaths(kind string, paths []string) (dirs, files []string) {
	for _, p := range paths {
		info, err := os.Stat(p)
		if err != nil {
			logging.E("Failed to stat %s path %q: %v", kind, p, err)
			continue
		}
		if info.IsDir() {
			dirs = append(dirs, p)
		} else {
			files = append(files, p)
		}
	}
	return dirs, files
}
