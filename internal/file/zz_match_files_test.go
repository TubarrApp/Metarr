package file

import (
	"maps"
	"path/filepath"
	"testing"

	"metarr/internal/models"

	"github.com/TubarrApp/gocommon/sharedconsts"
)

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
