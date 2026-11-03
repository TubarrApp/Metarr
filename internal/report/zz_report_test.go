package report

import (
	"errors"
	"io"
	"os"
	"testing"
)

// captureStdout returns what a function prints on stdout.
func captureStdout(t *testing.T, printer func()) string {
	t.Helper()
	read, write, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	stdout := os.Stdout
	os.Stdout = write
	t.Cleanup(func() { os.Stdout = stdout })

	printer()
	if err := write.Close(); err != nil {
		t.Fatal(err)
	}
	out, err := io.ReadAll(read)
	if err != nil {
		t.Fatal(err)
	}
	return string(out)
}

// TestWording tests the exact lines printed, as Tubarr reads them from Metarr's stdout.
func TestWording(t *testing.T) {
	for _, c := range []struct {
		name, want string
		printer    func()
	}{
		{"video path", "current video path: /v.mp4\n", func() { VideoPath("/v.mp4") }},
		{"final paths", "final video path: /v.mp4\nfinal json path: /v.json\n", func() { FinalPaths("/v.mp4", "/v.json") }},
		{"error's first line", "metarr error: failed\n", func() { Error(errors.New("failed\n\nCaptured output:\nnoise")) }},
	} {
		if got := captureStdout(t, c.printer); got != c.want {
			t.Errorf("%s: got %q, want %q", c.name, got, c.want)
		}
	}
}
