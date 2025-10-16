package processing

import (
	"context"
	"fmt"
	"metarr/internal/cfg"
	"metarr/internal/domain/enums"
	"metarr/internal/domain/keys"
	"metarr/internal/ffmpeg"
	"metarr/internal/metadata/procmeta"
	"metarr/internal/models"
	"metarr/internal/transformations"
	"metarr/internal/utils/fs/fsread"
	"metarr/internal/utils/logging"
	"os"
	"path/filepath"
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
func processFiles(batch *batch, core *models.Core, openVideo, openMeta *os.File) error {
	var (
		skipVideos bool
		err        error
	)

	if cfg.IsSet(keys.SkipVideos) {
		skipVideos = cfg.GetBool(keys.SkipVideos)
	} else {
		skipVideos = batch.SkipVideos
	}

	// Match and video file maps, and meta file count
	if err = getFiles(batch, openMeta, openVideo, skipVideos); err != nil {
		return err
	}

	logging.I("Found %d file(s) to process", batch.bp.counts.totalMatched)
	logging.D(3, "Matched metafiles: %d", batch.bp.counts.totalMatched)

	var (
		muProcessed sync.Mutex
		muFailed    sync.Mutex
	)

	cancel := core.Cancel
	cleanupChan := core.Cleanup
	ctx := core.Ctx
	wg := core.Wg

	if err := processMetadataFiles(batch.bp, ctx, batch.bp.syncMapToRegularMap(&batch.bp.files.matched), &muFailed); err != nil {
		logging.E("Error processing metadata files: %v", err)
	}

	setupCleanup(batch, ctx, cancel, cleanupChan, wg, batch.bp.syncMapToRegularMap(&batch.bp.files.video), &muFailed)

	matchedCount := int(batch.bp.counts.totalMatched)
	processedModels := make([]*models.FileData, 0, matchedCount)

	numWorkers := cfg.GetInt(keys.Concurrency)
	if numWorkers < 1 {
		numWorkers = 1
	}

	jobs := make(chan workItem, min(matchedCount, numWorkers*2))
	results := make(chan *models.FileData, min(matchedCount, numWorkers*2))

	// Start workers
	for w := 1; w <= numWorkers; w++ {
		wg.Add(1)
		go workerVideoProcess(batch, w, jobs, results, wg, ctx)
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

	// Handle temp files and cleanup
	if err = cleanupTempFiles(batch.bp.syncMapToRegularMap(&batch.bp.files.video)); err != nil {
		logging.AddToErrorArray(err)
		logging.E("Failed to cleanup temp files: %v", err)
	}

	errArray := logging.GetErrorArray()
	if errArray != nil {
		batch.bp.logFailedVideos()
	}

	return nil
}

// workerVideoProcess performs the video processing operation for a worker.
func workerVideoProcess(batch *batch, id int, jobs <-chan workItem, results chan<- *models.FileData, wg *sync.WaitGroup, ctx context.Context) {
	defer wg.Done()

	for job := range jobs {

		skipVideos := job.skipVids
		filename := job.filename

		select {
		case <-ctx.Done():
			logging.I("Worker %d stopping due to context cancellation", id)
			return
		default:
			logging.D(1, "Worker %d processing file: %s", id, filename)

			executed, err := executeFile(batch.bp, ctx, skipVideos, filename, job.fileData)
			if err != nil {
				logging.E("Worker %d error executing file %q: %v", id, filename, err)
				continue
			}

			renameFiles(job.filename, job.metaFilename, batch.ID, executed, job.skipVids)

			results <- executed
		}
	}
}

// processMetadataFiles processes metafiles such as .json, .nfo, and so on.
func processMetadataFiles(bp *batchProcessor, ctx context.Context, matchedFiles map[string]*models.FileData, muFailed *sync.Mutex) error {
	for _, fd := range matchedFiles {
		var err error
		switch fd.MetaFileType {
		case enums.MetaFiletypeJSON:
			logging.D(3, "File: %s: Meta file type in model as %v", fd.JSONFilePath, fd.MetaFileType)
			_, err = procmeta.ProcessJSONFile(ctx, fd)
		case enums.MetaFiletypeNFO:
			logging.D(3, "File: %s: Meta file type in model as %v", fd.NFOFilePath, fd.MetaFileType)
			_, err = procmeta.ProcessNFOFiles(fd)
		}

		if err != nil {
			logging.AddToErrorArray(err)
			logging.E("Failed processing metadata for file %q: %v", fd.OriginalVideoPath, err)

			muFailed.Lock()
			bp.logFailedVideos()
			muFailed.Unlock()
		}
	}
	return nil
}

// getFiles returns a map of matched video/metadata files.
func getFiles(batch *batch, openMeta, openVideo *os.File, skipVideos bool) error {
	var (
		videoMap,
		metaMap map[string]*models.FileData
		err error
	)

	// Batch is a directory request...
	if batch.IsDirs {
		metaMap, err = fsread.GetMetadataFiles(openMeta)
		if err != nil {
			logging.E("Failed to retrieve metadata files in %q: %v", openMeta.Name(), err)
			batch.bp.addFailure(failedVideo{
				filename: openMeta.Name(),
				err:      err.Error(),
			})
		}

		if !skipVideos {
			videoMap, err = fsread.GetVideoFiles(openVideo)
			if err != nil {
				logging.E("Failed to retrieve video files in %q: %v", openVideo.Name(), err)
				batch.bp.addFailure(failedVideo{
					filename: openVideo.Name(),
					err:      err.Error(),
				})
			}
		}
		// Batch is a file request...
	} else if !batch.IsDirs {
		metaMap, err = fsread.GetSingleMetadataFile(openMeta)
		if err != nil {
			logging.E("Failed to retrieve metadata file %q: %v", openMeta.Name(), err)
			batch.bp.addFailure(failedVideo{
				filename: openMeta.Name(),
				err:      err.Error(),
			})
		}

		if !skipVideos {
			videoMap, err = fsread.GetSingleVideoFile(openVideo)
			if err != nil {
				logging.E("Failed to retrieve video file %q: %v", openVideo.Name(), err)
				batch.bp.addFailure(failedVideo{
					filename: openVideo.Name(),
					err:      err.Error(),
				})
			}
		}
	}

	// Match video and metadata files
	var matchedFiles map[string]*models.FileData // No need to assign length (just a placeholder var)
	if !skipVideos {
		matchedFiles, err = fsread.MatchVideoWithMetadata(videoMap, metaMap, batch.ID)
		if err != nil {
			return fmt.Errorf("error matching videos with metadata: %w", err)
		}
	} else {
		matchedFiles = metaMap
	}

	// Strip existing date tag
	if cfg.GetBool(keys.DeleteDateTagPfx) {
		logging.I("Stripping date tags from files...")
		err := transformations.StripDateTagFromFilename(matchedFiles, videoMap, metaMap)
		if err != nil {
			logging.E("Failed to strip date tags: %v", err)
		}
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
func executeFile(bp *batchProcessor, ctx context.Context, skipVideos bool, filename string, fd *models.FileData) (*models.FileData, error) {

	// Check for context cancellation
	select {
	case <-ctx.Done():
		return nil, fmt.Errorf("did not process %q due to program cancellation", filename)
	default:
	}

	// Print progress for metadata
	currentMeta := atomic.AddInt32(&bp.counts.processedMeta, 1)
	totalMeta := atomic.LoadInt32(&bp.counts.totalMeta)
	printProgress(typeMeta, currentMeta, totalMeta, fd.JSONDirectory)

	// System resource check
	sysResourceLoop(filename)

	// Process file based on type
	isVideoFile := fd.OriginalVideoPath != ""

	if isVideoFile {
		logging.I("Processing file: %s", filename)
		if !skipVideos {
			if err := ffmpeg.ExecuteVideo(ctx, fd); err != nil {

				errMsg := fmt.Errorf("failed to process video '%v': %w", filename, err)
				logging.AddToErrorArray(errMsg)
				logging.E("Failed to execute video %q: %v", fd.OriginalVideoPath, err)

				bp.addFailure(failedVideo{
					filename: filename,
					err:      errMsg.Error(),
				})
				return nil, errMsg
			}
			fmt.Println()
			logging.S("Successfully processed video %s", filename)
		}
	} else {
		fmt.Println()
		logging.S("Successfully processed metadata for %s", filename)
	}

	// Print progress for video
	currentVideo := atomic.AddInt32(&bp.counts.processedVideo, 1)
	totalVideo := atomic.LoadInt32(&bp.counts.totalVideo)
	printProgress(typeVideo, currentVideo, totalVideo, fd.JSONDirectory)

	return fd, nil
}

// setupCleanup creates a cleanup routine for file processing.
func setupCleanup(batch *batch, ctx context.Context, cancel context.CancelFunc, cleanupChan chan os.Signal, wg *sync.WaitGroup, videoMap map[string]*models.FileData, muFailed *sync.Mutex) {
	go func() {
		select {
		case <-cleanupChan:
			fmt.Println("\nSignal received, cleaning up temporary files...")
			cancel()
		case <-ctx.Done():
			fmt.Println("\nContext cancelled, cleaning up temporary files...")
		}

		wg.Wait()

		if err := cleanupTempFiles(videoMap); err != nil {
			logging.E("Failed to cleanup temp files: %v", err)
		}

		muFailed.Lock()
		batch.bp.logFailedVideos()
		muFailed.Unlock()

		os.Exit(0)
	}()
}
