package ffmpeg

import (
	"io"
	"maps"
	"os"
	"slices"
	"strings"
	"testing"

	"metarr/internal/domain/consts"
	"metarr/internal/domain/logger"
	"metarr/internal/models"
	"metarr/internal/parsing"

	"github.com/TubarrApp/gocommon/sharedconsts"
	"github.com/TubarrApp/gocommon/sharedtags"
)

// TestMain gives the global logger a console writer, since the zero value has none.
func TestMain(m *testing.M) {
	logger.Pl.Console = io.Discard
	os.Exit(m.Run())
}

// Tags FFmpeg renames while writing each container, by lowercase name (from libavformat's conversion tables).
var (
	asfRenames = map[string]string{
		"album_artist": "wm/albumartist", "album": "wm/albumtitle", "artist": "author", "comment": "description",
		"composer": "wm/composer", "encoded_by": "wm/encodedby", "encoder": "wm/encodingsettings", "genre": "wm/genre",
		"language": "wm/language", "filename": "wm/originalfilename", "disc": "wm/partofset",
		"disc_subtitle": "wm/setsubtitle", "publisher": "wm/publisher", "track": "wm/tracknumber",
		"service_provider": "wm/mediastationcallsign", "service_name": "wm/mediastationname",
	}
	aviRenames = map[string]string{
		"artist": "iart", "comment": "icmt", "copyright": "icop", "date": "icrd", "genre": "ignr", "language": "ilng",
		"title": "inam", "album": "iprd", "track": "iprt", "encoder": "isft", "timecode": "ismp", "encoded_by": "itch",
	}
	matroskaRenames = map[string]string{"performer": "lead_performer", "track": "part_number"}
	oggRenames      = map[string]string{
		"album_artist": "albumartist", "track": "tracknumber", "disc": "discnumber", "disc_subtitle": "discsubtitle",
		"comment": "description",
	}
	ffmpegRenames = map[string]map[string]string{
		sharedconsts.ExtASF:  asfRenames,
		sharedconsts.ExtWMV:  asfRenames,
		sharedconsts.ExtAVI:  aviRenames,
		sharedconsts.ExtMKV:  matroskaRenames,
		sharedconsts.ExtWEBM: matroskaRenames,
		sharedconsts.ExtOGM:  oggRenames,
		sharedconsts.ExtOGV:  oggRenames,
	}
)

// fullFileData returns file data with every field the FFmpeg command writes filled in.
func fullFileData() *models.FileData {
	return &models.FileData{
		MTitleDesc: &models.MetadataTitlesDescs{
			Title: "title", Subtitle: "subtitle", Description: "description", LongDescription: "long",
			Synopsis: "synopsis", Summary: "summary", Comment: "comment",
		},
		MCredits: &models.MetadataCredits{
			Actor: "actor", Author: "author", Artist: "artist", AlbumArtist: "album artist", Creator: "creator",
			Studio: "studio", Publisher: "publisher", Producer: "producer", Performer: "performer",
			Composer: "composer", Director: "director", Writer: "writer",
		},
		MDates: &models.MetadataDates{
			CreationTime: "2024-01-02T03:04:05Z", Date: "2024-01-02", OriginallyAvailableAt: "2024-01-02",
			ReleaseDate: "2024-01-02", UploadDate: "2024-01-02", Year: "2024",
		},
		MShowData: &models.MetadataShowData{
			Show: "show", EpisodeID: "S01E02", EpisodeSort: "2", SeasonNumber: "1", SeasonTitle: "season",
			Network: "network",
		},
		MOther: &models.MetadataOtherData{
			Language: "en", Genre: "genre", HDVideo: "2", Album: "album", Track: "3", Disc: "1", Keywords: "a, b",
			Copyright: "copyright",
		},
		MWebData: &models.MetadataWebData{WebpageURL: "https://example.com/watch"},
		MetaOps:  models.NewMetaOps(),
	}
}

// TestContainerTagsUnique tests that no two fields are written as the same tag, including tags FFmpeg renames itself.
func TestContainerTagsUnique(t *testing.T) {
	keys := slices.Sorted(maps.Keys(MetadataTags(fullFileData())))
	for ext := range sharedconsts.AllVidExtensions {
		seen := make(map[string]string, len(keys))
		for _, key := range keys {
			tag := strings.ToLower(parsing.GetContainerKeys(key, ext))
			if tag == "" {
				continue
			}
			if renamed, ok := ffmpegRenames[ext][tag]; ok {
				tag = renamed
			}
			if other, dup := seen[tag]; dup {
				t.Errorf("%s: %q and %q are both written as %q", ext, other, key, tag)
			}
			seen[tag] = key
		}
	}
}

// TestMetadataTagsHoldsEveryField tests that every filled field reaches the FFmpeg command's metadata.
func TestMetadataTagsHoldsEveryField(t *testing.T) {
	tags := MetadataTags(fullFileData())
	for _, key := range []string{
		sharedtags.JAlbumArtist, sharedtags.JNetwork, sharedtags.JAlbum, sharedtags.JCopyright, sharedtags.JDiscNumber,
		sharedtags.JGenre, sharedtags.JHDVideo, sharedtags.JKeywords, sharedtags.JLanguage, sharedtags.JTrackNumber,
		sharedtags.JWebpageURL, sharedtags.JShow, sharedtags.JSeasonNumber, sharedtags.JEpisodeID,
	} {
		if tags[key] == "" {
			t.Errorf("MetadataTags has no %q", key)
		}
	}
}

// TestFinalCommandMetadata tests that fields a container can't hold are left out, and the muxer is named where needed.
func TestFinalCommandMetadata(t *testing.T) {
	tests := []struct {
		ext          string
		wantMuxer    string
		want, absent []string
	}{
		{ext: ".mp4", want: []string{"show=show", "album_artist=album artist", "synopsis=synopsis"}},
		{ext: ".mov", want: []string{"comment=long"}, absent: []string{"synopsis=", "author=", "show="}},
		{ext: ".ogm", wantMuxer: consts.FFmpegMuxerOGV, want: []string{"DESCRIPTION=long"}, absent: []string{"comment="}},
		{ext: ".rmvb", wantMuxer: consts.FFmpegMuxerRM, want: []string{"Comment=long"}, absent: []string{"show="}},
		{ext: ".MKV", want: []string{"ACTOR=actor", "LEAD_PERFORMER=performer", "season_title=season"}},
	}

	for _, tt := range tests {
		t.Run(tt.ext, func(t *testing.T) {
			b := &ffCommandBuilder{
				builder:     &strings.Builder{},
				inputFile:   "in.mkv",
				outputFile:  "out" + tt.ext,
				metadataMap: MetadataTags(fullFileData()),
			}
			args, err := b.buildFinalCommand(nil, false)
			if err != nil {
				t.Fatal(err)
			}

			// Check the metadata arguments.
			for _, w := range tt.want {
				if !slices.Contains(args, w) {
					t.Errorf("args lack %q: %v", w, args)
				}
			}
			for _, a := range tt.absent {
				if slices.ContainsFunc(args, func(arg string) bool { return strings.HasPrefix(arg, a) }) {
					t.Errorf("args hold %q: %v", a, args)
				}
			}

			// Check the muxer, and that the output file stays last.
			gotMuxer := ""
			if i := slices.Index(args, consts.FFmpegFormat); i >= 0 {
				gotMuxer = args[i+1]
			}
			if gotMuxer != tt.wantMuxer {
				t.Errorf("muxer = %q, want %q", gotMuxer, tt.wantMuxer)
			}
			if args[len(args)-1] != b.outputFile {
				t.Errorf("last arg = %q, want the output file", args[len(args)-1])
			}
		})
	}
}
