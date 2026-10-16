package file

import (
	"io"
	"os"
	"path/filepath"
	"testing"

	"metarr/internal/domain/logger"
)

// TestMain gives the global logger a console writer, since the zero value has none.
func TestMain(m *testing.M) {
	logger.Pl.Console = io.Discard
	os.Exit(m.Run())
}

// TestCopyFileCreatesDestDir tests that copyFile creates the destination's directory, not a directory at the file's path.
func TestCopyFileCreatesDestDir(t *testing.T) {
	tmp := t.TempDir()
	src := filepath.Join(tmp, "src.mp4")
	if err := os.WriteFile(src, []byte("video"), 0o644); err != nil {
		t.Fatal(err)
	}
	dst := filepath.Join(tmp, "new", "dir", "dst.mp4")

	if err := copyFile(src, dst); err != nil {
		t.Fatalf("copyFile: %v", err)
	}
	info, err := os.Stat(dst)
	if err != nil {
		t.Fatal(err)
	}
	if !info.Mode().IsRegular() {
		t.Fatalf("destination is %v, want a regular file", info.Mode())
	}
}
