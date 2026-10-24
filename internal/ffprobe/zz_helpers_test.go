package ffprobe

import (
	"testing"

	"github.com/TubarrApp/gocommon/sharedtags"
)

// TestDiffMapReadsProbeNames tests that each written tag is compared with the name FFprobe reports it under.
func TestDiffMapReadsProbeNames(t *testing.T) {
	tests := []struct {
		name  string
		ext   string
		tags  map[string]string
		probe ffprobeOutput
	}{
		{
			name: "avi tags come back under FFmpeg's names",
			ext:  ".avi",
			tags: map[string]string{
				sharedtags.JArtist: "artist", sharedtags.JDescription: "desc", sharedtags.JReleaseDate: "2024-01-02",
				sharedtags.JSynopsis: "syn", sharedtags.JTrackNumber: "3", sharedtags.JAlbum: "album",
			},
			probe: probeTags(ffprobeTags{
				"artist": "artist", "comment": "desc", "date": "2024-01-02", "subject": "syn", "track": "3", "product": "album",
			}),
		},
		{
			name: "asf tags come back converted, or under their Windows Media names",
			ext:  ".wmv",
			tags: map[string]string{
				sharedtags.JArtist: "artist", sharedtags.JComment: "comment", sharedtags.JComposer: "composer",
				sharedtags.JYear: "2024", sharedtags.JShow: "show",
			},
			probe: probeTags(ffprobeTags{
				"artist": "artist", "comment": "comment", "composer": "composer", "WM/Year": "2024", "show": "show",
			}),
		},
		{
			name: "matroska tags come back in capitals, the performer and part converted",
			ext:  ".mkv",
			tags: map[string]string{
				sharedtags.JActor: "actor", sharedtags.JPerformer: "performer", sharedtags.JTrackNumber: "3",
				sharedtags.JTitle: "title", sharedtags.JSeasonTitle: "season",
			},
			probe: probeTags(ffprobeTags{
				"ACTOR": "actor", "performer": "performer", "track": "3", "title": "title", "SEASON_TITLE": "season",
			}),
		},
		{
			name: "ogg tags come back on the stream, the description as the comment",
			ext:  ".ogv",
			tags: map[string]string{
				sharedtags.JDescription: "desc", sharedtags.JAlbumArtist: "album artist", sharedtags.JDiscNumber: "1",
			},
			probe: ffprobeOutput{Streams: []ffprobeStream{{Tags: ffprobeTags{"comment": "desc", "album_artist": "album artist", "disc": "1"}}}},
		},
		{
			name:  "mpeg-ts service tags come back on the program",
			ext:   ".ts",
			tags:  map[string]string{sharedtags.JTitle: "title", sharedtags.JArtist: "artist"},
			probe: ffprobeOutput{Programs: []ffprobeProgram{{Tags: ffprobeTags{"service_name": "title", "service_provider": "artist"}}}},
		},
		{
			name:  "dates compare only their date part",
			ext:   ".mp4",
			tags:  map[string]string{sharedtags.JCreationTime: "2024-01-02T03:04:05Z"},
			probe: probeTags(ffprobeTags{"creation_time": "2024-01-02T00:00:00.000000Z"}),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			diffs := getDiffMapForFiletype(tt.ext, tt.tags, tt.probe)
			if len(diffs) != len(tt.tags) {
				t.Errorf("compared %d tags, want %d: %v", len(diffs), len(tt.tags), diffs)
			}
			for tag, d := range diffs {
				if d.existing != d.new {
					t.Errorf("%q read back as %q, want %q", tag, d.existing, d.new)
				}
			}
		})
	}
}

// TestDiffMapSkipsUnheldFields tests that fields a container can't hold aren't compared.
func TestDiffMapSkipsUnheldFields(t *testing.T) {
	diffs := getDiffMapForFiletype(".mov", map[string]string{sharedtags.JSynopsis: "syn", sharedtags.JTitle: "title"}, ffprobeOutput{})
	if _, ok := diffs[sharedtags.MOVTitle]; !ok || len(diffs) != 1 {
		t.Errorf("diffs = %v, want only the title", diffs)
	}
}

// probeTags returns FFprobe output holding the given format tags.
func probeTags(tags ffprobeTags) ffprobeOutput {
	return ffprobeOutput{Format: ffprobeFormat{Tags: tags}}
}
