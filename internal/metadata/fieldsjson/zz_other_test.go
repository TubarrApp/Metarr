package fieldsjson

import (
	"testing"

	"metarr/internal/models"

	"github.com/TubarrApp/gocommon/sharedtags"
)

// TestJSONText tests that text, numbers and lists of text are read, skipping keys with no value.
func TestJSONText(t *testing.T) {
	json := map[string]any{
		"empty": "", "text": "a", "number": float64(2), "list": []any{"x", "", "y", 3.0}, "nolist": []any{},
	}
	for _, c := range []struct {
		keys []string
		want string
	}{
		{[]string{"text"}, "a"},
		{[]string{"empty", "text"}, "a"},
		{[]string{"number"}, "2"},
		{[]string{"list"}, "x, y"},
		{[]string{"nolist", "missing"}, ""},
	} {
		if got := jsonText(json, c.keys...); got != c.want {
			t.Errorf("jsonText(%v) = %q, want %q", c.keys, got, c.want)
		}
	}
}

// TestFillShowAndOtherData tests that fields are read from their own keys, or else yt-dlp's or inferred values.
func TestFillShowAndOtherData(t *testing.T) {
	fd := &models.FileData{
		MCredits:  &models.MetadataCredits{Channel: "channel"},
		MShowData: &models.MetadataShowData{},
		MOther:    &models.MetadataOtherData{},
	}
	json := map[string]any{
		sharedtags.JSeries: "series", sharedtags.JSeasonNumber: 2.0, sharedtags.JSeason: "Season 2",
		sharedtags.JEpisodeNumber: 5.0, sharedtags.JEpisodeID: "S02E05", sharedtags.JLicense: "CC BY",
		sharedtags.JCategories: []any{"Gaming"}, sharedtags.JTags: []any{"a", "b"}, sharedtags.JHeight: 1080.0,
		sharedtags.JLanguage: "en", sharedtags.JAlbum: "album", sharedtags.JTrackNumber: 3.0, sharedtags.JDiscNumber: 1.0,
	}
	fillShowData(fd, json)
	fillOtherData(fd, json)

	for _, c := range []struct{ name, got, want string }{
		{"show", fd.MShowData.Show, "series"},
		{"season number", fd.MShowData.SeasonNumber, "2"},
		{"season title", fd.MShowData.SeasonTitle, "Season 2"},
		{"episode sort", fd.MShowData.EpisodeSort, "5"},
		{"episode ID", fd.MShowData.EpisodeID, "S02E05"},
		{"network", fd.MShowData.Network, "channel"},
		{"copyright", fd.MOther.Copyright, "CC BY"},
		{"genre", fd.MOther.Genre, "Gaming"},
		{"keywords", fd.MOther.Keywords, "a, b"},
		{"HD video", fd.MOther.HDVideo, "2"},
		{"language", fd.MOther.Language, "en"},
		{"album", fd.MOther.Album, "album"},
		{"track", fd.MOther.Track, "3"},
		{"disc", fd.MOther.Disc, "1"},
	} {
		if c.got != c.want {
			t.Errorf("%s = %q, want %q", c.name, c.got, c.want)
		}
	}
}

// TestHDVideoFromHeight tests the iTunes HD video flag worked out from a video's height.
func TestHDVideoFromHeight(t *testing.T) {
	for _, c := range []struct {
		height any
		want   string
	}{
		{2160.0, "2"}, {1080.0, "2"}, {720.0, "1"}, {480.0, "0"}, {0.0, ""}, {"1080", ""}, {nil, ""},
	} {
		if got := hdVideoFromHeight(c.height); got != c.want {
			t.Errorf("hdVideoFromHeight(%v) = %q, want %q", c.height, got, c.want)
		}
	}
}
