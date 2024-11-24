package processing

import (
	"context"
	"fmt"
	"metarr/internal/cfg"
	"metarr/internal/domain/enums"
	"metarr/internal/domain/keys"
	"metarr/internal/ffmpeg"
	metaReader "metarr/internal/metadata/reader"
	"metarr/internal/models"
	fsRead "metarr/internal/utils/fs/read"
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

var (
	totalMetaFiles,
	totalVideoFiles,
	processedMetaFiles,
	processedVideoFiles int32

	failedVideos []failedVideo
)

type failedVideo struct {
	filename string
	err      string
}

type getFilesOutput struct {
	matched,
	videoMap map[string]*models.FileData
	metaMapLen int
	openVideoFilename,
	openMetaFilename,
	directory string
}

type workItem struct {
	filename     string
	fileData     *models.FileData
	metaFilename string
	skipVids     bool
}

// processFiles is the main program function to process folder entries
func ProcessFiles(batch models.Batch, core *models.Core, openVideo, openMeta *os.File) {

	// Reset counts and get skip video bool
	skipVideos := prepNewBatch(batch.SkipVideos)

	// Match and video file maps, and meta file count
	files := getFiles(batch, openMeta, openVideo, skipVideos)
	atomic.StoreInt32(&totalMetaFiles, int32(files.metaMapLen))
	atomic.StoreInt32(&totalVideoFiles, int32(len(files.videoMap)))

	totalFiles := totalMetaFiles + totalVideoFiles
	logging.I(fmt.Sprintf("Found %d file(s) to process", totalFiles))
	logging.D(3, "Matched metafiles: %v", files.matched)

	var (
		muProcessed sync.Mutex
		muFailed    sync.Mutex
	)

	cancel := core.Cancel
	cleanupChan := core.Cleanup
	ctx := core.Ctx
	wg := core.Wg

	if err := processMetadataFiles(ctx, files.matched, &muFailed); err != nil {
		logging.E(0, "Error processing metadata files: %v", err)
	}

	setupCleanup(cleanupChan, cancel, wg, files.videoMap, &muFailed)

	matchedCount := len(files.matched)
	processedModels := make([]*models.FileData, 0, matchedCount)
	jobs := make(chan workItem, matchedCount)
	results := make(chan *models.FileData, matchedCount)

	numWorkers := cfg.GetInt(keys.Concurrency)
	if numWorkers < 1 {
		numWorkers = 1
	}

	// Start workers
	for w := 1; w <= numWorkers; w++ {
		wg.Add(1)
		go workerProcess(w, jobs, results, wg, ctx)
	}

	// Send jobs to workers
	for name, data := range files.matched {
		jobs <- workItem{
			filename:     name,
			fileData:     data,
			metaFilename: files.openMetaFilename,
			skipVids:     skipVideos,
		}
	}
	close(jobs)

	// Collect results in a separate goroutine
	go func() {
		for result := range results {
			if result != nil {
				muProcessed.Lock()
				processedModels = append(processedModels, result)
				muProcessed.Unlock()
			}
		}
	}()

	wg.Wait()
	close(results)

	// Handle temp files and cleanup
	err := cleanupTempFiles(files.videoMap)
	if err != nil {
		logging.ErrorArray = append(logging.ErrorArray, err)
		logging.E(0, "Failed to cleanup temp files: %v", err)
	}

	if len(logging.ErrorArray) == 0 || logging.ErrorArray == nil {
		logging.S(0, "Successfully processed all files in directory %q with no errors.", filepath.Dir(files.openMetaFilename))
		fmt.Println()
		return
	}

	if logging.ErrorArray != nil {
		logFailedVideos()
	}
}

// workerProcess performs the video processing operation for a worker.
func workerProcess(id int, jobs <-chan workItem, results chan<- *models.FileData, wg *sync.WaitGroup, ctx context.Context) {
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

			executed, err := executeFile(ctx, skipVideos, filename, job.fileData)
			if err != nil {
				logging.E(0, fmt.Sprintf("Worker %d error executing file %q: %v", id, filename, err))
				continue
			}

			renameFiles(job.filename, job.metaFilename, executed, job.skipVids)

			results <- executed
		}
	}
}

// processMetadataFiles processes metafiles such as .json, .nfo, and so on.
func processMetadataFiles(ctx context.Context, matchedFiles map[string]*models.FileData, muFailed *sync.Mutex) error {
	for _, fd := range matchedFiles {
		var err error
		switch fd.MetaFileType {
		case enums.METAFILE_JSON:
			logging.D(3, "File: %s: Meta file type in model as %v", fd.JSONFilePath, fd.MetaFileType)
			_, err = metaReader.ProcessJSONFile(ctx, fd)
		case enums.METAFILE_NFO:
			logging.D(3, "File: %s: Meta file type in model as %v", fd.NFOFilePath, fd.MetaFileType)
			_, err = metaReader.ProcessNFOFiles(fd)
		}

		if err != nil {
			logging.ErrorArray = append(logging.ErrorArray, err)
			errMsg := fmt.Errorf("error processing metadata for file %q: %w", fd.OriginalVideoPath, err)
			logging.E(0, errMsg.Error())

			muFailed.Lock()
			logFailedVideos()
			muFailed.Unlock()
		}
	}
	return nil
}

// getFiles returns a map of matched video/metadata files.
func getFiles(batch models.Batch, openMeta, openVideo *os.File, skipVideos bool) getFilesOutput {
	var (
		videoMap,
		metaMap map[string]*models.FileData

		err error
	)

	// Batch is a directory request...
	if batch.IsDirs {
		metaMap, err = fsRead.GetMetadataFiles(openMeta)
		if err != nil {
			logging.E(0, err.Error())
			failedVideos = append(failedVideos, failedVideo{
				filename: openMeta.Name(),
				err:      err.Error(),
			})
		}

		if !skipVideos {
			videoMap, err = fsRead.GetVideoFiles(openVideo)
			if err != nil {
				failedVideos = append(failedVideos, failedVideo{
					filename: openVideo.Name(),
					err:      err.Error(),
				})
			}
		}
	}

	// Batch is a file request...
	if !batch.IsDirs {
		metaMap, err = fsRead.GetSingleMetadataFile(openMeta)
		if err != nil {
			logging.E(0, err.Error())
			failedVideos = append(failedVideos, failedVideo{
				filename: openMeta.Name(),
				err:      err.Error(),
			})
		}

		if !skipVideos {
			videoMap, err = fsRead.GetSingleVideoFile(openVideo)
			if err != nil {
				failedVideos = append(failedVideos, failedVideo{
					filename: openVideo.Name(),
					err:      err.Error(),
				})
			}
		}
	}

	// Match video and metadata files
	var matchedFiles map[string]*models.FileData // No need to assign length (just a placeholder var)
	if !skipVideos {
		matchedFiles, err = fsRead.MatchVideoWithMetadata(videoMap, metaMap, batch.IsDirs)
		if err != nil {
			logging.E(0, "Error matching videos with metadata: %v", err)
			os.Exit(1)
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

	var rtn = getFilesOutput{
		matched:           matchedFiles,
		videoMap:          videoMap,
		metaMapLen:        len(metaMap),
		openVideoFilename: openVideoFilename,
		openMetaFilename:  openMetaFilename,
		directory:         directory,
	}
	return rtn
}

// processFile handles processing for both video and metadata files
func executeFile(ctx context.Context, skipVideos bool, filename string, fd *models.FileData) (*models.FileData, error) {

	// Check for context cancellation
	select {
	case <-ctx.Done():
		return nil, fmt.Errorf("did not process %q due to program cancellation", filename)
	default:
	}

	var muPrint sync.Mutex

	// Print progress for metadata
	currentMeta := atomic.AddInt32(&processedMetaFiles, 1)
	totalMeta := atomic.LoadInt32(&totalMetaFiles)
	printProgress(typeMeta, currentMeta, totalMeta, fd.JSONDirectory, &muPrint)

	// System resource check
	sysResourceLoop(filename)

	// Process file based on type
	isVideoFile := fd.OriginalVideoPath != ""

	if isVideoFile {
		logging.I("Processing file: %s", filename)
		if !skipVideos {
			if err := ffmpeg.ExecuteVideo(ctx, fd); err != nil {
				errMsg := fmt.Errorf("failed to process video '%v': %w", filename, err)
				logging.ErrorArray = append(logging.ErrorArray, errMsg)
				logging.E(0, errMsg.Error())

				failedVideos = append(failedVideos, failedVideo{
					filename: filename,
					err:      errMsg.Error(),
				})
				return nil, errMsg
			}
			logging.S(0, "Successfully processed video %s", filename)
		}
	} else {
		logging.I("Processing metadata file: %s", filename)
		logging.S(0, "Successfully processed metadata for %s", filename)
	}

	// Print progress for video
	currentVideo := atomic.AddInt32(&processedVideoFiles, 1)
	totalVideo := atomic.LoadInt32(&totalVideoFiles)
	printProgress(typeVideo, currentVideo, totalVideo, fd.JSONDirectory, &muPrint)

	return fd, nil
}

// setupCleanup creates a cleanup routine for file processing.
func setupCleanup(cleanupChan chan os.Signal, cancel context.CancelFunc, wg *sync.WaitGroup, videoMap map[string]*models.FileData, muFailed *sync.Mutex) {
	go func() {
		<-cleanupChan
		fmt.Println("\nSignal received, cleaning up temporary files...")
		cancel()
		wg.Wait()

		if err := cleanupTempFiles(videoMap); err != nil {
			logging.E(0, "Failed to cleanup temp files: %v", err)
		}

		muFailed.Lock()
		logFailedVideos()
		muFailed.Unlock()

		os.Exit(0)
	}()
}
