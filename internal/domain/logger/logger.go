// Package logger holds the program logger.
package logger

import (
	"bytes"
	"net/http"
	"sync"

	"github.com/TubarrApp/gocommon/logging"
)

// Tubarr endpoint.
const tubarrLogServer = "http://127.0.0.1:8827/metarr-logs"

// Log vars.
var (
	Pl              = new(logging.ProgramLogger)
	logMutex        sync.Mutex
	lastSentPos     int
	lastSentWrapped bool
)

// SendLogs POSTs logs to Tubarr.
func SendLogs() {
	logMutex.Lock()
	defer logMutex.Unlock()

	// Get new logs since last successful send.
	logs := Pl.GetLogsSincePosition(lastSentPos, lastSentWrapped)

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
			lastSentPos = Pl.GetBufferPosition()
			lastSentWrapped = Pl.IsBufferFull()
		}

		if closeErr := resp.Body.Close(); closeErr != nil {
			Pl.E("Could not close response body: %v", closeErr)
		}
	}
}
