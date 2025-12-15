// Package logger holds the program logger.
package logger

import (
	"bytes"
	"net/http"
	"sync"

	"github.com/TubarrApp/gocommon/logging"
)

// Pl is the logger for the program.
var Pl = new(logging.ProgramLogger)

// Log vars.
var (
	tubarrLogServer = "http://127.0.0.1:8827/metarr-logs"
	logMutex        sync.Mutex
	lastSentPos     int
	lastSentWrapped bool
)

// SendLogs POSTs logs to Tubarr.
func SendLogs() {
	logMutex.Lock()
	defer logMutex.Unlock()

	pl, ok := logging.GetProgramLogger("Metarr")
	if !ok {
		return
	}

	// Get new logs since last successful send.
	logs := pl.GetLogsSincePosition(lastSentPos, lastSentWrapped)

	if len(logs) > 0 {
		// POST logs to Tubarr.
		body := bytes.Join(logs, []byte{})
		resp, err := http.Post(tubarrLogServer, "text/plain", bytes.NewReader(body))
		if err != nil {
			Pl.E("Could not send logs to Tubarr: %v", err)
			return
		}

		// Update tracking if POST was successful.
		if resp.StatusCode == http.StatusOK {
			lastSentPos = pl.GetBufferPosition()
			lastSentWrapped = pl.IsBufferFull()
		}

		if closeErr := resp.Body.Close(); closeErr != nil {
			Pl.E("Could not close response body: %v", closeErr)
		}
	}
}
