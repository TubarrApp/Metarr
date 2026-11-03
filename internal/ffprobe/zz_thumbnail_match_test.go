package ffprobe

import (
	"io"
	"os/exec"
	"path/filepath"
	"testing"

	"metarr/internal/abstractions"
	"metarr/internal/domain/keys"
	"metarr/internal/domain/logger"
	"metarr/internal/models"

	"github.com/TubarrApp/gocommon/sharedconsts"
)

// TestEmbeddedThumbnailNotInMetafile tests a video isn't reprocessed for having a thumbnail its metafile doesn't name, unless it's being stripped.
func TestEmbeddedThumbnailNotInMetafile(t *testing.T) {
	if _, err := exec.LookPath(sharedconsts.ProgramFFmpeg); err != nil {
		t.Skip("FFmpeg isn't installed")
	}
	logger.Pl.Console = io.Discard
	t.Cleanup(func() { abstractions.Set(keys.StripThumbnails, false) })

	dir := t.TempDir()
	thumbnail := filepath.Join(dir, "thumbnail.jpg")
	if out, err := exec.Command(sharedconsts.ProgramFFmpeg, "-nostdin", "-v", "error", "-f", "lavfi", "-i", "testsrc=d=0.1:s=32x32", "-frames:v", "1", thumbnail).CombinedOutput(); err != nil {
		t.Fatalf("making thumbnail: %v\n%s", err, out)
	}
	path := filepath.Join(dir, "v.mp4")
	if out, err := exec.Command(sharedconsts.ProgramFFmpeg, "-nostdin", "-v", "error", "-f", "lavfi", "-i", "testsrc=s=64x36:r=5:d=0.4", "-i", thumbnail,
		"-map", "0", "-map", "1", "-c:v:0", "mpeg4", "-c:v:1", "copy", "-disposition:v:1", "attached_pic", path).CombinedOutput(); err != nil {
		t.Fatalf("making source video: %v\n%s", err, out)
	}

	for _, strip := range []bool{false, true} {
		abstractions.Set(keys.StripThumbnails, strip)
		fd := models.NewFileData()
		fd.OriginalVideoPath = path
		if got := CheckMetaMatches(t.Context(), sharedconsts.ExtMP4, fd, map[string]string{}); got == strip {
			t.Errorf("strip %v: metadata matches = %v", strip, got)
		}
		if !fd.HasEmbeddedThumbnail {
			t.Errorf("strip %v: thumbnail not noticed", strip)
		}
	}
}

// TestMissingThumbnailOnlyMattersWhereItFits tests a video without the metadata's thumbnail is only reprocessed in a container that can hold one.
func TestMissingThumbnailOnlyMattersWhereItFits(t *testing.T) {
	if _, err := exec.LookPath(sharedconsts.ProgramFFmpeg); err != nil {
		t.Skip("FFmpeg isn't installed")
	}
	logger.Pl.Console = io.Discard

	path := filepath.Join(t.TempDir(), "v.mp4")
	if out, err := exec.Command(sharedconsts.ProgramFFmpeg, "-nostdin", "-v", "error", "-f", "lavfi", "-i", "testsrc=s=64x36:r=5:d=0.4", "-c:v", "mpeg4", path).CombinedOutput(); err != nil {
		t.Fatalf("making source video: %v\n%s", err, out)
	}
	fd := models.NewFileData()
	fd.OriginalVideoPath = path
	fd.MWebData.Thumbnail = "https://example.com/thumb.jpg"

	for ext, want := range map[string]bool{
		sharedconsts.ExtWEBM: true,
		sharedconsts.ExtMOV:  true,
		sharedconsts.ExtMP4:  false,
		sharedconsts.ExtMKV:  false,
	} {
		if got := CheckMetaMatches(t.Context(), ext, fd, map[string]string{}); got != want {
			t.Errorf("CheckMetaMatches for %s = %v, want %v", ext, got, want)
		}
	}
}
