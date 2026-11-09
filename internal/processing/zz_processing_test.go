package processing

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"metarr/internal/abstractions"
	"metarr/internal/domain/keys"
	"metarr/internal/domain/logger"
	"metarr/internal/file"
	"metarr/internal/models"

	"github.com/TubarrApp/gocommon/sharedconsts"
	"github.com/TubarrApp/gocommon/sharedenums"
	"github.com/TubarrApp/gocommon/sharedparsing"
	"github.com/spf13/viper"
)

// ******************************** Batches *******************************************************************************************

// TestMain gives the global logger a console writer, since the zero value has none.
func TestMain(m *testing.M) {
	logger.Pl.Console = io.Discard
	os.Exit(m.Run())
}

// TestBrokenNFOGivesError tests an NFO that can't be read returns an error, rather than crashing on the missing data.
func TestBrokenNFOGivesError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "v.nfo")
	if err := os.WriteFile(path, []byte("<movie><title>oops"), 0o644); err != nil {
		t.Fatal(err)
	}
	fd := models.NewFileData()
	fd.MetaFilePath = path
	if err := processNFOFiles(t.Context(), fd, true); err == nil {
		t.Error("no error from a broken NFO")
	}
}

// TestFilenameDateTagIgnoresFolderName tests a date tag in the metafile's folder name isn't mistaken for one in its own.
func TestFilenameDateTagIgnoresFolderName(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "[2025-01-02] folder")
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "v.json")
	if err := os.WriteFile(path, []byte(`{"title":"T","upload_date":"20250102"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	fd := models.NewFileData()
	fd.MetaFilePath = path
	fd.MetaDirectory = dir
	fd.MetaFileType = sharedconsts.MExtJSON
	fd.FilenameOps.DateTag = models.FOpDateTag{Loc: sharedenums.DateTagLocPrefix, DateFormat: sharedenums.DateYyyyMmDd}

	if err := processJSONFile(t.Context(), fd, true); err != nil {
		t.Fatal(err)
	}
	if fd.FilenameDateTag != "[2025-01-02]" {
		t.Errorf("date tag = %q", fd.FilenameDateTag)
	}
}

// TestFilenameDateTagSetWhenNameHasIt tests the tag is made even if the name has it, as a set op may replace that name.
func TestFilenameDateTagSetWhenNameHasIt(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "[25-01-02] v.json")
	if err := os.WriteFile(path, []byte(`{"title":"T","upload_date":"20250102"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	fd := models.NewFileData()
	fd.MetaFilePath = path
	fd.MetaDirectory = dir
	fd.MetaFileType = sharedconsts.MExtJSON
	fd.FilenameOps.DateTag = models.FOpDateTag{Loc: sharedenums.DateTagLocPrefix, DateFormat: sharedenums.DateYyMmDd}

	if err := processJSONFile(t.Context(), fd, true); err != nil {
		t.Fatal(err)
	}
	if fd.FilenameDateTag != "[25-01-02]" {
		t.Errorf("date tag = %q", fd.FilenameDateTag)
	}
}

// TestNFOFilenameDateTag tests an NFO's date makes the filename date tag, and that a missing one makes none.
func TestNFOFilenameDateTag(t *testing.T) {
	for _, tc := range []struct{ data, want string }{
		{"<movie>\n<title>T</title>\n<premiered>2025-01-02</premiered>\n</movie>\n", "[25-01-02]"},
		{"<movie>\n<title>T</title>\n</movie>\n", ""},
	} {
		path := filepath.Join(t.TempDir(), "v.nfo")
		if err := os.WriteFile(path, []byte(tc.data), 0o644); err != nil {
			t.Fatal(err)
		}
		fd := models.NewFileData()
		fd.MetaFilePath = path
		fd.MetaFileType = sharedconsts.MExtNFO
		fd.FilenameOps.DateTag = models.FOpDateTag{Loc: sharedenums.DateTagLocPrefix, DateFormat: sharedenums.DateYyMmDd}

		if err := processNFOFiles(t.Context(), fd, true); err != nil {
			t.Fatal(err)
		}
		if fd.FilenameDateTag != tc.want {
			t.Errorf("date tag = %q, want %q", fd.FilenameDateTag, tc.want)
		}
	}
}

// TestFilteredOpsReadNFOFields tests filters on an NFO go by its tag names, including every value of a repeated tag.
func TestFilteredOpsReadNFOFields(t *testing.T) {
	t.Cleanup(viper.Reset)
	const data = "<movie>\n<title>T</title>\n<originaltitle>Original</originaltitle>\n<genre>Drama</genre>\n<genre>Comedy</genre>\n</movie>\n"

	for _, tc := range []struct {
		filter string
		want   bool
	}{
		{"originaltitle:contains:orig", true},
		{"originaltitle:contains:other", false},
		{"genre:contains:comedy", true},
		{"title,originaltitle:contains:orig", true},
		{"studio:contains", false},
		{"studio:omits", true},
		{"studio:not-equals:", false},
		{"title:not-equals:", true},
	} {
		t.Run(tc.filter, func(t *testing.T) {
			filtered, _, err := sharedparsing.ParseFilteredFilenameOps([]string{tc.filter + "||prefix:matched "})
			if err != nil {
				t.Fatal(err)
			}
			abstractions.Set(keys.FilteredFilenameOpsModels, filtered)

			path := filepath.Join(t.TempDir(), "v.nfo")
			if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
				t.Fatal(err)
			}
			fd := models.NewFileData()
			fd.MetaFilePath = path
			fd.MetaFileType = sharedconsts.MExtNFO
			if err := processNFOFiles(t.Context(), fd, true); err != nil {
				t.Fatal(err)
			}
			if got := len(fd.FilenameOps.Prefixes) == 1; got != tc.want {
				t.Errorf("op applied = %v, want %v", got, tc.want)
			}
		})
	}
}

// TestBrokenMetafileLeavesBatch tests a file whose metadata can't be read goes no further, while the rest of the batch does.
func TestBrokenMetafileLeavesBatch(t *testing.T) {
	t.Cleanup(viper.Reset)
	abstractions.Set(keys.Concurrency, 1)
	abstractions.Set(keys.MaxCPU, 101.0)
	abstractions.Set(keys.InputMExts, []string{"all"})
	if err := file.InitFetchFilesVars(); err != nil {
		t.Fatal(err)
	}

	dir := t.TempDir()
	good := filepath.Join(dir, "good.json")
	if err := os.WriteFile(good, []byte(`{"title":"T"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "bad.json"), []byte(`{"title":`), 0o644); err != nil {
		t.Fatal(err)
	}
	openMeta, err := os.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer closeBatchFile(openMeta)

	b := &batch{ID: 1, JSON: dir, IsDirs: true, SkipVideos: true, bp: &batchProcessor{batchID: 1}}
	processed, err := processFiles(b, &models.Core{Ctx: t.Context(), Wg: &sync.WaitGroup{}}, nil, openMeta)
	if err != nil {
		t.Fatal(err)
	}
	if len(processed) != 1 || processed[0].MetaFilePath != good {
		t.Errorf("processed %d files, want only %s", len(processed), good)
	}
}

// TestMetaDirWithoutVideoDir tests metadata folders are batched whether or not a video folder was given too.
func TestMetaDirWithoutVideoDir(t *testing.T) {
	t.Cleanup(viper.Reset)
	videos := t.TempDir()
	meta := t.TempDir()
	extra := t.TempDir()

	abstractions.Set(keys.MetaDirs, []string{meta})
	tasks, err := initializeBatchConfigs()
	if err != nil {
		t.Fatal(err)
	}
	if len(tasks) != 1 || tasks[0].JSON != meta || !tasks[0].IsDirs || !tasks[0].SkipVideos {
		t.Errorf("metadata folder alone: %+v", tasks)
	}

	abstractions.Set(keys.VideoDirs, []string{videos})
	abstractions.Set(keys.MetaDirs, []string{meta, extra})
	if tasks, err = initializeBatchConfigs(); err != nil {
		t.Fatal(err)
	}
	if len(tasks) != 2 || tasks[0].Video != videos || tasks[0].JSON != meta || tasks[0].SkipVideos || tasks[1].JSON != extra || !tasks[1].SkipVideos {
		t.Errorf("one video folder, two metadata folders: %+v", tasks)
	}
}

// TestMetaFileWithoutVideoFile tests metafiles are batched whether or not a video file was given too.
func TestMetaFileWithoutVideoFile(t *testing.T) {
	t.Cleanup(viper.Reset)
	dir := t.TempDir()
	video := filepath.Join(dir, "v.mp4")
	meta := filepath.Join(dir, "v.json")
	extra := filepath.Join(dir, "w.json")
	for _, f := range []string{video, meta, extra} {
		if err := os.WriteFile(f, []byte("{}"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	abstractions.Set(keys.MetaFiles, []string{meta})
	tasks, err := initializeBatchConfigs()
	if err != nil {
		t.Fatal(err)
	}
	if len(tasks) != 1 || tasks[0].JSON != meta || tasks[0].IsDirs || !tasks[0].SkipVideos {
		t.Errorf("metafile alone: %+v", tasks)
	}

	abstractions.Set(keys.VideoFiles, []string{video})
	abstractions.Set(keys.MetaFiles, []string{meta, extra})
	if tasks, err = initializeBatchConfigs(); err != nil {
		t.Fatal(err)
	}
	if len(tasks) != 2 || tasks[0].Video != video || tasks[0].JSON != meta || tasks[0].SkipVideos || tasks[1].JSON != extra || !tasks[1].SkipVideos {
		t.Errorf("one video file, two metafiles: %+v", tasks)
	}
}

// TestCanceledBatchStops tests canceling stops a batch with more files than the workers' queue holds.
func TestCanceledBatchStops(t *testing.T) {
	t.Cleanup(viper.Reset)
	abstractions.Set(keys.Concurrency, 1)
	abstractions.Set(keys.InputMExts, []string{"all"})
	if err := file.InitFetchFilesVars(); err != nil {
		t.Fatal(err)
	}

	// More metafiles than the single worker's queue of two.
	dir := t.TempDir()
	for i := range 8 {
		path := filepath.Join(dir, fmt.Sprintf("v%d.json", i))
		if err := os.WriteFile(path, []byte(`{"title":"T"}`), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	openMeta, err := os.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer closeBatchFile(openMeta)

	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	b := &batch{ID: 1, JSON: dir, IsDirs: true, SkipVideos: true, bp: &batchProcessor{batchID: 1}}
	core := &models.Core{Ctx: ctx, Wg: &sync.WaitGroup{}}

	done := make(chan error, 1)
	go func() {
		_, err := processFiles(b, core, nil, openMeta)
		done <- err
	}()
	select {
	case err := <-done:
		if err != nil {
			t.Error(err)
		}
	case <-time.After(30 * time.Second):
		t.Fatal("the batch didn't stop")
	}
}

// ******************************** Video only ****************************************************************************************

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
