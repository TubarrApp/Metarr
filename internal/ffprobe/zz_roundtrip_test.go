package ffprobe

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"metarr/internal/parsing"

	"github.com/TubarrApp/gocommon/sharedconsts"
	"github.com/TubarrApp/gocommon/sharedtags"
)

// roundTripTags holds a value for every field the FFmpeg command writes.
var roundTripTags = map[string]string{
	sharedtags.JTitle: "Title", sharedtags.JSubtitle: "Subtitle", sharedtags.JComment: "Comment",
	sharedtags.JDescription: "Description", sharedtags.JLongDesc: "Description", sharedtags.JSummary: "Summary",
	sharedtags.JSynopsis: "Synopsis", sharedtags.JActor: "Actor", sharedtags.JAuthor: "Author",
	sharedtags.JArtist: "Artist", sharedtags.JAlbumArtist: "Album Artist", sharedtags.JCreator: "Creator",
	sharedtags.JStudio: "Studio", sharedtags.JPublisher: "Publisher", sharedtags.JProducer: "Producer",
	sharedtags.JPerformer: "Performer", sharedtags.JComposer: "Composer", sharedtags.JDirector: "Director",
	sharedtags.JWriter: "Writer", sharedtags.JCreationTime: "2024-01-02T03:04:05Z", sharedtags.JDate: "2024-01-02",
	sharedtags.JOriginallyAvailable: "2024-01-02", sharedtags.JReleaseDate: "2024-01-02",
	sharedtags.JUploadDate: "2024-01-02", sharedtags.JYear: "2024", sharedtags.JEpisodeID: "S01E02",
	sharedtags.JEpisodeSort: "2", sharedtags.JNetwork: "Network", sharedtags.JSeasonNumber: "1",
	sharedtags.JSeasonTitle: "Season", sharedtags.JShow: "Show", sharedtags.JAlbum: "Album",
	sharedtags.JCopyright: "Copyright", sharedtags.JDiscNumber: "1", sharedtags.JGenre: "Genre",
	sharedtags.JHDVideo: "2", sharedtags.JKeywords: "a, b", sharedtags.JLanguage: "eng",
	sharedtags.JTrackNumber: "3", sharedtags.JWebpageURL: "https://example.com/watch",
}

// TestRoundTripWithFFmpeg writes every container with the installed FFmpeg and checks FFprobe reads each tag back.
func TestRoundTripWithFFmpeg(t *testing.T) {
	if os.Getenv("METARR_FFMPEG_ROUNDTRIP") == "" {
		t.Skip("set METARR_FFMPEG_ROUNDTRIP=1 to run against the installed FFmpeg")
	}

	// Make a short source clip.
	dir := t.TempDir()
	src := filepath.Join(dir, "src.mkv")
	if out, err := exec.Command("ffmpeg", "-v", "error", "-f", "lavfi", "-i", "color=s=176x144:d=0.2", src).CombinedOutput(); err != nil {
		t.Fatalf("making source clip: %v\n%s", err, out)
	}
	tags := roundTripTags

	for ext := range sharedconsts.AllVidExtensions {
		t.Run(ext, func(t *testing.T) {
			if ext == sharedconsts.Ext3GP || ext == sharedconsts.Ext3G2 {
				t.Skip("FFprobe can't read 3GP tags back")
			}

			// Write the container with every tag it can hold, as the FFmpeg command does.
			out := filepath.Join(dir, "out"+ext)
			args := []string{"-v", "error", "-y", "-i", src}
			for key, value := range tags {
				if tag := parsing.GetContainerKeys(key, ext); tag != "" {
					args = append(args, "-metadata", tag+"="+value)
				}
			}
			switch ext {
			case sharedconsts.ExtOGM:
				args = append(args, "-f", "ogv")
			case sharedconsts.ExtRMVB:
				args = append(args, "-f", "rm")
			}
			if output, err := exec.Command("ffmpeg", append(args, out)...).CombinedOutput(); err != nil {
				t.Skipf("FFmpeg can't write %s here: %v\n%s", ext, err, output)
			}

			// Read it back and compare.
			probed, err := exec.Command("ffprobe", "-v", "quiet", "-print_format", "json", "-show_format", "-show_streams", "-show_programs", out).Output()
			if err != nil {
				t.Fatalf("ffprobe: %v", err)
			}
			var ffData ffprobeOutput
			if err := json.Unmarshal(probed, &ffData); err != nil {
				t.Fatal(err)
			}
			for tag, d := range getDiffMapForFiletype(ext, tags, ffData) {
				if d.existing != d.new {
					t.Errorf("%q read back as %q, want %q", tag, d.existing, d.new)
				}
			}
		})
	}
}
