package transformations

import (
	"io"
	"os"
	"path/filepath"
	"testing"

	"metarr/internal/domain/enums"
	"metarr/internal/domain/keys"
	"metarr/internal/domain/logger"
	"metarr/internal/models"
	"metarr/internal/parsing"

	"github.com/TubarrApp/gocommon/sharedconsts"
	"github.com/TubarrApp/gocommon/sharedenums"
	"github.com/spf13/viper"
)

// TestDateTagInNewName tests a date tag replaces one already there, is never stacked, and survives set replacing the name.
func TestDateTagInNewName(t *testing.T) {
	logger.Pl.Console = io.Discard
	const tag = "[25-01-02]"

	for _, tc := range []struct {
		name, base, set string
		loc             sharedenums.DateTagLocation
		want            string
	}{
		{"plain name", "old", "", sharedenums.DateTagLocPrefix, tag + " old"},
		{"set", "old", "Title", sharedenums.DateTagLocPrefix, tag + " Title"},
		{"set on a name with the tag", tag + " old", "Title", sharedenums.DateTagLocPrefix, tag + " Title"},
		{"set on a name with another tag", "[24-12-31] old", "Title", sharedenums.DateTagLocPrefix, tag + " Title"},
		{"same tag isn't stacked", tag + " old", "", sharedenums.DateTagLocPrefix, tag + " old"},
		{"same tag at the other end isn't stacked", "old " + tag, "", sharedenums.DateTagLocPrefix, "old " + tag},
		{"newer date replaces the prefix", "[24-12-31] old", "", sharedenums.DateTagLocPrefix, tag + " old"},
		{"newer date replaces the suffix", "old [24-12-31]", "", sharedenums.DateTagLocSuffix, "old " + tag},
		{"other format replaced", "[2025-01-02] old", "", sharedenums.DateTagLocPrefix, tag + " old"},
		{"date at the other end left alone", "old [24-12-31]", "", sharedenums.DateTagLocPrefix, tag + " old [24-12-31]"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			fd := models.NewFileData()
			fd.PostFFmpegVideoPath = filepath.Join(dir, tc.base+".mp4")
			fd.VideoDirectory = dir
			fd.FilenameDateTag = tag
			fd.FilenameOps = models.NewFilenameOps()
			fd.FilenameOps.DateTag = models.FOpDateTag{Loc: tc.loc, DateFormat: sharedenums.DateYyMmDd}
			if tc.set != "" {
				fd.FilenameOps.Set = models.FOpSet{IsSet: true, Value: tc.set}
			}

			fp := &fileProcessor{fd: fd, metatagParser: parsing.NewMetaTemplateParser("test.json")}
			got, err := fp.constructNewNames(tc.base, enums.RenamingSkip, fd)
			if err != nil {
				t.Fatal(err)
			}
			if got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

// TestRenameFillsTemplatesFromNFO tests filename operations read template tags from an NFO, as they do from JSON.
func TestRenameFillsTemplatesFromNFO(t *testing.T) {
	logger.Pl.Console = io.Discard
	t.Cleanup(viper.Reset)
	viper.Set(keys.FilenameOpsModels, models.NewFilenameOps())

	dir := t.TempDir()
	meta := filepath.Join(dir, "v.nfo")
	data := "<movie>\n<title>Cats &amp; Dogs</title>\n<studio>Studio</studio>\n<cast><actor><name>Actor</name></actor></cast>\n</movie>\n"
	if err := os.WriteFile(meta, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
	fd := models.NewFileData()
	fd.MetaFilePath = meta
	fd.MetaDirectory = dir
	fd.MetaFileType = sharedconsts.MExtNFO
	fd.FilenameOps = models.NewFilenameOps()
	fd.FilenameOps.Set = models.FOpSet{IsSet: true, Value: "{{title}} - {{studio}} - {{actor}}"}

	if err := RenameFiles(t.Context(), []*models.FileData{fd}); err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(dir, "Cats & Dogs - Studio - Actor.nfo")
	if _, err := os.Stat(want); err != nil {
		t.Error(err)
	}
	if fd.FinalMetaPath != want {
		t.Errorf("final metafile path = %q, want %q", fd.FinalMetaPath, want)
	}
}

// TestPurgeOnlyAfterMove tests the metafile is kept if the move fails, and purged without being moved otherwise.
func TestPurgeOnlyAfterMove(t *testing.T) {
	logger.Pl.Console = io.Discard
	t.Cleanup(viper.Reset)
	viper.Set(keys.MetaPurge, "all")
	viper.Set(keys.MetaPurgeEnum, enums.PurgeMetaAll)

	for _, tc := range []struct {
		name      string
		collision bool
	}{
		{"move fails", true},
		{"move works", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			in := t.TempDir()
			out := t.TempDir()
			viper.Set(keys.OutputDirectory, out)
			video := filepath.Join(in, "v.mp4")
			meta := filepath.Join(in, "v.json")
			for _, f := range []string{video, meta} {
				if err := os.WriteFile(f, []byte("x"), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			if tc.collision {
				if err := os.WriteFile(filepath.Join(out, "v.mp4"), []byte("taken"), 0o644); err != nil {
					t.Fatal(err)
				}
			}

			fd := &models.FileData{PostFFmpegVideoPath: video, MetaFilePath: meta}
			err := (&fileProcessor{fd: fd, outputDir: out}).process()
			if (err != nil) != tc.collision {
				t.Errorf("process: %v", err)
			}

			_, metaErr := os.Stat(meta)
			if (metaErr == nil) != tc.collision {
				t.Errorf("metafile still there: %v, want %v", metaErr == nil, tc.collision)
			}
			if _, err := os.Stat(filepath.Join(out, "v.json")); err == nil {
				t.Error("metafile was moved")
			}
			if !tc.collision && fd.FinalMetaPath != "" {
				t.Errorf("final metafile path = %q, want none", fd.FinalMetaPath)
			}
		})
	}
}

// TestRenameMetafileWithoutVideo tests a metafile with no video is still renamed, without the skip videos setting.
func TestRenameMetafileWithoutVideo(t *testing.T) {
	logger.Pl.Console = io.Discard
	t.Cleanup(viper.Reset)
	viper.Set(keys.FilenameOpsModels, models.NewFilenameOps())

	dir := t.TempDir()
	meta := filepath.Join(dir, "v.json")
	if err := os.WriteFile(meta, []byte(`{"title":"T"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	fd := models.NewFileData()
	fd.MetaFilePath = meta
	fd.MetaDirectory = dir
	fd.MetaFileType = sharedconsts.MExtJSON
	fd.FilenameOps = models.NewFilenameOps()
	fd.FilenameOps.Prefixes = []models.FOpPrefix{{Value: "new "}}

	if err := RenameFiles(t.Context(), []*models.FileData{fd}); err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(dir, "new v.json")
	if _, err := os.Stat(want); err != nil {
		t.Error(err)
	}
	if fd.FinalMetaPath != want {
		t.Errorf("final metafile path = %q, want %q", fd.FinalMetaPath, want)
	}
}

// TestRenameVideoWithoutMetafile tests a video with no metafile is renamed and moved, reports no metafile, and a purge passes it over.
func TestRenameVideoWithoutMetafile(t *testing.T) {
	logger.Pl.Console = io.Discard
	t.Cleanup(viper.Reset)
	viper.Set(keys.FilenameOpsModels, models.NewFilenameOps())
	viper.Set(keys.MetaPurge, "all")
	viper.Set(keys.MetaPurgeEnum, enums.PurgeMetaAll)

	for _, tc := range []struct {
		name, prefix, want string
	}{
		{"plain prefix", "new ", "new v.mp4"},
		{"template prefix keeps the name", "{{title}} ", "v.mp4"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			in := t.TempDir()
			out := t.TempDir()
			viper.Set(keys.OutputDirectory, out)
			video := filepath.Join(in, "v.mp4")
			if err := os.WriteFile(video, []byte("x"), 0o644); err != nil {
				t.Fatal(err)
			}
			fd := models.NewFileData()
			fd.OriginalVideoPath = video
			fd.PostFFmpegVideoPath = video
			fd.VideoDirectory = in
			fd.FilenameOps = models.NewFilenameOps()
			fd.FilenameOps.Prefixes = []models.FOpPrefix{{Value: tc.prefix}}

			if err := RenameFiles(t.Context(), []*models.FileData{fd}); err != nil {
				t.Fatal(err)
			}
			want := filepath.Join(out, tc.want)
			if _, err := os.Stat(want); err != nil {
				t.Error(err)
			}
			if fd.FinalVideoPath != want || fd.FinalMetaPath != "" || fd.RenamedMetaPath != "" {
				t.Errorf("final video %q, final metafile %q, renamed metafile %q; want %q and no metafile", fd.FinalVideoPath, fd.FinalMetaPath, fd.RenamedMetaPath, want)
			}
		})
	}
}

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

// TestExistingPath tests that the first path on disk is chosen, so a failed move reports where files are.
func TestExistingPath(t *testing.T) {
	dir := t.TempDir()
	current := filepath.Join(dir, "v.mp4")
	if err := os.WriteFile(current, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	moved := filepath.Join(dir, "out", "v.mp4")

	if got := existingPath(moved, current); got != current {
		t.Errorf("existingPath(unmoved) = %q, want %q", got, current)
	}
	if got := existingPath("", current); got != current {
		t.Errorf("existingPath with empty first = %q, want %q", got, current)
	}
	if got := existingPath(moved, filepath.Join(dir, "gone.mp4")); got != "" {
		t.Errorf("existingPath(none present) = %q, want empty", got)
	}
}
