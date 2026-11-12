package processing

import (
	"context"
	"fmt"
	"metarr/internal/abstractions"
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

	"github.com/TubarrApp/gocommon/sharedconsts"
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

	// Match and video file maps, and meta file count.
	if err := getFiles(batch, openMeta, openVideo, skipVideos); err != nil {
		return nil, err
	}

	matchedCount := int(batch.bp.counts.totalMatched.Load())
	logger.Pl.I("Found %d file(s) to process", matchedCount)
	logger.Pl.D(3, "Matched metafiles: %d", matchedCount)

	var (
		muProcessed sync.Mutex
		muFailed    sync.Mutex
	)

	ctx := core.Ctx
	wg := core.Wg

	processMetadataFiles(ctx, batch.bp, skipVideos, batch.bp.syncMapToRegularMap(&batch.bp.files.matched), &muFailed)

	processedModels := make([]*models.FileData, 0, matchedCount)
	numWorkers := max(abstractions.GetInt(keys.Concurrency), 1)

	jobs := make(chan workItem, min(matchedCount, numWorkers*2))
	results := make(chan *models.FileData, min(matchedCount, numWorkers*2))

	// Start workers.
	for worker := 1; worker <= numWorkers; worker++ {
		wg.Add(1)
		go workerVideoProcess(ctx, wg, batch, worker, jobs, results)
	}

	// Collector routine to collect results from the results channel.
	var collectorWg sync.WaitGroup
	collectorWg.Go(func() {
		for result := range results {
			if result != nil {
				muProcessed.Lock()
				processedModels = append(processedModels, result)
				muProcessed.Unlock()
			}
		}
	})

	// Send jobs to workers, stopping on cancellation.
sendJobs:
	for name, data := range batch.bp.syncMapToRegularMap(&batch.bp.files.matched) {
		job := workItem{
			filename:     name,
			fileData:     data,
			metaFilename: batch.bp.filepaths.metaFile,
			skipVids:     skipVideos,
		}
		select {
		case jobs <- job:
		case <-ctx.Done():
			break sendJobs
		}
	}
	close(jobs)
	wg.Wait()

	close(results)
	collectorWg.Wait()

	// Get errors.
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

	// Execute video jobs.
	for job := range jobs {
		skipVideos := job.skipVids
		filename := job.filename

		select {
		case <-ctx.Done():
			logger.Pl.I("Worker %d stopping due to context cancellation", id)
			return
		default:
			logger.Pl.D(2, "Worker %d processing file: %s", id, filename)

			executed, err := executeFile(ctx, batch.bp, skipVideos, filename, job.fileData)
			if err != nil {
				logger.Pl.E("Worker %d error executing file %q: %v", id, filename, err)
				continue
			}
			results <- executed
		}
	}
}

// processMetadataFiles processes metafiles such as .json and .nfo, and checks whether a video with none needs FFmpeg.
func processMetadataFiles(ctx context.Context, bp *batchProcessor, skipVideos bool, matchedFiles map[string]*models.FileData, muFailed *sync.Mutex) {
	for name, fd := range matchedFiles {
		var err error
		switch fd.MetaFileType {
		case "": // No metafile, so no tags to write, but the thumbnail settings still apply.
			if !skipVideos && filetypeMetaCheckSwitch(ctx, fd) {
				fd.MetaAlreadyExists = true
			}
		case sharedconsts.MExtJSON:
			logger.Pl.D(3, "File: %s: Meta file type in model as %v", fd.MetaFilePath, fd.MetaFileType)
			err = processJSONFile(ctx, fd, skipVideos)
		case sharedconsts.MExtNFO:
			logger.Pl.D(3, "File: %s: Meta file type in model as %v", fd.MetaFilePath, fd.MetaFileType)
			err = processNFOFiles(ctx, fd, skipVideos)
		}
		if err != nil {
			vars.AddToErrorArray(err)
			logger.Pl.E("Failed processing metadata for file %q: %v", fd.OriginalVideoPath, err)

			// Without its metadata, the file goes no further.
			bp.files.matched.Delete(name)

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
		if openMeta != nil {
			metaMap, err = file.GetMetadataFiles(openMeta)
			if err != nil {
				batch.bp.addFailure(failedVideo{
					filename: openMeta.Name(),
					err:      err.Error(),
				})
				return fmt.Errorf("failed to retrieve metadata files in %q: %w", openMeta.Name(), err)
			}
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
		if openMeta != nil {
			metaMap, err = file.GetSingleMetadataFile(openMeta)
			if err != nil {
				batch.bp.addFailure(failedVideo{
					filename: openMeta.Name(),
					err:      err.Error(),
				})
				return fmt.Errorf("failed to retrieve metadata file %q: %w", openMeta.Name(), err)
			}
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

	// Match video and metadata files, or take the videos alone if the batch has no metadata input.
	var matchedFiles map[string]*models.FileData // Do not assign length (just a placeholder var, will show unused otherwise).
	switch {
	case skipVideos:
		matchedFiles = metaMap
	case openMeta == nil:
		matchedFiles = videoMap
	default:
		matchedFiles, err = file.MatchVideoWithMetadata(videoMap, metaMap, batch.ID)
		if err != nil {
			return fmt.Errorf("error matching videos with metadata: %w", err)
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

	batch.bp.counts.totalMeta.Store(int32(len(metaMap)))
	batch.bp.counts.totalVideo.Store(int32(len(videoMap)))
	batch.bp.counts.totalMatched.Store(int32(len(matchedFiles)))

	batch.bp.filepaths.mu.Lock()
	batch.bp.filepaths.directory = directory
	batch.bp.filepaths.metaFile = openMetaFilename
	batch.bp.filepaths.videoFile = openVideoFilename
	batch.bp.filepaths.mu.Unlock()

	return nil
}

// executeFile handles processing for both video and metadata files.
func executeFile(ctx context.Context, bp *batchProcessor, skipVideos bool, filename string, fd *models.FileData) (*models.FileData, error) {
	// Check for context cancellation.
	select {
	case <-ctx.Done():
		return nil, fmt.Errorf("did not process %q due to program cancellation", filename)
	default:
	}

	// Print progress for metadata, if the file has a metafile.
	if fd.MetaFilePath != "" {
		currentMeta := bp.counts.processedMeta.Add(1)
		totalMeta := bp.counts.totalMeta.Load()
		printProgress(typeMeta, currentMeta, totalMeta, fd.MetaDirectory)
	}

	// System resource check.
	sysResourceLoop(ctx, filename)

	// Process file based on type.
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

			currentVideo := bp.counts.processedVideo.Add(1)
			totalVideo := bp.counts.totalVideo.Load()
			printProgress(typeVideo, currentVideo, totalVideo, fd.VideoDirectory)
		}
	} else {
		fmt.Fprintf(os.Stderr, "\n")
		logger.Pl.S("Successfully processed metadata for %s", filename)
	}

	return fd, nil
}
