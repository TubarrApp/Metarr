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

var batchPool = &sync.Pool{
	New: func() any {
		return &batchProcessor{
			failures: struct {
				items []failedVideo
				pool  []failedVideo
				mu    sync.Mutex
			}{
				items: make([]failedVideo, 0, 32),
				pool:  make([]failedVideo, 0, 32),
			},
		}
	},
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

	// Collect all processed files from all batches
	allProcessedFiles := []*models.FileData{} // Do not assign length, parent uses length to go into renaming

	// Begin iteration...
	skipVideos := abstractions.GetBool(keys.SkipVideos)
	failCount := 0
	for _, b := range batches {
		var (
			openVideo *os.File
			openJSON  *os.File
			err       error
		)

		batch := convertCfgToBatch(b)
		logger.Pl.I("Starting batch job %d. Skip videos on this run? %v", job, batch.SkipVideos)

		if batch.SkipVideos {
			skipVideos = true
		}

		// Open video file if necessary
		if !skipVideos {
			if openVideo, err = os.Open(batch.Video); err != nil {
				logger.Pl.E("Failed to open %s", batch.Video)
				failCount++
				continue
			}
		}

		// Open JSON file
		if openJSON, err = os.Open(batch.JSON); err != nil {
			logger.Pl.E("Failed to open %s", batch.JSON)
			// Close accompanying video...
			if openVideo != nil {
				if err := openVideo.Close(); err != nil {
					return allProcessedFiles, fmt.Errorf("failed to close failed video %q after JSON failure: %w", openVideo.Name(), err)
				}
			}
			failCount++
			continue
		}

		// Initiate batch process
		processedFiles, err := processBatch(batch, core, openVideo, openJSON)
		if err != nil {
			logger.Pl.E("Batch with ID %d failed: %v", batch.bp.batchID, err)
			failCount++
			continue
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

		logger.Pl.I("Finished tasks for:\n\n%sInput JSON %s: %q\n", videoDoneMsg, fileOrDirMsg, batch.JSON)

		// Close files explicitly at the end of each iteration
		if openVideo != nil {
			if err := openVideo.Close(); err != nil {
				logger.Pl.E("Failed to close video file %q after successful iteration: %v", openVideo.Name(), err)
			}
		}
		if err := openJSON.Close(); err != nil {
			logger.Pl.E("Failed to close JSON file %q after successful iteration: %v", openJSON.Name(), err)
		}
		job++
	}

	if failCount == len(batches) {
		return nil, fmt.Errorf("all batches failed")
	}

	logger.Pl.I("All batch tasks finished!")
	return allProcessedFiles, nil
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

// processBatch is the entrypoint for batch processing.
func processBatch(batch *batch, core *models.Core, openVideo, openMeta *os.File) (fdArray []*models.FileData, err error) {
	if batch == nil {
		return nil, errors.New("batch entered null")
	}

	if batch.bp, err = getNewBatchProcessor(batch.ID); err != nil {
		return nil, err
	}
	defer batch.bp.release()

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

// getBatchProcessor returns the singleton batchProcessor instance.
func getNewBatchProcessor(batchID int64) (*batchProcessor, error) {
	bp, ok := batchPool.Get().(*batchProcessor)
	if !ok || bp == nil {
		return nil, fmt.Errorf("internal error: got type %T for batch processor with ID %d", bp, batchID)
	}
	bp.batchID = batchID
	return bp, nil
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
	m.Range(func(key, value interface{}) bool {
		if fd, ok := value.(*models.FileData); ok {
			result[key.(string)] = fd
		}
		return true
	})
	return result
}

// reset prepares the batch processor for new batch operation.
func (bp *batchProcessor) reset(expectedCount int) {
	// Reset counters
	atomic.StoreInt32(&bp.counts.totalMeta, 0)
	atomic.StoreInt32(&bp.counts.totalVideo, 0)
	atomic.StoreInt32(&bp.counts.processedMeta, 0)
	atomic.StoreInt32(&bp.counts.processedVideo, 0)

	// Replace maps
	bp.files.matched = sync.Map{}
	bp.files.video = sync.Map{}

	// Reset failures
	bp.failures.mu.Lock()
	switch {
	case bp.failures.pool == nil:
		bp.failures.pool = make([]failedVideo, 0, max(32, expectedCount))

	case cap(bp.failures.pool) >= expectedCount:
		bp.failures.pool = bp.failures.pool[:0]

	default:
		newCap := max(expectedCount, cap(bp.failures.pool)*2)
		bp.failures.pool = make([]failedVideo, 0, newCap)
	}
	bp.failures.items = bp.failures.pool
	bp.failures.mu.Unlock()
}

// release returns the batchProcessor to the pool and sets values back to defaults.
func (bp *batchProcessor) release() {
	bp.reset(0)
	bp.batchID = 0
	batchPool.Put(bp)
}
