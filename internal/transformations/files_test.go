package transformations

import (
	"io"
	"os"
	"path/filepath"
	"testing"

	"metarr/internal/domain/keys"
	"metarr/internal/domain/logger"
	"metarr/internal/models"

	"github.com/spf13/viper"
)

// TestProcessMovesWithoutRename tests that an output directory moves files even when no rename style is set.
func TestProcessMovesWithoutRename(t *testing.T) {
	logger.Pl.Console = io.Discard
	in, out := t.TempDir(), filepath.Join(t.TempDir(), "2026")
	video, meta := filepath.Join(in, "v.mp4"), filepath.Join(in, "v.json")
	for _, f := range []string{video, meta} {
		if err := os.WriteFile(f, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	viper.Set(keys.OutputDirectory, out)
	t.Cleanup(viper.Reset)

	fd := &models.FileData{PostFFmpegVideoPath: video, MetaFilePath: meta}
	if err := (&fileProcessor{fd: fd, outputDir: out}).process(); err != nil {
		t.Fatal(err)
	}

	for _, f := range []string{filepath.Join(out, "v.mp4"), filepath.Join(out, "v.json")} {
		if _, err := os.Stat(f); err != nil {
			t.Errorf("%s not moved: %v", filepath.Base(f), err)
		}
	}
	if fd.FinalVideoPath != filepath.Join(out, "v.mp4") || fd.FinalMetaPath != filepath.Join(out, "v.json") {
		t.Errorf("final paths = %q, %q; want them in %q", fd.FinalVideoPath, fd.FinalMetaPath, out)
	}
}
