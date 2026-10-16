// Package logger holds the program logger.
package logger

import (
	"bytes"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/TubarrApp/gocommon/logging"
)

// Tubarr endpoint, and the token Tubarr passes Metarr to authorize its logs.
const (
	tubarrLogServer = "http://127.0.0.1:8827/metarr-logs"
	logTokenEnv     = "TUBARR_LOG_TOKEN"
	logTokenHeader  = "X-Tubarr-Log-Token"
)

// logPostTimeout bounds a log POST.
//
// Posting logs is best-effort, and SendLogs runs from a defer in main. Without a
// timeout, a Tubarr instance which accepts the connection but never responds
// hangs program exit indefinitely, and the signals which would otherwise kill
// the program are trapped for context cancellation.
const logPostTimeout = 3 * time.Second

// Log vars.
var (
	Pl       = new(logging.ProgramLogger)
	logMutex sync.Mutex
	sentLogs uint64 // Log lines already sent to Tubarr.

	logClient = &http.Client{Timeout: logPostTimeout}
)

// SendLogs POSTs logs to Tubarr.
func SendLogs() {
	logMutex.Lock()
	defer logMutex.Unlock()

	logs, next := Pl.LogsSince(sentLogs)
	if len(logs) == 0 {
		return
	}

	req, err := http.NewRequest(http.MethodPost, tubarrLogServer, bytes.NewReader(bytes.Join(logs, []byte{})))
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "text/plain")
	req.Header.Set(logTokenHeader, os.Getenv(logTokenEnv))
	resp, _ := logClient.Do(req) // Do not check error, error expected if running in CLI-only.
	if resp == nil {
		return
	}
	if resp.StatusCode == http.StatusOK {
		sentLogs = next
	}
	if closeErr := resp.Body.Close(); closeErr != nil {
		Pl.E("Could not close response body: %v", closeErr)
	}
}
