package processing

import (
	"context"
	"fmt"
	"metarr/internal/cfg"
	consts "metarr/internal/domain/constants"
	enums "metarr/internal/domain/enums"
	keys "metarr/internal/domain/keys"
	"metarr/internal/ffmpeg"
	reader "metarr/internal/metadata/reader"
	"metarr/internal/models"
	"metarr/internal/transformations"
	fsRead "metarr/internal/utils/fs/read"
	logging "metarr/internal/utils/logging"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
)

var (
	totalMetaFiles,
	totalVideoFiles,
	processedMetaFiles,
	processedVideoFiles int32

	failedVideos []failedVideo
	muPrint      sync.Mutex
)

type failedVideo struct {
	filename string
	err      string
}

// processFiles is the main program function to process folder entries
func ProcessFiles(batch *models.Batch, openVideo, openMeta *os.File) {

	cancel := batch.Core.Cancel
	cleanupChan := batch.Core.Cleanup
	ctx := batch.Core.Ctx
	wg := batch.Core.Wg

	// Reset counts
	atomic.StoreInt32(&totalMetaFiles, 0)
	atomic.StoreInt32(&totalVideoFiles, 0)
	atomic.StoreInt32(&processedMetaFiles, 0)
	atomic.StoreInt32(&processedVideoFiles, 0)

	var (
		videoMap,
		metaMap,
		matchedFiles map[string]*models.FileData

		processedDataArray []*models.FileData
		err                error
		skipVideos         bool
	)

	if cfg.IsSet(keys.SkipVideos) {
		skipVideos = cfg.GetBool(keys.SkipVideos)
	} else {
		skipVideos = batch.SkipVideos
	}

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
	if !skipVideos {
		matchedFiles, err = fsRead.MatchVideoWithMetadata(videoMap, metaMap)
		if err != nil {
			logging.E(0, "Error matching videos with metadata: %v", err)
			os.Exit(1)
		}
	} else {
		matchedFiles = metaMap
	}

	atomic.StoreInt32(&totalMetaFiles, int32(len(metaMap)))
	atomic.StoreInt32(&totalVideoFiles, int32(len(videoMap)))

	logging.I("Found %d file(s) to process in the directory", totalMetaFiles+totalVideoFiles)
	logging.D(3, "Matched metafiles: %v", matchedFiles)

	for _, fileData := range matchedFiles {
		var (
			processedData *models.FileData
			err           error
		)

		switch fileData.MetaFileType {
		case enums.METAFILE_JSON:
			logging.D(3, "File: %s: Meta file type in model as %v", fileData.JSONFilePath, fileData.MetaFileType)
			processedData, err = reader.ProcessJSONFile(fileData)

		case enums.METAFILE_NFO:
			logging.D(3, "File: %s: Meta file type in model as %v", fileData.NFOFilePath, fileData.MetaFileType)
			processedData, err = reader.ProcessNFOFiles(fileData)
		}
		if err != nil {
			logging.ErrorArray = append(logging.ErrorArray, err)
			errMsg := fmt.Errorf("error processing metadata for file '%s': %w", fileData.OriginalVideoPath, err)
			logging.E(0, errMsg.Error())

			failedVideos = append(failedVideos, failedVideo{
				filename: fileData.OriginalVideoPath,
				err:      errMsg.Error(),
			})
			continue
		}
		processedDataArray = append(processedDataArray, processedData)
	}

	// Goroutine to handle signals and cleanup
	go func() {
		<-cleanupChan

		fmt.Println("\nSignal received, cleaning up temporary files...")
		cancel()

		err = cleanupTempFiles(videoMap)
		if err != nil {
			logging.ErrorArray = append(logging.ErrorArray, err)
			fmt.Printf("\nFailed to cleanup temp files: %v", err)
			logging.E(0, "Failed to cleanup temp files", err)
		}
		logging.I("Process was interrupted by a syscall", nil)

		if len(failedVideos) > 0 {
			logging.P(consts.RedError + "Failed videos:")
			for _, failed := range failedVideos {
				fmt.Println()
				logging.P("Filename: %v", failed.filename)
				logging.P("Error: %v", failed.err)
			}
			fmt.Println()
		}

		wg.Wait()
		os.Exit(0)
	}()

	sem := make(chan struct{}, cfg.GetInt(keys.Concurrency))

	if !skipVideos {
		for fileName, fileData := range matchedFiles {
			executeFile(ctx, wg, sem, fileName, fileData)
		}
	}

	wg.Wait()

	err = cleanupTempFiles(videoMap)
	if err != nil {
		logging.ErrorArray = append(logging.ErrorArray, err)
		logging.E(0, "Failed to cleanup temp files: %v", err)
	}

	var (
		replaceToStyle enums.ReplaceToStyle
		ok             bool
	)

	if cfg.IsSet(keys.Rename) {
		if replaceToStyle, ok = cfg.Get(keys.Rename).(enums.ReplaceToStyle); !ok {
			logging.E(0, "Received wrong type for rename style. Got %T", replaceToStyle)
		} else {
			logging.D(2, "Got rename style as %T index %v", replaceToStyle, replaceToStyle)
		}
	}

	// var inputVideoDir string
	inputJsonDir, _ := filepath.Abs(openMeta.Name())
	inputJsonDir = strings.TrimSuffix(inputJsonDir, openMeta.Name())
	// if !skipVideos {
	// 	inputVideoDir, _ = filepath.Abs(openVideo.Name())
	// }

	err = transformations.FileRename(processedDataArray, replaceToStyle, skipVideos)
	if err != nil {
		logging.ErrorArray = append(logging.ErrorArray, err)
		logging.E(0, "Failed to rename files: %v", err)
	} else {
		logging.S(0, "Successfully formatted file names in directory: %v", inputJsonDir)
	}

	if len(logging.ErrorArray) == 0 || logging.ErrorArray == nil {
		logging.S(0, "Successfully processed all files in directory (%v) with no errors.", inputJsonDir)

		fmt.Println()
	} else {

		if logging.ErrorArray != nil {
			logging.E(0, "Program finished, but some errors were encountered: %v", logging.ErrorArray)

			if len(failedVideos) > 0 {
				logging.P(consts.RedError + "Failed videos:")
				for _, failed := range failedVideos {
					fmt.Println()
					logging.P("Filename: %v", failed.filename)
					logging.P("Error: %v", failed.err)
				}
			}
			fmt.Println()
		}
	}
}

// processFile handles processing for both video and metadata files
func executeFile(ctx context.Context, wg *sync.WaitGroup, sem chan struct{}, fileName string, fileData *models.FileData) {
	wg.Add(1)
	go func(fileName string, fileData *models.FileData) {

		defer wg.Done()

		currentFile := atomic.AddInt32(&processedMetaFiles, 1)
		total := atomic.LoadInt32(&totalMetaFiles)

		muPrint.Lock()
		fmt.Printf("\n==============================================================\n")
		fmt.Printf("    Processed metafile %d of %d\n", currentFile, total)
		fmt.Printf("    Remaining in '%s': %d\n", fileData.JSONDirectory, total-currentFile)
		fmt.Printf("==============================================================\n\n")
		muPrint.Unlock()

		sem <- struct{}{}
		defer func() {
			<-sem
		}()

		select {
		case <-ctx.Done():
			fmt.Printf("Skipping processing for %s due to cancellation\n", fileName)
			return
		default:
		}

		sysResourceLoop(fileName)

		skipVideos := cfg.GetBool(keys.SkipVideos)
		isVideoFile := fileData.OriginalVideoPath != ""

		if isVideoFile {
			logging.I("Processing file: %s", fileName)
		} else {
			logging.I("Processing metadata file: %s", fileName)
		}

		if isVideoFile && !skipVideos {
			err := ffmpeg.ExecuteVideo(fileData)
			if err != nil {
				logging.ErrorArray = append(logging.ErrorArray, err)
				errMsg := fmt.Errorf("failed to process video '%v': %w", fileName, err)
				logging.E(0, errMsg.Error())

				failedVideos = append(failedVideos, failedVideo{
					filename: fileName,
					err:      errMsg.Error(),
				})

			} else {
				logging.S(0, "Successfully processed video %s", fileName)
			}
		} else {
			logging.S(0, "Successfully processed metadata for %s", fileName)
		}

		currentFile = atomic.AddInt32(&processedVideoFiles, 1)
		total = atomic.LoadInt32(&totalVideoFiles)

		muPrint.Lock()
		fmt.Printf("\n==============================================================\n")
		fmt.Printf("    Processed video file %d of %d\n", currentFile, total)
		fmt.Printf("    Remaining in '%s': %d\n", fileData.JSONDirectory, total-currentFile)
		fmt.Printf("==============================================================\n\n")
		muPrint.Unlock()

	}(fileName, fileData)
}
