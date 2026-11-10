// Package logger holds the program logger.
package logger

import (
	"bytes"
	"cmp"
	"crypto/tls"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/TubarrApp/gocommon/logging"
	"github.com/TubarrApp/gocommon/sharedconsts"
)

// Tubarr endpoint, and the token Tubarr passes Metarr to authorize its logs.
const (
	tubarrLogServer = "http://127.0.0.1:8827/metarr-logs" // Used when Tubarr doesn't pass logURLEnv.
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

	logClient = newLogClient()
)

// newLogClient returns the client for posting logs to Tubarr, which may serve HTTPS with a self-signed certificate.
//
// Certificates aren't verified: the log token, known only to the Tubarr that started Metarr, is what matters.
func newLogClient() *http.Client {
	t := http.DefaultTransport.(*http.Transport).Clone()
	t.Proxy = nil                                             // Tubarr is local, never go through a proxy.
	t.TLSClientConfig = &tls.Config{InsecureSkipVerify: true} //nolint:gosec // See above.
	return &http.Client{Timeout: logPostTimeout, Transport: t}
}

// SendLogs POSTs new log lines to Tubarr, and takes the debug level Tubarr replies with.
func SendLogs() {
	logMutex.Lock()
	defer logMutex.Unlock()

	// Post even with no new lines, so a quiet Metarr still picks up level changes.
	logs, next := Pl.LogsSince(sentLogs)
	req, err := http.NewRequest(http.MethodPost, cmp.Or(os.Getenv(sharedconsts.EnvTubarrLogURL), tubarrLogServer), bytes.NewReader(bytes.Join(logs, []byte{})))
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "text/plain")
	req.Header.Set(logTokenHeader, os.Getenv(sharedconsts.EnvTubarrLogToken))
	resp, _ := logClient.Do(req) // Do not check error, error expected if running in CLI-only.
	if resp == nil {
		return
	}
	if resp.StatusCode == http.StatusOK {
		sentLogs = next

		// Follow Tubarr's debug level, ignoring replies that aren't one.
		reply, _ := io.ReadAll(io.LimitReader(resp.Body, 16))
		if level, err := strconv.Atoi(strings.TrimSpace(string(reply))); err == nil {
			logging.SetLevel(min(max(level, 0), 5))
		}
	}
	if closeErr := resp.Body.Close(); closeErr != nil {
		Pl.E("Could not close response body: %v", closeErr)
	}
}
