package transformations

import (
	"io"
	"os"
	"path/filepath"
	"testing"

	"metarr/internal/domain/logger"
	"metarr/internal/models"

	"github.com/TubarrApp/gocommon/sharedconsts"
)

// TestRestoreOriginalName tests a backed-up original gets its name back only when the new video no longer has it.
func TestRestoreOriginalName(t *testing.T) {
	logger.Pl.Console = io.Discard
	for _, tc := range []struct {
		name      string
		newVideo  bool // The new video still has the original's path.
		wantOwned bool // The original has its own name back.
	}{
		{"new video moved or renamed", false, true},
		{"new video kept the path", true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			orig := filepath.Join(dir, "video.mp4")
			backup := filepath.Join(dir, "video"+sharedconsts.BackupTag+".mp4")
			if err := os.WriteFile(backup, []byte("original"), 0o644); err != nil {
				t.Fatal(err)
			}
			if tc.newVideo {
				if err := os.WriteFile(orig, []byte("new"), 0o644); err != nil {
					t.Fatal(err)
				}
			}

			fd := &models.FileData{OriginalVideoPath: orig, BackupVideoPath: backup}
			if err := restoreOriginalName(fd); err != nil {
				t.Fatal(err)
			}

			// The original's bytes are at its own path, or still at the backup's.
			got, err := os.ReadFile(orig)
			owned := err == nil && string(got) == "original"
			if _, err := os.Stat(backup); owned != tc.wantOwned || (err == nil) == tc.wantOwned {
				t.Errorf("original has its name back: %v, backup left: %v; want %v, %v", owned, err == nil, tc.wantOwned, !tc.wantOwned)
			}
		})
	}
}
