// Package processing is the primary Metarr process, handling batches of files and/or directories.
package processing

import (
	"fmt"
	"metarr/internal/abstractions"
	"metarr/internal/domain/keys"
	"metarr/internal/models"
	"metarr/internal/utils/logging"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
)

var (
	logInit    bool
	muBatchID  sync.Mutex
	muLogSetup sync.Mutex
	atomID     int64
)

type batch struct {
	ID         int64
	Video      string
	JSON       string
	IsDirs     bool
	SkipVideos bool
	MetaOps    *models.MetaOps
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

// StartBatchLoop begins processing the batch.
func StartBatchLoop(core *models.Core, batches []models.BatchConfig) error {
	if len(batches) == 0 {
		logging.I("No batches sent in?")
		return nil
	}

	job := 1
	skipVideos := abstractions.GetBool(keys.SkipVideos)

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
					return fmt.Errorf("failed to close failed video %q after JSON failure: %w", openVideo.Name(), err)
				}
			}
			continue
		}

		// Start logging
		if !logInit {
			muLogSetup.Lock()
			dir := filepath.Dir(openJSON.Name())
			logging.I("Setting log file at %q", dir)

			if err = logging.SetupLogging(dir); err != nil {
				fmt.Printf("\n\nWarning: Log file was not created\nReason: %s\n\n", err)
			}
			logInit = true
			muLogSetup.Unlock()
		}

		// Initiate batch process
		if err := processBatch(batch, core, openVideo, openJSON); err != nil {
			return err
		}

		// Completion message
		fileOrDirMsg := "Directory"
		if !batch.IsDirs {
			fileOrDirMsg = "File"
		}

		var videoDoneMsg string
		if !batch.SkipVideos {
			videoDoneMsg = fmt.Sprintf("Video %s: %q\n", fileOrDirMsg, batch.Video)
		}
		logging.I("Finished tasks for:\n\n%sJSON %s: %q\n", videoDoneMsg, fileOrDirMsg, batch.JSON)

		// Close files explicitly at the end of each iteration
		if openVideo != nil {
			if err := openVideo.Close(); err != nil {
				logging.E("Failed to close video file %q after successful iteration: %v", openVideo.Name(), err)
			}
		}
		if err := openJSON.Close(); err != nil {
			logging.E("Failed to close JSON file %q after successful iteration: %v", openVideo.Name(), err)
		}

		job++
	}
	logging.I("All batch tasks finished!")
	return nil
}

// convertCfgToBatch converts a config batch to a local batch.
func convertCfgToBatch(config models.BatchConfig) *batch {

	muBatchID.Lock()
	atomic.AddInt64(&atomID, 1)
	id := atomic.LoadInt64(&atomID)
	muBatchID.Unlock()

	newBatch := &batch{
		ID:         id,
		Video:      config.Video,
		JSON:       config.JSON,
		IsDirs:     config.IsDirs,
		MetaOps:    config.MetaOps,
		SkipVideos: config.SkipVideos,
	}

	newBatch.MetaOps = models.EnsureMetaOps(newBatch.MetaOps)
	return newBatch
}
