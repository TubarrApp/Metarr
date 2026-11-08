package processing

import (
	"os"
	"path/filepath"
	"sync"
	"testing"

	"metarr/internal/abstractions"
	"metarr/internal/domain/keys"
	"metarr/internal/file"
	"metarr/internal/models"

	"github.com/spf13/viper"
)

// TestVideoDirWithoutMetaDir tests a video folder given alone is batched, and its videos taken with no metafile.
func TestVideoDirWithoutMetaDir(t *testing.T) {
	t.Cleanup(viper.Reset)
	abstractions.Set(keys.InputVExts, []string{"all"})
	if err := file.InitFetchFilesVars(); err != nil {
		t.Fatal(err)
	}

	dir := t.TempDir()
	for _, name := range []string{"a.mp4", "b.mkv", "b.json"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	// Batched with no metadata input.
	abstractions.Set(keys.VideoDirs, []string{dir})
	tasks, err := initializeBatchConfigs()
	if err != nil {
		t.Fatal(err)
	}
	if len(tasks) != 1 || tasks[0].Video != dir || tasks[0].JSON != "" || !tasks[0].IsDirs || tasks[0].SkipVideos {
		t.Fatalf("video folder alone: %+v", tasks)
	}

	// Its videos are taken as they are, even one with a metafile beside it.
	openVideo, err := os.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer closeBatchFile(openVideo)
	b := convertCfgToBatch(tasks[0])
	b.bp = &batchProcessor{batchID: b.ID}
	if err := getFiles(b, nil, openVideo, false); err != nil {
		t.Fatal(err)
	}
	matched := b.bp.syncMapToRegularMap(&b.bp.files.matched)
	if len(matched) != 2 {
		t.Fatalf("took %d videos, want 2", len(matched))
	}
	for name, fd := range matched {
		if fd.OriginalVideoPath != filepath.Join(dir, name) || fd.MetaFilePath != "" {
			t.Errorf("%s: video %q, metafile %q", name, fd.OriginalVideoPath, fd.MetaFilePath)
		}
	}
}

// TestBatchPairsPairFirst tests a batch pair stays paired when a video folder is given alone beside it.
func TestBatchPairsPairFirst(t *testing.T) {
	t.Cleanup(viper.Reset)
	alone := t.TempDir()
	pairVideo := t.TempDir()
	pairMeta := t.TempDir()

	abstractions.Set(keys.VideoDirs, []string{alone})
	abstractions.Set(keys.BatchPairs, models.BatchPairs{VideoDirs: []string{pairVideo}, MetaDirs: []string{pairMeta}})
	tasks, err := initializeBatchConfigs()
	if err != nil {
		t.Fatal(err)
	}
	if len(tasks) != 2 || tasks[0].Video != pairVideo || tasks[0].JSON != pairMeta || tasks[1].Video != alone || tasks[1].JSON != "" {
		t.Errorf("tasks: %+v", tasks)
	}
}

// TestMissingSourceStopsRun tests a source that isn't there is an error, rather than left out to shift the pairs after it.
func TestMissingSourceStopsRun(t *testing.T) {
	t.Cleanup(viper.Reset)
	videos := []string{t.TempDir(), t.TempDir()}
	abstractions.Set(keys.VideoDirs, videos)
	abstractions.Set(keys.MetaDirs, []string{filepath.Join(t.TempDir(), "missing"), t.TempDir()})
	if tasks, err := initializeBatchConfigs(); err == nil {
		t.Errorf("no error, got tasks: %+v", tasks)
	}
}

// TestVideoOnlyBatchSkippingVideos tests a batch with no metadata input is refused when videos are skipped.
func TestVideoOnlyBatchSkippingVideos(t *testing.T) {
	b := &batch{ID: 1, Video: t.TempDir(), IsDirs: true, SkipVideos: true}
	if _, err := runBatch(b, &models.Core{Ctx: t.Context(), Wg: &sync.WaitGroup{}}); err == nil {
		t.Error("no error for a batch with nothing to process")
	}
}
