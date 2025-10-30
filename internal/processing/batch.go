// Package processing is the primary Metarr process, handling batches of files and/or directories.
package processing

import (
	"fmt"
	"metarr/internal/abstractions"
	"metarr/internal/domain/keys"
	"metarr/internal/models"
	"metarr/internal/utils/logging"
	"os"
	"sync"
	"sync/atomic"
)

var atomID int64

type batch struct {
	ID         int64
	Video      string
	JSON       string
	IsDirs     bool
	SkipVideos bool
	bp         *batchProcessor
}

type batchProcessor struct {
	batchID int64

	counts struct {
		totalMeta      int32
		totalVideo     int32
		totalMatched   int32
		processedMeta  int32
		processedVideo int32
	}

	files struct {
		matched sync.Map
		video   sync.Map
		metaLen int32
	}

	failures struct {
		items []failedVideo
		pool  []failedVideo
		mu    sync.Mutex
	}

	filepaths struct {
		mu        sync.RWMutex
		videoFile string
		metaFile  string
		directory string
	}
}

// ProcessBatches begins processing the batch.
func ProcessBatches(core *models.Core) ([]*models.FileData, error) {
	batches, err := initializeBatchConfigs()
	if err != nil {
		return nil, err
	}
	if len(batches) == 0 {
		logging.I("No batches to process. Exiting.")
		return nil, nil
	}
	job := 1

	// Collect all processed files from all batches
	allProcessedFiles := make([]*models.FileData, len(batches))

	// Begin iteration...
	skipVideos := abstractions.GetBool(keys.SkipVideos)
	for _, b := range batches {
		var (
			openVideo *os.File
			openJSON  *os.File
			err       error
		)

		batch := convertCfgToBatch(b)
		logging.I("Starting batch job %d. Skip videos on this run? %v", job, batch.SkipVideos)

		if batch.SkipVideos {
			skipVideos = true
		}

		// Open video file if necessary
		if !skipVideos {
			if openVideo, err = os.Open(batch.Video); err != nil {
				logging.E("Failed to open %s", batch.Video)
				continue
			}
		}

		// Open JSON file
		if openJSON, err = os.Open(batch.JSON); err != nil {
			logging.E("Failed to open %s", batch.JSON)
			// Close accompanying video...
			if openVideo != nil {
				if err := openVideo.Close(); err != nil {
					return allProcessedFiles, fmt.Errorf("failed to close failed video %q after JSON failure: %w", openVideo.Name(), err)
				}
			}
			continue
		}

		// Initiate batch process
		processedFiles, err := processBatch(batch, core, openVideo, openJSON)
		if err != nil {
			return allProcessedFiles, err
		}

		// Append this batch's files to the overall collection
		allProcessedFiles = append(allProcessedFiles, processedFiles...)

		// Completion message
		fileOrDirMsg := "Directory"
		if !batch.IsDirs {
			fileOrDirMsg = "File"
		}

		var videoDoneMsg string
		if !batch.SkipVideos {
			videoDoneMsg = fmt.Sprintf("Input Video %s: %q\n", fileOrDirMsg, batch.Video)
		}

		logging.I("Finished tasks for:\n\n%sInput JSON %s: %q\n", videoDoneMsg, fileOrDirMsg, batch.JSON)

		// Files will be closed by setupCleanup when context is done
		job++
	}
	logging.I("All batch tasks finished!")
	return allProcessedFiles, nil
}

// initializeBatchConfigs ensures the entered files and directories are valid and creates batch pairs.
func initializeBatchConfigs() (batchConfig []models.BatchConfig, err error) {
	videoDirs, videoFiles, jsonDirs, jsonFiles, err := getFileDirs()
	if err != nil {
		return nil, err
	}
	logging.P("Finding video and JSON directories...")

	// Make directory batches
	vDirCount := 0
	vFileCount := 0
	tasks := make([]models.BatchConfig, 0, (len(jsonDirs) + len(videoDirs) + len(jsonFiles) + len(videoFiles)))
	if len(videoDirs) > 0 {
		for i := range videoDirs {
			var newBatchConfig = models.BatchConfig{}

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

			// Send to tasks
			tasks = append(tasks, newBatchConfig)
			vDirCount++
		}
	}
	logging.I("Got %d directory pairs to process, %d singular JSON directories", vDirCount, len(jsonDirs)-vDirCount)

	// Remnant JSON directories
	if len(jsonDirs) > vDirCount {
		remnantJSONDirs := jsonDirs[vDirCount:]
		for i := range remnantJSONDirs {
			var newBatchConfig = models.BatchConfig{}

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

			// Send to tasks
			tasks = append(tasks, newBatchConfig)
		}
	}
	logging.I("Finding video and JSON files...")

	// Make file batches
	if len(videoFiles) > 0 {
		for i := range videoFiles {
			var newBatchConfig = models.BatchConfig{}
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

			// Send to tasks
			tasks = append(tasks, newBatchConfig)
			vFileCount++
		}

		logging.I("Got %d file pairs to process, %d singular JSON files", vFileCount, len(jsonFiles)-len(videoFiles))

		// Remnant JSON files
		if len(jsonFiles) > vFileCount {
			remnantJSONFiles := jsonFiles[vFileCount:]
			var newBatchConfig = models.BatchConfig{}

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

				// Send to tasks
				tasks = append(tasks, newBatchConfig)
			}
		}
	}
	logging.I("Got %d batch jobs to perform.", len(tasks))
	return tasks, nil
}

// convertCfgToBatch converts a config batch to a local batch.
func convertCfgToBatch(config models.BatchConfig) *batch {
	id := atomic.AddInt64(&atomID, 1)
	newBatch := &batch{
		ID:         id,
		Video:      config.Video,
		JSON:       config.JSON,
		IsDirs:     config.IsDirs,
		SkipVideos: config.SkipVideos,
	}
	return newBatch
}
