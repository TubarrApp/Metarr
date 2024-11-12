package processing

import (
	"fmt"
	"metarr/internal/cfg"
	keys "metarr/internal/domain/keys"
	"metarr/internal/models"
	logging "metarr/internal/utils/logging"
	"os"
	"path/filepath"
)

var logInit bool

func StartBatchLoop() {
	if !cfg.IsSet(keys.BatchPairs) {
		logging.I("No batches sent in?")
		return
	}

	batches, ok := cfg.Get(keys.BatchPairs).([]*models.Batch)
	if !ok {
		logging.E(0, "Wrong type or null batch pair. Type: %T", batches)
		return
	}

	// Begin iteration...
	for _, batch := range batches {
		var (
			openVideo *os.File
			openJson  *os.File
			err       error
		)

		if !batch.SkipVideos {
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
			openVideo.Close()
			continue
		}
		defer openJson.Close()

		// Start logging
		if !logInit {
			dir, err := filepath.Abs(openJson.Name())
			if err != nil {
				logging.E(0, "Failed to initialize logging on this run")

			} else if err = logging.SetupLogging(dir); err != nil {
				fmt.Printf("\n\nNotice: Log file was not created\nReason: %s\n\n", err)
			}
			logInit = true
		}

		ProcessFiles(batch, openVideo, openJson)
		logging.I("Finished tasks for video file/dir '%s' and JSON file/dir '%s'", openVideo.Name(), openJson.Name())

		// Reset for next loop
		openVideo.Close()
		openJson.Close()
	}

	logging.I("All batch tasks finished!")
}
