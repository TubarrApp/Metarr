package file

import (
	"io"
	iofs "io/fs"
	"metarr/internal/domain/logger"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

// TestMoveVideoNFO tests the NFO file named for a video follows it, with both JSON and NFO metafiles.
func TestMoveVideoNFO(t *testing.T) {
	logger.Pl.Console = io.Discard

	for _, tc := range []struct {
		name                      string
		files                     []string // Made in the "in" folder.
		video, renamedVideo       string   // Relative to the test's folder.
		meta, renamedMeta         string
		noMeta, skipVids          bool
		want                      []string
		movedFrom, movedTo        string // A file which must keep its contents when moved.
		untouched                 string // A file which must keep its contents where it is.
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
