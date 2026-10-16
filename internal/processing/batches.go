// Package processing is the primary Metarr process, handling batches of files and/or directories.
package processing

import (
	"errors"
	"fmt"
	"metarr/internal/abstractions"
	"metarr/internal/domain/keys"
	"metarr/internal/domain/logger"
	"metarr/internal/domain/vars"
	"metarr/internal/models"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
)

var atomID atomic.Int64

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
		totalMeta      atomic.Int32
		totalVideo     atomic.Int32
		totalMatched   atomic.Int32
		processedMeta  atomic.Int32
		processedVideo atomic.Int32
	}

	files struct {
		matched sync.Map
		video   sync.Map
		metaLen int32
	}

	failures struct {
		items []failedVideo
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
		logger.Pl.I("No batches to process. Exiting.")
		return nil, nil
	}
	job := 1

	// Collect all processed files from all batches.
	allProcessedFiles := []*models.FileData{} // Do not assign length.

	// Begin iteration...
	skipVideos := abstractions.GetBool(keys.SkipVideos)
	failCount := 0
	for _, b := range batches {
		batch := convertCfgToBatch(b)
		batch.SkipVideos = batch.SkipVideos || skipVideos
		logger.Pl.I("Starting batch job %d. Skip videos on this run? %v", job, batch.SkipVideos)

		processedFiles, err := runBatch(batch, core)
		if err != nil {
			logger.Pl.E("Batch with ID %d failed: %v", batch.ID, err)
			failCount++
			continue
		}
		allProcessedFiles = append(allProcessedFiles, processedFiles...)
		job++
	}

	if failCount == len(batches) {
		return nil, fmt.Errorf("all batches failed")
	}

	logger.Pl.I("All batch tasks finished!")
	return allProcessedFiles, nil
}

// runBatch opens a batch's video and JSON paths, processes them, and closes them.
func runBatch(batch *batch, core *models.Core) ([]*models.FileData, error) {
	var openVideo *os.File
	if !batch.SkipVideos {
		f, err := os.Open(batch.Video)
		if err != nil {
			return nil, fmt.Errorf("failed to open %s: %w", batch.Video, err)
		}
		openVideo = f
		defer closeBatchFile(openVideo)
	}

	openJSON, err := os.Open(batch.JSON)
	if err != nil {
		return nil, fmt.Errorf("failed to open %s: %w", batch.JSON, err)
	}
	defer closeBatchFile(openJSON)

	processedFiles, err := processBatch(batch, core, openVideo, openJSON)
	if err != nil {
		return nil, err
	}

	fileOrDirMsg := "Directory"
	if !batch.IsDirs {
		fileOrDirMsg = "File"
	}
	var videoDoneMsg string
	if !batch.SkipVideos {
		videoDoneMsg = fmt.Sprintf("Input Video %s: %q\n", fileOrDirMsg, batch.Video)
	}
	logger.Pl.I("Finished tasks for:\n\n%sInput JSON %s: %q\n", videoDoneMsg, fileOrDirMsg, batch.JSON)
	return processedFiles, nil
}

// closeBatchFile closes a batch's input file, logging any error.
func closeBatchFile(f *os.File) {
	if err := f.Close(); err != nil {
		logger.Pl.E("Failed to close %q: %v", f.Name(), err)
	}
}

// initializeBatchConfigs ensures the entered files and directories are valid and creates batch pairs.
func initializeBatchConfigs() (batchConfig []models.BatchConfig, err error) {
	videoDirs, videoFiles, jsonDirs, jsonFiles, err := getFileDirs()
	if err != nil {
		return nil, err
	}
	logger.Pl.P("Finding video and JSON directories...")

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
	logger.Pl.I("Got %d directory pairs to process, %d singular JSON directories", vDirCount, len(jsonDirs)-vDirCount)

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
	logger.Pl.I("Finding video and JSON files...")

	// Make file batches
	if len(videoFiles) > 0 {
		for i := range videoFiles {
			var newBatchConfig = models.BatchConfig{}
			logger.Pl.D(3, "Checking video file %q ...", videoFiles[i])

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
				logger.Pl.D(3, "Checking JSON file %q ...", jsonFiles[i])
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

		logger.Pl.I("Got %d file pairs to process, %d singular JSON files", vFileCount, len(jsonFiles)-len(videoFiles))

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
	logger.Pl.I("Got %d batch jobs to perform.", len(tasks))
	return tasks, nil
}

// convertCfgToBatch converts a config batch to a local batch.
func convertCfgToBatch(config models.BatchConfig) *batch {
	id := atomID.Add(1)
	newBatch := &batch{
		ID:         id,
		Video:      config.Video,
		JSON:       config.JSON,
		IsDirs:     config.IsDirs,
		SkipVideos: config.SkipVideos,
	}
	return newBatch
}

// processBatch is the entrypoint for batch processing.
func processBatch(batch *batch, core *models.Core, openVideo, openMeta *os.File) (fdArray []*models.FileData, err error) {
	if batch == nil {
		return nil, errors.New("batch entered null")
	}

	batch.bp = &batchProcessor{batchID: batch.ID}

	if fdArray, err = processFiles(batch, core, openVideo, openMeta); err != nil {
		return fdArray, err
	}

	errArray := vars.GetErrorArray()
	if len(errArray) == 0 {
		fmt.Fprintf(os.Stderr, "\n")
		logger.Pl.S("Successfully processed all files in directory %q with no errors.\n", filepath.Dir(batch.bp.filepaths.metaFile))
		return fdArray, nil
	}
	return fdArray, nil
}

// addFailedVideo aadds a new failed video to the array.
func (bp *batchProcessor) addFailure(f failedVideo) {
	bp.failures.mu.Lock()
	bp.failures.items = append(bp.failures.items, f)
	bp.failures.mu.Unlock()
}

// logFailedVideos logs videos which failed during this batch.
func (bp *batchProcessor) logFailedVideos() {
	if len(bp.failures.items) == 0 {
		return
	}

	for i, failed := range bp.failures.items {
		if i == 0 {
			logger.Pl.E("Batch finished, but some errors were encountered:")
		}
		fmt.Fprintf(os.Stderr, "\n")
		logger.Pl.P("Filename: %v", failed.filename)
		logger.Pl.P("Error: %v", failed.err)
	}
	fmt.Fprintf(os.Stderr, "\n")
}

// syncMapToRegularMap converts the sync map back to a regular map for further processing.
func (bp *batchProcessor) syncMapToRegularMap(m *sync.Map) map[string]*models.FileData {
	result := make(map[string]*models.FileData)
	m.Range(func(key, value any) bool {
		if fd, ok := value.(*models.FileData); ok {
			result[key.(string)] = fd
		}
		return true
	})
	return result
}
