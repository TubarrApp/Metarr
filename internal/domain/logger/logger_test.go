package logger

import (
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/TubarrApp/gocommon/logging"
)

// TestSendLogsHTTPS tests that logs reach the URL Tubarr passes, over HTTPS with a self-signed certificate.
func TestSendLogsHTTPS(t *testing.T) {
	var gotToken, gotBody string
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotToken = r.Header.Get(logTokenHeader)
		body, _ := io.ReadAll(r.Body)
		gotBody = string(body)
	}))
	defer srv.Close()

	pl, err := logging.SetupLogging(logging.Config{
		LogFilePath: filepath.Join(t.TempDir(), "metarr.log"),
		Console:     io.Discard,
		Program:     "Metarr",
	})
	if err != nil {
		t.Fatal(err)
	}
	Pl, sentLogs = pl, 0
	t.Setenv(logURLEnv, srv.URL+"/metarr-logs")
	t.Setenv(logTokenEnv, "secret")

	Pl.I("hello from metarr")
	SendLogs()

	if gotToken != "secret" {
		t.Errorf("token = %q, want %q", gotToken, "secret")
	}
	if !strings.Contains(gotBody, "hello from metarr") {
		t.Errorf("body %q missing the log line", gotBody)
	}
	if sentLogs == 0 {
		t.Error("sent logs not marked as sent")
	}
}
