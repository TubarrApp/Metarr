// Package processing is the primary Metarr process, handling batches of files and/or directories.
package processing

import (
	"fmt"
	"metarr/internal/abstractions"
	"metarr/internal/domain/enums"
	"metarr/internal/domain/keys"
	"metarr/internal/models"
	"metarr/internal/transformations"
	"metarr/internal/utils/logging"
	"os"
	"sort"
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

// StartMainBatchLoop begins processing the batch.
func StartMainBatchLoop(core *models.Core, batches []models.BatchConfig) ([]*models.FileData, error) {
	if len(batches) == 0 {
		logging.I("No batches sent in?")
		return nil, nil
	}

	job := 1
	skipVideos := abstractions.GetBool(keys.SkipVideos)

	// Collect all processed files from all batches
	allProcessedFiles := make([]*models.FileData, 0)

	// Begin iteration...
	for _, b := range batches {
		var (
			openVideo *os.File
			openJSON  *os.File
			err       error
		)

		batch := convertCfgToBatch(b)
		logging.I("Starting batch job %d. Skip videos on this run? %v", job, batch.SkipVideos)

		if batch.SkipVideos {
			skipVideos = true
		}

		// Open video file if necessary
		if !skipVideos {
			if openVideo, err = os.Open(batch.Video); err != nil {
				logging.E("Failed to open %s", batch.Video)
				continue
			}
		}

		// Open JSON file
		if openJSON, err = os.Open(batch.JSON); err != nil {
			logging.E("Failed to open %s", batch.JSON)
			// Close accompanying video...
			if openVideo != nil {
				if err := openVideo.Close(); err != nil {
					return allProcessedFiles, fmt.Errorf("failed to close failed video %q after JSON failure: %w", openVideo.Name(), err)
				}
			}
			continue
		}

		// Initiate batch process
		processedFiles, err := processBatch(batch, core, openVideo, openJSON)
		if err != nil {
			return allProcessedFiles, err
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

		logging.I("Finished tasks for:\n\n%sInput JSON %s: %q\n", videoDoneMsg, fileOrDirMsg, batch.JSON)

		// Close files explicitly at the end of each iteration
		if openVideo != nil {
			if err := openVideo.Close(); err != nil {
				logging.E("Failed to close video file %q after successful iteration: %v", openVideo.Name(), err)
			}
		}
		if err := openJSON.Close(); err != nil {
			logging.E("Failed to close JSON file %q after successful iteration: %v", openJSON.Name(), err)
		}
		job++
	}
	logging.I("All batch tasks finished!")
	return allProcessedFiles, nil
}

// RenameFiles begins file renaming operations for a batch.
func RenameFiles(fdArray []*models.FileData) error {
	var replaceStyle enums.ReplaceToStyle
	skipVideos := abstractions.GetBool(keys.SkipVideos)

	if abstractions.IsSet(keys.Rename) {
		if style, ok := abstractions.Get(keys.Rename).(enums.ReplaceToStyle); ok {
			replaceStyle = style
			logging.D(2, "Got rename style as %T index %v", replaceStyle, replaceStyle)
		} else {
			return fmt.Errorf("invalid rename style type")
		}
	}

	// Create a copy to sort
	sortedFiles := make([]*models.FileData, 0, len(fdArray))
	for _, fd := range fdArray {
		if fd != nil {
			sortedFiles = append(sortedFiles, fd)
		}
	}

	// Sort alphabetically by meta path
	sort.Slice(sortedFiles, func(i, j int) bool {
		return sortedFiles[i].MetaFilePath < sortedFiles[j].MetaFilePath
	})

	// Iterate over sorted list
	processedDirs := make(map[string]bool)
	for _, fd := range sortedFiles {
		if fd == nil {
			continue
		}

		// Rename
		if err := transformations.FileRename(fd, replaceStyle, skipVideos); err != nil {
			logging.AddToErrorArray(err)
			logging.E("Failed to rename file %q: %v", fd.OriginalVideoBaseName, err)
			continue
		}

		// Track directory for success message
		var directory string
		if fd.MetaDirectory != "" {
			directory = fd.MetaDirectory
		} else if fd.VideoDirectory != "" {
			directory = fd.VideoDirectory
		}
		if directory != "" {
			processedDirs[directory] = true
		}
	}

	// Log success per directory
	for dir := range processedDirs {
		logging.S("Successfully formatted file names in directory: %s", dir)
	}

	return nil
}

// convertCfgToBatch converts a config batch to a local batch.
func convertCfgToBatch(config models.BatchConfig) *batch {
	atomic.AddInt64(&atomID, 1)
	id := atomic.LoadInt64(&atomID)

	newBatch := &batch{
		ID:         id,
		Video:      config.Video,
		JSON:       config.JSON,
		IsDirs:     config.IsDirs,
		SkipVideos: config.SkipVideos,
	}
	return newBatch
}
