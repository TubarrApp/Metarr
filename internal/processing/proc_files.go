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

// processFiles is the main program function to process folder entries
func ProcessFiles(batch models.Batch, core *models.Core, openVideo, openMeta *os.File) {

	cancel := core.Cancel
	cleanupChan := core.Cleanup
	ctx := core.Ctx
	wg := core.Wg

	// Reset counts and get skip video bool
	skipVideos := prepNewBatch(batch.SkipVideos)

	// Match and video file maps, and meta file count
	matchedFiles, videoMap, metaCount := getFiles(batch, openMeta, openVideo, skipVideos)

	atomic.StoreInt32(&totalMetaFiles, int32(metaCount))
	atomic.StoreInt32(&totalVideoFiles, int32(len(videoMap)))

	logging.I("Found %d file(s) to process", totalMetaFiles+totalVideoFiles)
	logging.D(3, "Matched metafiles: %v", matchedFiles)

	var (
		muProcessed sync.Mutex
		muFailed    sync.Mutex
	)

	processMetadataFiles(ctx, matchedFiles, &muFailed)

	sem := make(chan struct{}, cfg.GetInt(keys.Concurrency))
	processedModels := make([]*models.FileData, 0, len(matchedFiles))

	setupCleanup(cleanupChan, cancel, wg, videoMap, &muFailed)

	if !skipVideos {
		for name, data := range matchedFiles {
			wg.Add(1)
			go func(filename string, fileData *models.FileData) {
				defer wg.Done()

				sem <- struct{}{}
				defer func() { <-sem }()

				rtn, err := executeFile(ctx, filename, fileData)
				if err != nil {
					logging.E(0, "error executing file %q: %v", filename, err)
					return
				}

				muProcessed.Lock()
				processedModels = append(processedModels, rtn)
				muProcessed.Unlock()

			}(name, data)
		}
		wg.Wait()
	}

	err := cleanupTempFiles(videoMap)
	if err != nil {
		logging.ErrorArray = append(logging.ErrorArray, err)
		logging.E(0, "Failed to cleanup temp files: %v", err)
	}

	directory := renameFiles(openVideo.Name(), openMeta.Name(), processedModels, skipVideos)

	if len(logging.ErrorArray) == 0 || logging.ErrorArray == nil {
		logging.S(0, "Successfully processed all files in directory %q with no errors.", directory)
		fmt.Println()
		return
	}

	if logging.ErrorArray != nil {
		logFailedVideos()
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
			failedVideos = append(failedVideos, failedVideo{
				filename: fd.OriginalVideoPath,
				err:      errMsg.Error(),
			})
			muFailed.Unlock()
		}
	}
	return nil
}

// getFiles returns a map of matched video/metadata files.
func getFiles(batch models.Batch, openMeta, openVideo *os.File, skipVideos bool) (matched, videos map[string]*models.FileData, metaCount int) {
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

	var matchedFiles map[string]*models.FileData
	// Match video and metadata files
	if !skipVideos {
		matchedFiles, err = fsRead.MatchVideoWithMetadata(videoMap, metaMap, batch.IsDirs)
		if err != nil {
			logging.E(0, "Error matching videos with metadata: %v", err)
			os.Exit(1)
		}
	} else {
		matchedFiles = metaMap
	}
	return matchedFiles, videoMap, len(metaMap)
}

// processFile handles processing for both video and metadata files
func executeFile(ctx context.Context, filename string, fd *models.FileData) (*models.FileData, error) {

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
	skipVideos := cfg.GetBool(keys.SkipVideos)
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
