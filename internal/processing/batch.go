package processing

import (
	"fmt"
	"metarr/internal/cfg"
	keys "metarr/internal/domain/keys"
	"metarr/internal/models"
	logging "metarr/internal/utils/logging"
	"os"
	"path/filepath"
	"strings"
)

var logInit bool

// StartBatchLoop begins processing the batch
func StartBatchLoop(core *models.Core) {
	if !cfg.IsSet(keys.BatchPairs) {
		logging.I("No batches sent in?")
		return
	}

	batches, ok := cfg.Get(keys.BatchPairs).([]models.Batch)
	if !ok {
		logging.E(0, "Wrong type or null batch pair. Type: %T", batches)
		return
	}

	job := 1
	skipVideos := cfg.GetBool(keys.SkipVideos)

	// Begin iteration...
	for _, batch := range batches {
		var (
			openVideo *os.File
			openJson  *os.File
			err       error
		)

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
			dir, err := filepath.Abs(openJson.Name())
			if err != nil {
				logging.E(0, "Failed to initialize logging on this run, could not get absolute path of %v", openJson.Name())
			}
			dir = strings.TrimSuffix(dir, openJson.Name())
			logging.I("Setting log file at %q", dir)

			if err = logging.SetupLogging(dir); err != nil {
				fmt.Printf("\n\nWarning: Log file was not created\nReason: %s\n\n", err)
			}
			logInit = true
		}

		// Process the files
		ProcessFiles(batch, core, openVideo, openJson)
		logging.I("Finished tasks for files/directories:\n\nVideo: %q\nJSON: %q\n", batch.Video, batch.Json)

		// Close files explicitly at the end of each iteration
		if openVideo != nil {
			openVideo.Close()
		}
		openJson.Close()

		job++
	}

	logging.I("All batch tasks finished!")
}
