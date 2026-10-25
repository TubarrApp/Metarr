package fieldsjson

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"metarr/internal/metadata/metawriters"
	"metarr/internal/models"
)

// fillCase is one link in a fill chain: the metafile content, the field checked, and its desired value.
type fillCase struct {
	name  string
	json  string
	field func(*models.FileData) string
	want  string
}

// TestFillChains tests each fallback used when filling fields from a metafile, one case per link in each chain.
func TestFillChains(t *testing.T) {
	var (
		fulltitle   = func(fd *models.FileData) string { return fd.MTitleDesc.Fulltitle }
		title       = func(fd *models.FileData) string { return fd.MTitleDesc.Title }
		description = func(fd *models.FileData) string { return fd.MTitleDesc.Description }
		synopsis    = func(fd *models.FileData) string { return fd.MTitleDesc.Synopsis }
		artist      = func(fd *models.FileData) string { return fd.MCredits.Artist }
		albumArtist = func(fd *models.FileData) string { return fd.MCredits.AlbumArtist }
		composer    = func(fd *models.FileData) string { return fd.MCredits.Composer }
		director    = func(fd *models.FileData) string { return fd.MCredits.Director }
		writer      = func(fd *models.FileData) string { return fd.MCredits.Writer }
		show        = func(fd *models.FileData) string { return fd.MShowData.Show }
		seasonTitle = func(fd *models.FileData) string { return fd.MShowData.SeasonTitle }
		episodeSort = func(fd *models.FileData) string { return fd.MShowData.EpisodeSort }
		network     = func(fd *models.FileData) string { return fd.MShowData.Network }
		genre       = func(fd *models.FileData) string { return fd.MOther.Genre }
		keywords    = func(fd *models.FileData) string { return fd.MOther.Keywords }
		copyright   = func(fd *models.FileData) string { return fd.MOther.Copyright }
		hdVideo     = func(fd *models.FileData) string { return fd.MOther.HDVideo }
	)

	chains := []struct {
		chain string
		cases []fillCase
	}{
		{"Titles fill each other", []fillCase{
			{"fulltitle from title", `{"title": "T"}`, fulltitle, "T"},
			{"title from fulltitle", `{"fulltitle": "F"}`, title, "F"},
		}},
		{"Empty descriptions: longdescription > long_description > description > synopsis > summary > comment", []fillCase{
			{"from longdescription first", `{"longdescription": "L", "long_description": "U", "description": "D"}`, synopsis, "L"},
			{"from long_description next", `{"long_description": "U", "description": "D"}`, synopsis, "U"},
			{"from description next", `{"description": "D", "summary": "S"}`, synopsis, "D"},
			{"from synopsis next", `{"synopsis": "Sy", "summary": "Su"}`, description, "Sy"},
			{"from summary next", `{"summary": "Su", "comment": "C"}`, description, "Su"},
			{"from comment last", `{"comment": "C"}`, description, "C"},
			{"a filled description is kept", `{"description": "D", "longdescription": "L"}`, description, "D"},
		}},
		{"Artist and album artist fill each other", []fillCase{
			{"album artist from artist", `{"artist": "Ar"}`, albumArtist, "Ar"},
			{"artist from album artist", `{"album_artist": "AA"}`, artist, "AA"},
			{"album artist from artist, not an earlier credit", `{"author": "Au", "artist": "Ar"}`, albumArtist, "Ar"},
		}},
		{"Other empty credits fill from the first filled credit, in order", []fillCase{
			{"writer from channel, before director", `{"director": "Dir", "channel": "Chan"}`, writer, "Chan"},
			{"composer from author, before artist", `{"artist": "Ar", "author": "Au"}`, composer, "Au"},
			{"a filled credit is kept", `{"director": "Dir", "channel": "Chan"}`, director, "Dir"},
		}},
		{"Show details", []fillCase{
			{"show from show", `{"show": "Show", "series": "Series"}`, show, "Show"},
			{"show from series", `{"series": "Series"}`, show, "Series"},
			{"season title from season_title", `{"season_title": "ST", "season": "S"}`, seasonTitle, "ST"},
			{"season title from season", `{"season": "Season 2"}`, seasonTitle, "Season 2"},
			{"episode sort from episode_sort", `{"episode_sort": "4", "episode_number": 5}`, episodeSort, "4"},
			{"episode sort from episode_number", `{"episode_number": 5}`, episodeSort, "5"},
			{"network from network", `{"network": "Net", "channel": "Chan"}`, network, "Net"},
			{"network from channel", `{"channel": "Chan"}`, network, "Chan"},
		}},
		{"Other details", []fillCase{
			{"genre from genre", `{"genre": "G", "genres": ["Gs"], "categories": ["C"]}`, genre, "G"},
			{"genre from genres", `{"genres": ["Comedy", "Drama"], "categories": ["C"]}`, genre, "Comedy, Drama"},
			{"genre from categories", `{"categories": ["Gaming"]}`, genre, "Gaming"},
			{"keywords from keywords", `{"keywords": "k", "tags": ["a"]}`, keywords, "k"},
			{"keywords from tags", `{"tags": ["a", "b"]}`, keywords, "a, b"},
			{"copyright from copyright", `{"copyright": "C", "license": "L"}`, copyright, "C"},
			{"copyright from license", `{"license": "CC BY"}`, copyright, "CC BY"},
			{"HD video from hd_video", `{"hd_video": "1", "height": 1080}`, hdVideo, "1"},
			{"HD video from a 1080p height", `{"height": 1080}`, hdVideo, "2"},
			{"HD video from a 720p height", `{"height": 720}`, hdVideo, "1"},
			{"HD video from a lower height", `{"height": 480}`, hdVideo, "0"},
		}},
	}

	// Run each case, collecting a table to print once.
	var table strings.Builder
	for _, chain := range chains {
		fmt.Fprintf(&table, "\n%s\n", chain.chain)
		for _, c := range chain.cases {
			got := c.field(fillFieldsFrom(t, c.json))
			mark := "✓"
			if got != c.want {
				mark = "✗"
				t.Errorf("%s: got %q, want %q", c.name, got, c.want)
			}
			fmt.Fprintf(&table, "  %s %-48s got: %-16q from: %s\n", mark, c.name, got, c.json)
		}
	}
	t.Log(table.String())
}

// fillFieldsFrom fills a file's titles, credits, descriptions and other details from metafile content, as processing does.
func fillFieldsFrom(t *testing.T, content string) *models.FileData {
	t.Helper()
	path := filepath.Join(t.TempDir(), "video.json")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	f, err := os.OpenFile(path, os.O_RDWR, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	rw := metawriters.NewJSONFileRW(context.Background(), f)
	data, err := rw.DecodeJSON(f)
	if err != nil {
		t.Fatal(err)
	}
	fd := models.NewFileData()
	fd.MetaFilePath = path
	FillJSONFields(fd, data, rw)
	return fd
}
