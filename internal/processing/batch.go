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

	// Begin iteration...
	for _, batch := range batches {
		var (
			openVideo *os.File
			openJson  *os.File
			err       error
		)

		logging.I("Starting batch job %d. Skip videos on this run? %v", job, batch.SkipVideos)
		skipVideos := cfg.GetBool(keys.SkipVideos) || batch.SkipVideos

		if !skipVideos {
			openVideo, err = os.Open(batch.Video)
			if err != nil {
				logging.E(0, "Failed to open %s", batch.Video)
				continue
			}
			defer openVideo.Close()
		}

		openJson, err = os.Open(batch.Json)
		if err != nil {
			logging.E(0, "Failed to open %s", batch.Json)

			if !skipVideos {
				openVideo.Close()
			}

			continue
		}
		defer openJson.Close()

		// Start logging
		if !logInit {
			dir, err := filepath.Abs(openJson.Name())
			if err != nil {
				logging.E(0, "Failed to initialize logging on this run, could not get absolute path of %v", openJson.Name())
			}
			dir = strings.TrimSuffix(dir, openJson.Name())
			logging.I("Setting log file at '%s'", dir)

			if err = logging.SetupLogging(dir); err != nil {
				fmt.Printf("\n\nNotice: Log file was not created\nReason: %s\n\n", err)
			}
			logInit = true
		}

		ProcessFiles(batch, core, openVideo, openJson)
		logging.I("Finished tasks for video file/dir '%s' and JSON file/dir '%s'", batch.Video, batch.Json)

		// Reset for next loop
		if !skipVideos {
			openVideo.Close()
		}
		openJson.Close()
		job++
	}
	logging.I("All batch tasks finished!")
}
