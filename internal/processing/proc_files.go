package processing

import (
	"Metarr/internal/config"
	enums "Metarr/internal/domain/enums"
	keys "Metarr/internal/domain/keys"
	reader "Metarr/internal/metadata/reader"
	writer "Metarr/internal/metadata/writer"
	"Metarr/internal/models"
	"Metarr/internal/transformations"
	fsRead "Metarr/internal/utils/fs/read"
	logging "Metarr/internal/utils/logging"
	"context"
	"fmt"
	"os"
	"sync"
	"sync/atomic"
)

var (
	totalMetaFiles,
	totalVideoFiles,
	processedMetaFiles,
	processedVideoFiles int32

	processedDataArray []*models.FileData
)

// processFiles is the main program function to process folder entries
func ProcessFiles(ctx context.Context, cancel context.CancelFunc, wg *sync.WaitGroup, cleanupChan chan os.Signal, openVideo, openMeta *os.File) {

	skipVideos := config.GetBool(keys.SkipVideos)

	var (
		videoMap,
		metaMap,
		matchedFiles map[string]*models.FileData

		err error
	)

	// Process metadata, checking if it’s a directory or a single file
	if openMeta != nil {
		fileInfo, _ := openMeta.Stat()
		if fileInfo.IsDir() {
			metaMap, err = fsRead.GetMetadataFiles(openMeta)
		} else {
			metaMap, err = fsRead.GetSingleMetadataFile(openMeta)
		}
		if err != nil {
			logging.PrintE(0, "Error: %v", err)
			os.Exit(1)
		}
	}
	// Process video files, checking if it’s a directory or a single file
	if openVideo != nil {
		fileInfo, _ := openVideo.Stat()
		if fileInfo.IsDir() {
			videoMap, err = fsRead.GetVideoFiles(openVideo)
		} else if !skipVideos {
			videoMap, err = fsRead.GetSingleVideoFile(openVideo)
		}
		if err != nil {
			logging.PrintE(0, "Error fetching video files: %v", err)
			os.Exit(1)
		}

		// Match video and metadata files
		if !skipVideos {
			matchedFiles, err = fsRead.MatchVideoWithMetadata(videoMap, metaMap)
			if err != nil {
				logging.PrintE(0, "Error matching videos with metadata: %v", err)
				os.Exit(1)
			}
		} else {
			matchedFiles = metaMap
		}
	}

	config.Set(keys.VideoMap, videoMap)
	config.Set(keys.MetaMap, metaMap)

	atomic.StoreInt32(&totalMetaFiles, int32(len(metaMap)))
	atomic.StoreInt32(&totalVideoFiles, int32(len(videoMap)))

	fmt.Printf("\nFound %d file(s) to process in the directory\n", totalMetaFiles+totalVideoFiles)

	logging.PrintD(3, "Matched metafiles: %v", matchedFiles)

	for _, fileData := range matchedFiles {

		var (
			processedData *models.FileData
			err           error
		)

		if !config.IsSet(keys.SkipVideos) || metaChanges() {
			switch fileData.MetaFileType {
			case enums.METAFILE_JSON:
				logging.PrintD(3, "File: %s: Meta file type in model as %v", fileData.JSONFilePath, fileData.MetaFileType)
				processedData, err = reader.ProcessJSONFile(fileData)

			case enums.METAFILE_NFO:
				logging.PrintD(3, "File: %s: Meta file type in model as %v", fileData.NFOFilePath, fileData.MetaFileType)
				processedData, err = reader.ProcessNFOFiles(fileData)
			}
			if err != nil {
				logging.ErrorArray = append(logging.ErrorArray, err)
				errMsg := fmt.Errorf("error processing metadata for file: %w", err)
				logging.PrintE(0, errMsg.Error())
				return
			}
			processedDataArray = append(processedDataArray, processedData)
		} else {
			processedDataArray = append(processedDataArray, fileData)
		}
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

	sem := make(chan struct{}, config.GetInt(keys.Concurrency))

	for fileName, fileData := range matchedFiles {

		executeFile(ctx, wg, sem, fileName, fileData)
	}

	wg.Wait()

	err = cleanupTempFiles(videoMap)
	if err != nil {
		logging.ErrorArray = append(logging.ErrorArray, err)
		logging.PrintE(0, "Failed to cleanup temp files: %v", err)
	}

	replaceToStyle := config.Get(keys.Rename).(enums.ReplaceToStyle)
	inputVideoDir := config.GetString(keys.JsonDir)

	err = transformations.FileRename(processedDataArray, replaceToStyle)
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

		skipVideos := config.GetBool(keys.SkipVideos)
		isVideoFile := fileData.OriginalVideoPath != ""

		if isVideoFile {
			logging.PrintI("Processing file: %s", fileName)
		} else {
			logging.PrintI("Processing metadata file: %s", fileName)
		}

		switch {
		case isVideoFile && !skipVideos:

			if err := writer.WriteMetadata(fileData); err != nil {
				logging.ErrorArray = append(logging.ErrorArray, err)
				errMsg := fmt.Errorf("failed to process video '%v': %w", fileName, err)
				logging.PrintE(0, errMsg.Error())
			} else {
				logging.PrintS(0, "Successfully processed video %s", fileName)
			}

		default:
			logging.PrintS(0, "Successfully processed metadata for %s", fileName)
		}

		currentFile = atomic.AddInt32(&processedVideoFiles, 1)
		total = atomic.LoadInt32(&totalVideoFiles)

		fmt.Printf("\n====================================================\n")
		fmt.Printf("    Processed video file %d of %d\n", currentFile, total)
		fmt.Printf("    Remaining: %d\n", total-currentFile)
		fmt.Printf("====================================================\n\n")

	}(fileName, fileData)
}
