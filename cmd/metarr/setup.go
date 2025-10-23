package main

import (
	"errors"
	"fmt"
	"metarr/internal/domain/keys"
	"metarr/internal/domain/paths"
	"metarr/internal/models"
	"metarr/internal/utils/logging"
	"os"

	"github.com/spf13/viper"
)

// initializeApplication sets up the application for the current run.
func initializeApplication() error {
	// Setup files/dirs
	if err := paths.InitProgFilesDirs(); err != nil {
		fmt.Printf("Metarr exiting with error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("\nMain Metarr file/dir locations:\n\nMetarr Directories: %s\nLog file: %s\n\n",
		paths.HomeMetarrDir, paths.LogFilePath)

	// Setup logging
	if err := logging.SetupLogging(paths.HomeMetarrDir); err != nil {
		fmt.Printf("could not set up logging, proceeding without: %v", err)
	}

	return nil
}

// initializeBatchConfigs ensures the entered files and directories are valid and creates batch pairs.
func initializeBatchConfigs(metaOps *models.MetaOps) (batchConfig []models.BatchConfig, err error) {
	metaOps = models.EnsureMetaOps(metaOps)

	var videoDirs, videoFiles, jsonDirs, jsonFiles []string
	if viper.IsSet(keys.VideoFiles) {
		videoFiles = viper.GetStringSlice(keys.VideoFiles)
	}

	if viper.IsSet(keys.VideoDirs) {
		videoDirs = viper.GetStringSlice(keys.VideoDirs)
	}

	if viper.IsSet(keys.JSONFiles) {
		jsonFiles = viper.GetStringSlice(keys.JSONFiles)
	}

	if viper.IsSet(keys.JSONDirs) {
		jsonDirs = viper.GetStringSlice(keys.JSONDirs)
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

// getValidFileDirs checks for validity of files and directories.
func getValidFileDirs(videoDirs, videoFiles, jsonDirs, jsonFiles []string) (vDirs, vFiles, jDirs, jFiles []string) {
	validVDirs := make([]string, 0, len(videoDirs))
	validVFiles := make([]string, 0, len(videoFiles))
	validJDirs := make([]string, 0, len(jsonDirs))
	validJFiles := make([]string, 0, len(jsonFiles))

	// Check video directories
	for _, vDir := range videoDirs {
		vInfo, err := os.Stat(vDir)
		if err != nil {
			logging.E("Failed to stat video directory %q: %v", vDir, err)
			continue
		}
		if !vInfo.IsDir() {
			logging.W("User entered file %q as directory, appending to video files", vDir)
			validVFiles = append(validVFiles, vDir)
			continue
		}
		validVDirs = append(validVDirs, vDir)
	}

	// Check video files
	for _, vFile := range videoFiles {
		vInfo, err := os.Stat(vFile)
		if err != nil {
			logging.E("Failed to stat video file %q: %v", vFile, err)
			continue
		}
		if vInfo.IsDir() {
			logging.W("User entered directory %q as file, appending to valid video directories", vFile)
			validVDirs = append(validVFiles, vFile)
			continue
		}
		validVFiles = append(validVDirs, vFile)
	}

	// Check JSON directories
	for _, jDir := range jsonDirs {
		vInfo, err := os.Stat(jDir)
		if err != nil {
			logging.E("Failed to stat JSON directory %q: %v", jDir, err)
			continue
		}
		if !vInfo.IsDir() {
			logging.W("User entered file %q as directory, appending to valid JSON files", jDir)
			validVFiles = append(validVFiles, jDir)
			continue
		}
		validJDirs = append(validJDirs, jDir)
	}

	// Check JSON files
	for _, jFile := range jsonFiles {
		vInfo, err := os.Stat(jFile)
		if err != nil {
			logging.E("Failed to stat video file %q: %v", jFile, err)
			continue
		}
		if vInfo.IsDir() {
			logging.W("User ntered directory %q as file, appending to valid JSON directories", jFile)
			validVDirs = append(validVFiles, jFile)
			continue
		}
		validJFiles = append(validJFiles, jFile)
	}
	return validVDirs, validVFiles, validJDirs, validJFiles
}
