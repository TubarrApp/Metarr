package fieldsnfo

import (
	"testing"

	"metarr/internal/models"
)

// TestFillNFOShowAndOtherData tests that NFO genres and tags are joined, and a set stands in for the show.
func TestFillNFOShowAndOtherData(t *testing.T) {
	fd := &models.FileData{
		MCredits:  &models.MetadataCredits{Channel: "channel"},
		MShowData: &models.MetadataShowData{},
		MOther:    &models.MetadataOtherData{},
		NFOData: &models.NFOData{
			Genres: []string{"Comedy", "Drama"}, Tags: []string{"a", "b"}, Set: " set ", Language: "en", HDVideo: "1",
		},
	}
	fillNFOShowData(fd)
	fillNFOOtherData(fd)

	for _, c := range []struct{ name, got, want string }{
		{"show", fd.MShowData.Show, "set"},
		{"network", fd.MShowData.Network, "channel"},
		{"genre", fd.MOther.Genre, "Comedy, Drama"},
		{"keywords", fd.MOther.Keywords, "a, b"},
		{"language", fd.MOther.Language, "en"},
		{"HD video", fd.MOther.HDVideo, "1"},
	} {
		if c.got != c.want {
			t.Errorf("%s = %q, want %q", c.name, c.got, c.want)
		}
	}
}
