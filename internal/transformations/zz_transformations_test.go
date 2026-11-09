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

// ******************************** Files *********************************************************************************************

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

// ******************************** Helpers *******************************************************************************************

// TestFilenameOpsRerun tests that filename ops run again on a processed name don't stack prefixes or suffixes, and a
// date tag op replaces the tag already there.
func TestFilenameOpsRerun(t *testing.T) {
	fp := &fileProcessor{metatagParser: parsing.NewMetaTemplateParser("test.json")}
	if got := fp.prefixStrings("Title", []models.FOpPrefix{{Value: "[Cats] "}}); got != "[Cats] Title" {
		t.Errorf("first prefix = %q, want %q", got, "[Cats] Title")
	}

	const name = "[2023-11-05] [Cats] Title (HD)"
	if got := fp.prefixStrings(name, []models.FOpPrefix{{Value: "[Cats] "}}); got != name {
		t.Errorf("prefix again = %q, want %q unchanged", got, name)
	}
	if got := fp.suffixStrings(name, []models.FOpSuffix{{Value: " (HD)"}}); got != name {
		t.Errorf("suffix again = %q, want %q unchanged", got, name)
	}
	tag := models.FOpDateTag{Loc: sharedenums.DateTagLocPrefix, DateFormat: sharedenums.DateYyyyMmDd}
	if got := fp.addDateTag(name, tag, "[2024-05-01]"); got != "[2024-05-01] [Cats] Title (HD)" {
		t.Errorf("new date tag = %q, want the old one replaced", got)
	}
	if got := fp.addDateTag("[23-11-05] Title", tag, "[2023-11-05]"); got != "[2023-11-05] Title" {
		t.Errorf("date tag in a new format = %q, want the old one replaced", got)
	}
	suffix := models.FOpDateTag{Loc: sharedenums.DateTagLocSuffix, DateFormat: sharedenums.DateYyyyMmDd}
	if got := fp.addDateTag("Title [2023-11-05]", suffix, "[2024-05-01]"); got != "Title [2024-05-01]" {
		t.Errorf("new suffix date tag = %q, want the old one replaced", got)
	}
}

// TestFilenameReplaceAndDelete tests that every prefix replacement runs in order, and that deleting all date tags
// leaves the rest of the name intact.
func TestFilenameReplaceAndDelete(t *testing.T) {
	fp := &fileProcessor{metatagParser: parsing.NewMetaTemplateParser("test.json")}
	reps := []models.FOpReplacePrefix{{Prefix: "[OLD] ", Replacement: "[NEW] "}, {Prefix: "[NEW] ", Replacement: "[NEWER] "}}
	if got := fp.replacePrefix("[OLD] Title", reps); got != "[NEWER] Title" {
		t.Errorf("chained prefix replacements = %q, want both applied", got)
	}

	if got := fp.deleteDateTag("[2023-11-05] Title [Cats] [01-05-2024]"); got != "Title [Cats]" {
		t.Errorf("delete all date tags = %q, want %q", got, "Title [Cats]")
	}
}

// TestFixContractions tests that every contraction is fixed, case is kept, and non-ASCII text stays intact.
func TestFixContractions(t *testing.T) {
	tests := []struct{ in, want string }{
		{"Don t Stop", "Dont Stop"},
		{"don t stop, won t stop", "dont stop, wont stop"},
		{"İstanbul Don t Go", "İstanbul Dont Go"},
	}
	for _, tt := range tests {
		got, _, err := fixContractions(tt.in, tt.in, "video.mp4", enums.RenamingSpaces, false)
		if err != nil {
			t.Fatalf("fixContractions(%q): %v", tt.in, err)
		}
		if got != tt.want {
			t.Errorf("fixContractions(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

// ******************************** Restoring the original ****************************************************************************

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
