package processing

import (
	"Metarr/internal/cmd"
	"Metarr/internal/enums"
	"Metarr/internal/keys"
	"Metarr/internal/logging"
	"Metarr/internal/metadata"
	"Metarr/internal/models"
	"Metarr/internal/naming"
	"context"
	"fmt"
	"os"
	"sync"
	"sync/atomic"
)

var totalMetaFiles int32
var totalVideoFiles int32
var processedMetaFiles int32
var processedVideoFiles int32
var processedDataArray []*models.FileData

// processFiles is the main program function to process folder entries
func ProcessFiles(ctx context.Context, cancel context.CancelFunc, wg *sync.WaitGroup, cleanupChan chan os.Signal, openVideoDir, openJsonDir *os.File) {

	skipVideos := cmd.GetBool(keys.SkipVideos)

	var videoMap, metaMap, matchedFiles map[string]*models.FileData
	var err error

	metaMap, err = metadata.GetMetadataFiles(openJsonDir)
	if err != nil {
		logging.PrintE(0, "Error: %v", err)
		os.Exit(1)
	}

	if !skipVideos {
		videoMap, err = metadata.GetVideoFiles(openVideoDir)
		if err != nil {
			logging.PrintE(0, "Error fetching video files: %v", err)
			os.Exit(1)
		}

		matchedFiles, err = metadata.MatchVideoWithMetadata(videoMap, metaMap)
		if err != nil {
			logging.PrintE(0, "Error matching videos with metadata: %v", err)
			os.Exit(1)
		}
	} else {
		matchedFiles = metaMap
	}

	cmd.Set(keys.VideoMap, videoMap)
	cmd.Set(keys.MetaMap, metaMap)

	atomic.StoreInt32(&totalMetaFiles, int32(len(metaMap)))
	atomic.StoreInt32(&totalVideoFiles, int32(len(videoMap)))

	fmt.Printf("\nFound %d file(s) to process in the directory\n", totalMetaFiles+totalVideoFiles)

	for _, fileData := range matchedFiles {
		processedData, err := metadata.ProcessJSONFile(fileData)
		if err != nil {
			logging.ErrorArray = append(logging.ErrorArray, err)
			errMsg := fmt.Errorf("error processing JSON for file: %w", err)
			logging.PrintE(0, errMsg.Error())
			return
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
			logging.PrintE(0, "Failed to cleanup temp files", err)
		}

		logging.PrintI("Process was interrupted by a syscall", nil)

		wg.Wait()
		os.Exit(0)
	}()

	sem := make(chan struct{}, cmd.GetInt(keys.Concurrency))

	for fileName, fileData := range matchedFiles {

		executeFile(ctx, wg, sem, fileName, fileData)
	}

	wg.Wait()

	err = cleanupTempFiles(videoMap)
	if err != nil {
		logging.ErrorArray = append(logging.ErrorArray, err)
		logging.PrintE(0, "Failed to cleanup temp files: %v", err)
	}

	replaceToStyle := cmd.Get(keys.Rename).(enums.ReplaceToStyle)
	inputVideoDir := cmd.GetString(keys.JsonDir)

	err = naming.FileRename(processedDataArray, replaceToStyle)
	if err != nil {
		logging.ErrorArray = append(logging.ErrorArray, err)
		logging.PrintE(0, "Failed to rename files: %v", err)
	} else {
		logging.PrintS(0, "Successfully formatted file names in directory: %v", inputVideoDir)
	}

	if len(logging.ErrorArray) == 0 || logging.ErrorArray == nil {

		logging.PrintS(0, "Successfully processed all videos in directory (%v) with no errors.", inputVideoDir)
		fmt.Println()
	} else {

		logging.PrintE(0, "Program finished, but some errors were encountered: %v", logging.ErrorArray)
		fmt.Println()
	}
}

// processFile handles processing for both video and metadata files
func executeFile(ctx context.Context, wg *sync.WaitGroup, sem chan struct{}, fileName string, fileData *models.FileData) {
	wg.Add(1)
	go func(fileName string, fileData *models.FileData) {

		defer wg.Done()

		currentFile := atomic.AddInt32(&processedMetaFiles, 1)
		total := atomic.LoadInt32(&totalMetaFiles)

		fmt.Printf("\n====================================================\n")
		fmt.Printf("    Processed metafile %d of %d\n", currentFile, total)
		fmt.Printf("    Remaining: %d\n", total-currentFile)
		fmt.Printf("====================================================\n\n")

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

		skipVideos := cmd.GetBool(keys.SkipVideos)
		isVideoFile := fileData.OriginalVideoPath != ""

		if isVideoFile {
			logging.PrintI("Processing file: %s", fileName)
		} else {
			logging.PrintI("Processing metadata file: %s", fileName)
		}

		if isVideoFile && !skipVideos {
			err := metadata.WriteMetadata(fileData)
			if err != nil {
				logging.ErrorArray = append(logging.ErrorArray, err)
				errMsg := fmt.Errorf("failed to process video '%v': %w", fileName, err)
				logging.PrintE(0, errMsg.Error())
			} else {
				logging.PrintS(0, "Successfully processed video %s\n", fileName)
			}
		} else {
			logging.PrintS(0, "Successfully processed metadata for %s\n", fileName)
		}

		currentFile = atomic.AddInt32(&processedVideoFiles, 1)
		total = atomic.LoadInt32(&totalVideoFiles)

		fmt.Printf("\n====================================================\n")
		fmt.Printf("    Processed video file %d of %d\n", currentFile, total)
		fmt.Printf("    Remaining: %d\n", total-currentFile)
		fmt.Printf("====================================================\n\n")

	}(fileName, fileData)
}
