package file

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"metarr/internal/abstractions"
	"metarr/internal/domain/enums"
	"metarr/internal/domain/keys"
	"metarr/internal/models"

	"github.com/TubarrApp/gocommon/sharedconsts"
)

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
