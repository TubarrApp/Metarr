package ffmpeg

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"metarr/internal/abstractions"
	"metarr/internal/domain/keys"
	"metarr/internal/models"

	"github.com/TubarrApp/gocommon/sharedconsts"
)

// TestPurgeOriginalVideo tests the original video is kept unless purged, as a backup if the new video takes its path.
func TestPurgeOriginalVideo(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("FFmpeg isn't installed")
	}
	t.Cleanup(func() {
		abstractions.Set(keys.OutputFiletype, "")
		abstractions.Set(keys.PurgeOriginalVideo, false)
	})

	for _, tc := range []struct {
		name   string
		outExt string
		purge  bool
	}{
		{"new extension", sharedconsts.ExtMKV, false},
		{"new extension, purged", sharedconsts.ExtMKV, true},
		{"same extension", sharedconsts.ExtMP4, false},
		{"same extension, purged", sharedconsts.ExtMP4, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			abstractions.Set(keys.OutputFiletype, tc.outExt)
			abstractions.Set(keys.PurgeOriginalVideo, tc.purge)

			// Make a short source video.
			dir := t.TempDir()
			orig := filepath.Join(dir, "video"+sharedconsts.ExtMP4)
			if out, err := exec.Command("ffmpeg", "-v", "error", "-f", "lavfi", "-i", "testsrc=s=64x36:r=5:d=0.4", "-c:v", "mpeg4", orig).CombinedOutput(); err != nil {
				t.Fatalf("making source video: %v\n%s", err, out)
			}

			fd := models.NewFileData()
			fd.VideoDirectory = dir
			fd.OriginalVideoPath = orig
			if err := ExecuteVideo(t.Context(), fd); err != nil {
				t.Fatal(err)
			}

			// The original keeps its own name, unless the new video took it.
			keptPath := orig
			if tc.outExt == sharedconsts.ExtMP4 {
				keptPath = filepath.Join(dir, "video"+sharedconsts.BackupTag+sharedconsts.ExtMP4)
			}
			_, newErr := os.Stat(fd.PostFFmpegVideoPath)
			_, keptErr := os.Stat(keptPath)
			if newErr != nil || (keptErr == nil) == tc.purge {
				t.Errorf("new video: %v, original kept: %v; want the new video, original kept: %v", newErr, keptErr == nil, !tc.purge)
			}
		})
	}
}
