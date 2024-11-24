package processing

import (
	"fmt"
	"metarr/internal/cfg"
	keys "metarr/internal/domain/keys"
	"metarr/internal/models"
	logging "metarr/internal/utils/logging"
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
	Json       string
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

// StartBatchLoop begins processing the batch.
func StartBatchLoop(core *models.Core) error {
	if !cfg.IsSet(keys.BatchPairs) {
		logging.I("No batches sent in?")
		return nil
	}

	batches, ok := cfg.Get(keys.BatchPairs).([]cfg.BatchConfig)
	if !ok {
		logging.E(0, "Wrong type or null batch pair. Type: %T", batches)
		return nil
	}

	job := 1
	skipVideos := cfg.GetBool(keys.SkipVideos)

	// Begin iteration...
	for _, b := range batches {
		var (
			openVideo *os.File
			openJson  *os.File
			err       error
		)

		batch := convertCfgToBatch(b)
		logging.I("Starting batch job %d. Skip videos on this run? %v", job, batch.SkipVideos)

		if batch.SkipVideos {
			skipVideos = true
		}

		// Open video file if necessary
		if !skipVideos {
			openVideo, err = os.Open(batch.Video)
			if err != nil {
				logging.E(0, "Failed to open %s", batch.Video)
				continue
			}
		}

		// Open JSON file
		openJson, err = os.Open(batch.Json)
		if err != nil {
			logging.E(0, "Failed to open %s", batch.Json)

			if openVideo != nil {
				openVideo.Close()
			}

			continue
		}

		// Start logging
		if !logInit {
			muLogSetup.Lock()
			dir := filepath.Dir(openJson.Name())
			logging.I("Setting log file at %q", dir)

			if err = logging.SetupLogging(dir); err != nil {
				fmt.Printf("\n\nWarning: Log file was not created\nReason: %s\n\n", err)
			}
			logInit = true
			muLogSetup.Unlock()
		}

		// Initiate batch process
		if err := processBatch(batch, core, openVideo, openJson); err != nil {
			return err
		}

		logging.I("Finished tasks for files/directories:\n\nVideo: %q\nJSON: %q\n", batch.Video, batch.Json)

		// Close files explicitly at the end of each iteration
		if openVideo != nil {
			openVideo.Close()
		}
		openJson.Close()

		job++
	}

	logging.I("All batch tasks finished!")
	return nil
}

// convertCfgToBatch converts a config batch to a local batch.
func convertCfgToBatch(config cfg.BatchConfig) *batch {

	muBatchID.Lock()
	atomic.AddInt64(&atomID, 1)
	id := atomic.LoadInt64(&atomID)
	muBatchID.Unlock()

	return &batch{
		ID:         id,
		Video:      config.Video,
		Json:       config.Json,
		IsDirs:     config.IsDirs,
		SkipVideos: config.SkipVideos,
	}
}
