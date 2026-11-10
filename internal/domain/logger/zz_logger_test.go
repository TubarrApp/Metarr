package logger

import (
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/TubarrApp/gocommon/logging"
	"github.com/TubarrApp/gocommon/sharedconsts"
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
	t.Setenv(sharedconsts.EnvTubarrLogURL, srv.URL+"/metarr-logs")
	t.Setenv(sharedconsts.EnvTubarrLogToken, "secret")

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

// TestSendLogsFollowsLevel tests that Metarr takes the debug level Tubarr replies with, even with no new lines to send.
func TestSendLogsFollowsLevel(t *testing.T) {
	reply := "3"
	posts := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		posts++
		_, _ = io.WriteString(w, reply)
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
	t.Setenv(sharedconsts.EnvTubarrLogURL, srv.URL+"/metarr-logs")
	t.Cleanup(func() { logging.SetLevel(-1) })

	// No lines logged, so this post only asks for the level.
	logging.SetLevel(0)
	SendLogs()
	if posts != 1 {
		t.Fatalf("posts = %d, want 1 even with no new lines", posts)
	}
	if got := logging.Level(); got != 3 {
		t.Errorf("level = %d, want 3", got)
	}

	// Out of range levels are clamped to Metarr's range.
	reply = "-1"
	SendLogs()
	if got := logging.Level(); got != 0 {
		t.Errorf("level = %d, want 0 for a reply of -1", got)
	}

	// A reply that isn't a level leaves it as it was.
	reply = ""
	logging.SetLevel(2)
	SendLogs()
	if got := logging.Level(); got != 2 {
		t.Errorf("level = %d, want 2 kept for an empty reply", got)
	}
}
