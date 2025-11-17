package processing

import (
	"context"
	"fmt"
	"metarr/internal/abstractions"
	"metarr/internal/domain/consts"
	"metarr/internal/domain/keys"
	"metarr/internal/domain/logger"
	"metarr/internal/domain/vars"
	"metarr/internal/ffmpeg"
	"metarr/internal/file"
	"metarr/internal/models"
	"os"
	"path/filepath"
	"runtime/debug"
	"sync"
	"sync/atomic"
)

const (
	typeVideo = "video"
	typeMeta  = "metadata"
)

type failedVideo struct {
	filename string
	err      string
}

type workItem struct {
	filename     string
	fileData     *models.FileData
	metaFilename string
	skipVids     bool
}

// processFiles is the main program function to process folder entries.
func processFiles(batch *batch, core *models.Core, openVideo, openMeta *os.File) ([]*models.FileData, error) {
	var skipVideos bool

	if abstractions.IsSet(keys.SkipVideos) {
		skipVideos = abstractions.GetBool(keys.SkipVideos)
	} else {
		skipVideos = batch.SkipVideos
	}

	// Match and video file maps, and meta file count
	if err := getFiles(batch, openMeta, openVideo, skipVideos); err != nil {
		return nil, err
	}

	logger.Pl.I("Found %d file(s) to process", batch.bp.counts.totalMatched)
	logger.Pl.D(3, "Matched metafiles: %d", batch.bp.counts.totalMatched)

	var (
		muProcessed sync.Mutex
		muFailed    sync.Mutex
	)

	ctx := core.Ctx
	wg := core.Wg

	processMetadataFiles(ctx, batch.bp, batch.bp.syncMapToRegularMap(&batch.bp.files.matched), &muFailed)
	setupCleanup(ctx, wg, batch, &muFailed)

	matchedCount := int(batch.bp.counts.totalMatched)
	processedModels := make([]*models.FileData, 0, matchedCount)
	numWorkers := max(abstractions.GetInt(keys.Concurrency), 1)

	jobs := make(chan workItem, min(matchedCount, numWorkers*2))
	results := make(chan *models.FileData, min(matchedCount, numWorkers*2))

	// Start workers
	for worker := 1; worker <= numWorkers; worker++ {
		wg.Add(1)
		go workerVideoProcess(ctx, wg, batch, worker, jobs, results)
	}

	// Collector routine to collect results from the results channel
	var collectorWg sync.WaitGroup
	collectorWg.Add(1)
	go func() {
		defer collectorWg.Done()

		for result := range results {
			if result != nil {
				muProcessed.Lock()
				processedModels = append(processedModels, result)
				muProcessed.Unlock()
			}
		}
	}()

	// Send jobs to workers
	for name, data := range batch.bp.syncMapToRegularMap(&batch.bp.files.matched) {
		jobs <- workItem{
			filename:     name,
			fileData:     data,
			metaFilename: batch.bp.filepaths.metaFile,
			skipVids:     skipVideos,
		}
	}
	close(jobs)
	wg.Wait()

	close(results)
	collectorWg.Wait()

	// Get errors
	errArray := vars.GetErrorArray()
	if errArray != nil {
		batch.bp.logFailedVideos()
	}
	return processedModels, nil
}

// workerVideoProcess performs the video processing operation for a worker.
func workerVideoProcess(ctx context.Context, wg *sync.WaitGroup, batch *batch, id int, jobs <-chan workItem, results chan<- *models.FileData) {
	defer wg.Done()
	defer func() {
		if r := recover(); r != nil {
			logger.Pl.E("Worker %d panicked: %v\n%s", id, r, debug.Stack())
		}
	}()

	// Execute video jobs
	for job := range jobs {
		skipVideos := job.skipVids
		filename := job.filename

		select {
		case <-ctx.Done():
			logger.Pl.I("Worker %d stopping due to context cancellation", id)
			return
		default:
			logger.Pl.D(1, "Worker %d processing file: %s", id, filename)

			executed, err := executeFile(ctx, batch.bp, skipVideos, filename, job.fileData)
			if err != nil {
				logger.Pl.E("Worker %d error executing file %q: %v", id, filename, err)
				continue
			}
			results <- executed
		}
	}
}

// processMetadataFiles processes metafiles such as .json, .nfo, and so on.
func processMetadataFiles(ctx context.Context, bp *batchProcessor, matchedFiles map[string]*models.FileData, muFailed *sync.Mutex) {
	for _, fd := range matchedFiles {
		var err error
		switch fd.MetaFileType {
		case consts.MExtJSON:
			logger.Pl.D(3, "File: %s: Meta file type in model as %v", fd.MetaFilePath, fd.MetaFileType)
			err = processJSONFile(ctx, fd)
		case consts.MExtNFO:
			logger.Pl.D(3, "File: %s: Meta file type in model as %v", fd.MetaFilePath, fd.MetaFileType)
			err = processNFOFiles(ctx, fd)
		}
		if err != nil {
			vars.AddToErrorArray(err)
			logger.Pl.E("Failed processing metadata for file %q: %v", fd.OriginalVideoPath, err)

			muFailed.Lock()
			bp.logFailedVideos()
			muFailed.Unlock()
		}
	}
}

// getFiles returns a map of matched video/metadata files.
func getFiles(batch *batch, openMeta, openVideo *os.File, skipVideos bool) (err error) {
	videoMap := make(map[string]*models.FileData)
	metaMap := make(map[string]*models.FileData)

	// Batch is a directory request...
	if batch.IsDirs {
		metaMap, err = file.GetMetadataFiles(openMeta)
		if err != nil {
			batch.bp.addFailure(failedVideo{
				filename: openMeta.Name(),
				err:      err.Error(),
			})
			return fmt.Errorf("failed to retrieve metadata files in %q: %w", openMeta.Name(), err)
		}

		if !skipVideos {
			videoMap, err = file.GetVideoFiles(openVideo)
			if err != nil {
				batch.bp.addFailure(failedVideo{
					filename: openVideo.Name(),
					err:      err.Error(),
				})
				return fmt.Errorf("failed to retrieve video files in %q: %w", openVideo.Name(), err)
			}
		}
		// Batch is a file request...
	} else if !batch.IsDirs {
		metaMap, err = file.GetSingleMetadataFile(openMeta)
		if err != nil {
			batch.bp.addFailure(failedVideo{
				filename: openMeta.Name(),
				err:      err.Error(),
			})
			return fmt.Errorf("failed to retrieve metadata file %q: %w", openMeta.Name(), err)
		}

		if !skipVideos {
			videoMap, err = file.GetSingleVideoFile(openVideo)
			if err != nil {
				batch.bp.addFailure(failedVideo{
					filename: openVideo.Name(),
					err:      err.Error(),
				})
				return fmt.Errorf("failed to retrieve video file %q: %w", openVideo.Name(), err)
			}
		}
	}

	// Match video and metadata files
	var matchedFiles map[string]*models.FileData // No need to assign length (just a placeholder var)
	if !skipVideos {
		matchedFiles, err = file.MatchVideoWithMetadata(videoMap, metaMap, batch.ID)
		if err != nil {
			return fmt.Errorf("error matching videos with metadata: %w", err)
		}
	} else {
		matchedFiles = metaMap
	}

	var (
		openMetaFilename,
		openVideoFilename,
		directory string
	)

	switch {
	case openMeta != nil && openMeta.Name() != "":
		directory = filepath.Dir(openMeta.Name())
	case openVideo != nil && openVideo.Name() != "":
		directory = filepath.Dir(openVideo.Name())
	}

	if openMeta != nil {
		openMetaFilename = openMeta.Name()
	}

	if openVideo != nil {
		openVideoFilename = openVideo.Name()
	}

	for k, v := range matchedFiles {
		batch.bp.files.matched.Store(k, v)
	}
	for k, v := range videoMap {
		batch.bp.files.video.Store(k, v)
	}

	atomic.StoreInt32(&batch.bp.counts.totalMeta, int32(len(metaMap)))
	atomic.StoreInt32(&batch.bp.counts.totalVideo, int32(len(videoMap)))
	atomic.StoreInt32(&batch.bp.counts.totalMatched, int32(len(matchedFiles)))

	batch.bp.filepaths.mu.Lock()
	batch.bp.filepaths.directory = directory
	batch.bp.filepaths.metaFile = openMetaFilename
	batch.bp.filepaths.videoFile = openVideoFilename
	batch.bp.filepaths.mu.Unlock()

	return nil
}

// executeFile handles processing for both video and metadata files.
func executeFile(ctx context.Context, bp *batchProcessor, skipVideos bool, filename string, fd *models.FileData) (*models.FileData, error) {
	// Check for context cancellation
	select {
	case <-ctx.Done():
		return nil, fmt.Errorf("did not process %q due to program cancellation", filename)
	default:
	}

	// Print progress for metadata
	currentMeta := atomic.AddInt32(&bp.counts.processedMeta, 1)
	totalMeta := atomic.LoadInt32(&bp.counts.totalMeta)
	printProgress(typeMeta, currentMeta, totalMeta, fd.MetaDirectory)

	// System resource check
	sysResourceLoop(ctx, filename)

	// Process file based on type
	isVideoFile := fd.OriginalVideoPath != ""

	if isVideoFile {
		logger.Pl.I("Processing file: %s", filename)
		if !skipVideos {
			if err := ffmpeg.ExecuteVideo(ctx, fd); err != nil {

				errMsg := fmt.Errorf("failed to process video '%v': %w", filename, err)
				vars.AddToErrorArray(errMsg)
				logger.Pl.E("Failed to execute video %q: %v", fd.OriginalVideoPath, err)

				bp.addFailure(failedVideo{
					filename: filename,
					err:      errMsg.Error(),
				})
				return nil, errMsg
			}
			fmt.Fprintf(os.Stderr, "\n")
			logger.Pl.S("Successfully processed video %s", filename)
		}
	} else {
		fmt.Fprintf(os.Stderr, "\n")
		logger.Pl.S("Successfully processed metadata for %s", filename)
	}

	// Print progress for video
	currentVideo := atomic.AddInt32(&bp.counts.processedVideo, 1)
	totalVideo := atomic.LoadInt32(&bp.counts.totalVideo)
	printProgress(typeVideo, currentVideo, totalVideo, fd.MetaDirectory)

	return fd, nil
}

// setupCleanup watches the context and safely cleans up batch resources on cancellation.
func setupCleanup(ctx context.Context, wg *sync.WaitGroup, batch *batch, muFailed *sync.Mutex) {
	go func() {
		// Wait for context finish or cancellation
		<-ctx.Done()
		logger.Pl.D(2, "Context ended, performing cleanup for batch %d", batch.bp.batchID)

		// Wait for workers to finish
		wg.Wait()

		// Log failed videos
		muFailed.Lock()
		batch.bp.logFailedVideos()
		muFailed.Unlock()

		// Release the batch processor back to the pool
		batch.bp.release()
	}()
}
