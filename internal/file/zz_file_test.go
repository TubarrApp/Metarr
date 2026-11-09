package file

import (
	"io"
	iofs "io/fs"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"metarr/internal/abstractions"
	"metarr/internal/domain/enums"
	"metarr/internal/domain/keys"
	"metarr/internal/domain/logger"
	"metarr/internal/models"

	"github.com/TubarrApp/gocommon/sharedconsts"
)

// ******************************** Write helpers *************************************************************************************

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

// writeFile writes content to name in dir and returns its path.
func writeFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

// TestCopyFileKeepsExistingDest tests that a failed copy onto an existing file leaves that file untouched.
func TestCopyFileKeepsExistingDest(t *testing.T) {
	tmp := t.TempDir()
	src := writeFile(t, tmp, "src.mp4", "new")
	dst := writeFile(t, tmp, "dst.mp4", "existing")

	if err := copyFile(src, dst); err == nil {
		t.Fatal("copyFile onto an existing file succeeded, want an error")
	}
	if got, _ := os.ReadFile(dst); string(got) != "existing" {
		t.Fatalf("destination = %q, want it unchanged", got)
	}
}

// TestMoveOrCopyFileRefusesOverwrite tests that a move never replaces an existing destination.
func TestMoveOrCopyFileRefusesOverwrite(t *testing.T) {
	tmp := t.TempDir()
	src := writeFile(t, tmp, "src.mp4", "new")
	dst := writeFile(t, tmp, "dst.mp4", "existing")

	if err := moveOrCopyFile(src, dst); err == nil {
		t.Fatal("moveOrCopyFile onto an existing file succeeded, want an error")
	}
	if got, _ := os.ReadFile(dst); string(got) != "existing" {
		t.Fatalf("destination = %q, want it unchanged", got)
	}
	if _, err := os.Stat(src); err != nil {
		t.Fatalf("source removed: %v", err)
	}
}

// TestReplaceContents tests that the open handle reads the new contents after a replace.
func TestReplaceContents(t *testing.T) {
	p := writeFile(t, t.TempDir(), "meta.json", `{"title":"old"}`)
	f, err := os.OpenFile(p, os.O_RDWR, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()

	if err := ReplaceContents(f, []byte(`{"title":"new"}`)); err != nil {
		t.Fatalf("ReplaceContents: %v", err)
	}
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		t.Fatal(err)
	}
	got, err := io.ReadAll(f)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != `{"title":"new"}` {
		t.Fatalf("handle reads %q, want the new contents", got)
	}
	if entries, _ := os.ReadDir(filepath.Dir(p)); len(entries) != 1 {
		t.Fatalf("directory holds %d entries, want only the replaced file", len(entries))
	}
}

// TestNextBackupPath tests that an identical backup is reused and a differing one gets a numbered sibling.
func TestNextBackupPath(t *testing.T) {
	tmp := t.TempDir()
	orig := writeFile(t, tmp, "video.mp4", "v1")

	first, err := nextBackupPath(orig)
	if err != nil || first == "" {
		t.Fatalf("nextBackupPath with no backups = %q, %v; want a path", first, err)
	}
	writeFile(t, tmp, filepath.Base(first), "v1")

	if p, err := nextBackupPath(orig); err != nil || p != "" {
		t.Fatalf("nextBackupPath with an identical backup = %q, %v; want \"\"", p, err)
	}

	if err := os.WriteFile(orig, []byte("v2"), 0o644); err != nil {
		t.Fatal(err)
	}
	second, err := nextBackupPath(orig)
	if err != nil || second == "" || second == first {
		t.Fatalf("nextBackupPath with a differing backup = %q, %v; want a new numbered path", second, err)
	}
}

// ******************************** Long names ****************************************************************************************

// TestReplaceContentsLongName tests a file named up to the filesystem's limit can be replaced, leaving no temp file.
func TestReplaceContentsLongName(t *testing.T) {
	dir := t.TempDir()
	name := strings.Repeat("a", sharedconsts.MaxNameBytes-len(sharedconsts.MExtJSON)) + sharedconsts.MExtJSON
	p := writeFile(t, dir, name, `{"title":"old"}`)
	f, err := os.OpenFile(p, os.O_RDWR, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()

	if err := ReplaceContents(f, []byte(`{"title":"new"}`)); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != `{"title":"new"}` {
		t.Errorf("file holds %q", got)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Errorf("%d files in the folder, want only the replaced one", len(entries))
	}
}

// TestRenameToBackupLongName tests a file named up to the filesystem's limit can be backed up, more than once.
func TestRenameToBackupLongName(t *testing.T) {
	dir := t.TempDir()
	name := strings.Repeat("a", sharedconsts.MaxNameBytes-len(sharedconsts.ExtMP4)) + sharedconsts.ExtMP4

	seen := make(map[string]bool)
	for _, contents := range []string{"first", "second"} {
		p := writeFile(t, dir, name, contents)
		backup, err := RenameToBackup(p)
		if err != nil {
			t.Fatal(err)
		}
		base := filepath.Base(backup)
		if len(base) > sharedconsts.MaxNameBytes || !strings.Contains(base, sharedconsts.BackupTag) || filepath.Ext(base) != sharedconsts.ExtMP4 {
			t.Errorf("backup name %q (%d bytes)", base, len(base))
		}
		got, err := os.ReadFile(backup)
		if err != nil {
			t.Fatal(err)
		}
		if string(got) != contents {
			t.Errorf("backup holds %q, want %q", got, contents)
		}
		if seen[backup] {
			t.Errorf("backup path %q used twice", backup)
		}
		seen[backup] = true
	}
}

// TestPurgesMetafile tests the purge setting covers the metafiles it names and no others.
func TestPurgesMetafile(t *testing.T) {
	t.Cleanup(func() { abstractions.Set(keys.MetaPurgeEnum, nil) })
	fsWriter, err := NewFSFileWriter(&models.FileData{PostFFmpegVideoPath: "/videos/v.mp4", MetaFilePath: "/videos/v.json"}, false, "")
	if err != nil {
		t.Fatal(err)
	}

	for _, tc := range []struct {
		setting   enums.PurgeMetafiles
		json, nfo bool
	}{
		{enums.PurgeMetaAll, true, true},
		{enums.PurgeMetaJSON, true, false},
		{enums.PurgeMetaNFO, false, true},
		{enums.PurgeMetaNone, false, false},
	} {
		abstractions.Set(keys.MetaPurgeEnum, tc.setting)
		for file, want := range map[string]bool{"v.json": tc.json, "V.JSON": tc.json, "v.nfo": tc.nfo} {
			if got, err := fsWriter.PurgesMetafile(file); err != nil || got != want {
				t.Errorf("setting %v, %s: got %v, %v; want %v", tc.setting, file, got, err, want)
			}
		}
	}
}

// ******************************** Matching files ************************************************************************************

// testFileMaps returns video and metafile maps for the names, as the directory readers make them.
func testFileMaps(videos, metas []string) (videoFiles, metaFiles map[string]*models.FileData) {
	videoFiles = make(map[string]*models.FileData, len(videos))
	for _, name := range videos {
		videoFiles[name] = &models.FileData{OriginalVideoPath: filepath.Join("/videos", name)}
	}
	metaFiles = make(map[string]*models.FileData, len(metas))
	for _, name := range metas {
		metaFiles[name] = &models.FileData{MetaFilePath: filepath.Join("/videos", name), MetaFileType: sharedconsts.MExtJSON}
	}
	return videoFiles, metaFiles
}

// TestMatchVideoWithMetadata tests each video gets its closest metafile, and no metafile goes to two videos.
func TestMatchVideoWithMetadata(t *testing.T) {
	for _, tc := range []struct {
		name          string
		videos, metas []string
		want          map[string]string
	}{
		{"same name", []string{"v.mp4"}, []string{"v.json"}, map[string]string{"v.mp4": "v.json"}},
		{"names differing by case", []string{"Foo.mp4", "foo.mp4"}, []string{"Foo.json", "foo.json"}, map[string]string{"Foo.mp4": "Foo.json", "foo.mp4": "foo.json"}},
		{"one metafile for names differing by case", []string{"Foo.mp4", "foo.mp4"}, []string{"foo.json"}, map[string]string{"foo.mp4": "foo.json"}},
		{"known suffix", []string{"v.mp4"}, []string{"v.info.json"}, map[string]string{"v.mp4": "v.info.json"}},
		{"known suffix in the video's name", []string{"x.metadata.mp4"}, []string{"x.metadata.json"}, map[string]string{"x.metadata.mp4": "x.metadata.json"}},
		{"with and without a known suffix", []string{"x.metadata.mp4", "x.mp4"}, []string{"x.metadata.json", "x.json"}, map[string]string{"x.metadata.mp4": "x.metadata.json", "x.mp4": "x.json"}},
		{"one metafile goes to the same name, not the name without its suffix", []string{"x.metadata.mp4", "x.mp4"}, []string{"x.metadata.json"}, map[string]string{"x.metadata.mp4": "x.metadata.json"}},
		{"metafile's whole name", []string{"file.json.mp4"}, []string{"file.json"}, map[string]string{"file.json.mp4": "file.json"}},
		{"different case only", []string{"My Video.mp4"}, []string{"my video.json"}, map[string]string{"My Video.mp4": "my video.json"}},
		{"unmatched video left out", []string{"v.mp4", "w.mp4"}, []string{"v.json"}, map[string]string{"v.mp4": "v.json"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			videoFiles, metaFiles := testFileMaps(tc.videos, tc.metas)
			matched, err := MatchVideoWithMetadata(videoFiles, metaFiles, 1)
			if err != nil {
				t.Fatal(err)
			}
			got := make(map[string]string, len(matched))
			for video, fd := range matched {
				got[video] = filepath.Base(fd.MetaFilePath)
			}
			if !maps.Equal(got, tc.want) {
				t.Errorf("got %v, want %v", got, tc.want)
			}
		})
	}
}

// TestMatchVideoWithMetadataNone tests it's an error if nothing matches, and that a nil video entry is passed over.
func TestMatchVideoWithMetadataNone(t *testing.T) {
	videoFiles, metaFiles := testFileMaps([]string{"v.mp4"}, []string{"other.json"})
	if _, err := MatchVideoWithMetadata(videoFiles, metaFiles, 1); err == nil {
		t.Error("no matches gave no error")
	}

	videoFiles, metaFiles = testFileMaps([]string{"v.mp4"}, []string{"v.json"})
	videoFiles["nil.mp4"] = nil
	matched, err := MatchVideoWithMetadata(videoFiles, metaFiles, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(matched) != 1 || matched["v.mp4"] == nil {
		t.Errorf("matched = %v, want only v.mp4", matched)
	}
}

// ******************************** Moving video and NFO ******************************************************************************

// TestMoveVideoNFO tests the NFO file named for a video follows it, with both JSON and NFO metafiles.
func TestMoveVideoNFO(t *testing.T) {
	logger.Pl.Console = io.Discard

	for _, tc := range []struct {
		name                string
		files               []string // Made in the "in" folder.
		video, renamedVideo string   // Relative to the test's folder.
		meta, renamedMeta   string
		noMeta, skipVids    bool
		want                []string
		movedFrom, movedTo  string // A file which must keep its contents when moved.
		untouched           string // A file which must keep its contents where it is.
	}{
		{
			name:         "renamed with a JSON metafile",
			files:        []string{"clip.mp4", "clip.json", "clip.nfo"},
			video:        "in/clip.mp4",
			renamedVideo: "in/Renamed Clip.mp4",
			meta:         "in/clip.json",
			renamedMeta:  "in/Renamed Clip.json",
			want:         []string{"in/Renamed Clip.json", "in/Renamed Clip.mp4", "in/Renamed Clip.nfo"},
			movedFrom:    "clip.nfo",
			movedTo:      "in/Renamed Clip.nfo",
		},
		{
			name:         "moved to the output directory",
			files:        []string{"clip.mp4", "clip.json", "clip.nfo"},
			video:        "in/clip.mp4",
			renamedVideo: "out/clip.mp4",
			meta:         "in/clip.json",
			renamedMeta:  "out/clip.json",
			want:         []string{"out/clip.json", "out/clip.mp4", "out/clip.nfo"},
			movedFrom:    "clip.nfo",
			movedTo:      "out/clip.nfo",
		},
		{
			name:         "only the last dot starts the extension, and similar names stay",
			files:        []string{"Ep. 5 - Cats.mp4", "Ep. 5 - Cats.json", "Ep. 5 - Cats.nfo", "Ep.nfo", "Ep. 5 - Cats.mp4.nfo"},
			video:        "in/Ep. 5 - Cats.mp4",
			renamedVideo: "in/Ep. 6 - Dogs.mp4",
			meta:         "in/Ep. 5 - Cats.json",
			renamedMeta:  "in/Ep. 6 - Dogs.json",
			want:         []string{"in/Ep. 5 - Cats.mp4.nfo", "in/Ep. 6 - Dogs.json", "in/Ep. 6 - Dogs.mp4", "in/Ep. 6 - Dogs.nfo", "in/Ep.nfo"},
			movedFrom:    "Ep. 5 - Cats.nfo",
			movedTo:      "in/Ep. 6 - Dogs.nfo",
		},
		{
			name:         "the NFO file is the metafile",
			files:        []string{"clip.mp4", "clip.nfo"},
			video:        "in/clip.mp4",
			renamedVideo: "in/Renamed Clip.mp4",
			meta:         "in/clip.nfo",
			renamedMeta:  "in/Renamed Clip.nfo",
			want:         []string{"in/Renamed Clip.mp4", "in/Renamed Clip.nfo"},
			movedFrom:    "clip.nfo",
			movedTo:      "in/Renamed Clip.nfo",
		},
		{
			name:         "the NFO file is a metafile being purged, which stays to be deleted",
			files:        []string{"clip.mp4", "clip.nfo"},
			video:        "in/clip.mp4",
			renamedVideo: "in/Renamed Clip.mp4",
			meta:         "in/clip.nfo",
			renamedMeta:  "in/Renamed Clip.nfo",
			noMeta:       true,
			want:         []string{"in/Renamed Clip.mp4", "in/clip.nfo"},
			untouched:    "in/clip.nfo",
		},
		{
			name:         "another NFO metafile takes the name first",
			files:        []string{"clip.mp4", "clip.nfo", "other.nfo"},
			video:        "in/clip.mp4",
			renamedVideo: "in/Renamed Clip.mp4",
			meta:         "in/other.nfo",
			renamedMeta:  "in/Renamed Clip.nfo",
			want:         []string{"in/Renamed Clip.mp4", "in/Renamed Clip.nfo", "in/clip.nfo"},
			movedFrom:    "other.nfo",
			movedTo:      "in/Renamed Clip.nfo",
			untouched:    "in/clip.nfo",
		},
		{
			name:         "no NFO file",
			files:        []string{"clip.mp4", "clip.json"},
			video:        "in/clip.mp4",
			renamedVideo: "in/Renamed Clip.mp4",
			meta:         "in/clip.json",
			renamedMeta:  "in/Renamed Clip.json",
			want:         []string{"in/Renamed Clip.json", "in/Renamed Clip.mp4"},
		},
		{
			name:         "the new name is already taken, so neither NFO file is lost",
			files:        []string{"clip.mp4", "clip.json", "clip.nfo", "Renamed Clip.nfo"},
			video:        "in/clip.mp4",
			renamedVideo: "in/Renamed Clip.mp4",
			meta:         "in/clip.json",
			renamedMeta:  "in/Renamed Clip.json",
			want:         []string{"in/Renamed Clip.json", "in/Renamed Clip.mp4", "in/Renamed Clip.nfo", "in/clip.nfo"},
			untouched:    "in/clip.nfo",
		},
		{
			name:         "the video isn't moved",
			files:        []string{"clip.mp4", "clip.json", "clip.nfo"},
			video:        "in/clip.mp4",
			renamedVideo: "in/Renamed Clip.mp4",
			meta:         "in/clip.json",
			renamedMeta:  "in/Renamed Clip.json",
			skipVids:     true,
			want:         []string{"in/Renamed Clip.json", "in/clip.mp4", "in/clip.nfo"},
			untouched:    "in/clip.nfo",
		},
		{
			name:         "the video keeps its path",
			files:        []string{"clip.mp4", "clip.json", "clip.nfo"},
			video:        "in/clip.mp4",
			renamedVideo: "in/clip.mp4",
			meta:         "in/clip.json",
			renamedMeta:  "in/clip.json",
			want:         []string{"in/clip.json", "in/clip.mp4", "in/clip.nfo"},
			untouched:    "in/clip.nfo",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			for _, dir := range []string{"in", "out"} {
				if err := os.Mkdir(filepath.Join(root, dir), 0o750); err != nil {
					t.Fatal(err)
				}
			}
			for _, name := range tc.files {
				if err := os.WriteFile(filepath.Join(root, "in", name), []byte(name), 0o600); err != nil {
					t.Fatal(err)
				}
			}

			w := &FSFileWriter{
				InputVideo:   filepath.Join(root, tc.video),
				RenamedVideo: filepath.Join(root, tc.renamedVideo),
				InputMeta:    filepath.Join(root, tc.meta),
				RenamedMeta:  filepath.Join(root, tc.renamedMeta),
				SkipVids:     tc.skipVids,
			}
			if err := w.RenameAndMoveFiles(tc.noMeta); err != nil {
				t.Fatal(err)
			}

			var got []string
			err := filepath.WalkDir(root, func(path string, d iofs.DirEntry, err error) error {
				if err != nil || d.IsDir() {
					return err
				}
				rel, err := filepath.Rel(root, path)
				got = append(got, filepath.ToSlash(rel))
				return err
			})
			if err != nil {
				t.Fatal(err)
			}
			slices.Sort(got)
			if !slices.Equal(got, tc.want) {
				t.Errorf("files:\ngot  %q\nwant %q", got, tc.want)
			}

			// Moved rather than made again, and left alone where it should be.
			if tc.movedTo != "" {
				if data, err := os.ReadFile(filepath.Join(root, tc.movedTo)); err != nil || string(data) != tc.movedFrom {
					t.Errorf("%s holds %q (%v), want the contents of %s", tc.movedTo, data, err, tc.movedFrom)
				}
			}
			if tc.untouched != "" {
				if data, err := os.ReadFile(filepath.Join(root, tc.untouched)); err != nil || string(data) != filepath.Base(tc.untouched) {
					t.Errorf("%s holds %q (%v), want its own contents", tc.untouched, data, err)
				}
			}
		})
	}
}
