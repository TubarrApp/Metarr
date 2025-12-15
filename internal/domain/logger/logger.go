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
		resp, _ := http.Post(tubarrLogServer, "text/plain", bytes.NewReader(body)) // Do not check error, error expected if running in CLI-only.

		// Update tracking if POST was successful.
		if resp != nil {
			if resp.StatusCode == http.StatusOK {
				lastSentPos = Pl.GetBufferPosition()
				lastSentWrapped = Pl.IsBufferFull()
			}

			if closeErr := resp.Body.Close(); closeErr != nil {
				Pl.E("Could not close response body: %v", closeErr)
			}
		}
	}
}
