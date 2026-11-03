package metabuilder

import (
	"encoding/xml"
	"strings"
	"testing"

	"metarr/internal/models"
)

// checkNFO fails the test for any wanted text missing from got, or any unwanted text in it.
func checkNFO(t *testing.T, got string, want, not []string) {
	t.Helper()
	for _, w := range want {
		if !strings.Contains(got, w) {
			t.Errorf("missing %q in:\n%s", w, got)
		}
	}
	for _, n := range not {
		if strings.Contains(got, n) {
			t.Errorf("unexpected %q in:\n%s", n, got)
		}
	}
}

// TestNFOOutlineAndTagline tests that the outline holds the whole description, and a subtitle only reaches the tagline.
func TestNFOOutlineAndTagline(t *testing.T) {
	for _, c := range []struct {
		name string
		in   models.MetadataTitlesDescs
		want []string
		not  []string
	}{
		{
			name: "no description",
			in:   models.MetadataTitlesDescs{Title: "T"},
			want: []string{"<title>T</title>"},
			not:  []string{"<plot>", "<outline>", "<tagline>"},
		},
		{
			name: "subtitle set to the title with no description",
			in:   models.MetadataTitlesDescs{Fulltitle: "T", Title: "T", Subtitle: "T"},
			want: []string{"<title>T</title>"},
			not:  []string{"<plot>", "<outline>", "<tagline>"},
		},
		{
			name: "subtitle set to the title with a description",
			in:   models.MetadataTitlesDescs{Fulltitle: "T", Subtitle: "T", Description: "D", Synopsis: "D"},
			want: []string{"<plot>D</plot>", "<outline>D</outline>"},
			not:  []string{"<outline>T</outline>", "<tagline>"},
		},
		{
			name: "real subtitle with no description",
			in:   models.MetadataTitlesDescs{Fulltitle: "T", Subtitle: "S"},
			want: []string{"<tagline>S</tagline>"},
			not:  []string{"<outline>"},
		},
		{
			name: "real subtitle with a description",
			in:   models.MetadataTitlesDescs{Fulltitle: "T", Subtitle: "S", Description: "D", Synopsis: "D"},
			want: []string{"<outline>D</outline>", "<tagline>S</tagline>"},
			not:  []string{"<outline>S</outline>"},
		},
		{
			name: "multi-line description is kept whole",
			in:   models.MetadataTitlesDescs{Description: "One\n\nTwo", Synopsis: "One\n\nTwo"},
			want: []string{"<outline>One&#xA;&#xA;Two</outline>"},
		},
		{
			name: "own synopsis is used over the description",
			in:   models.MetadataTitlesDescs{Description: "Long", Synopsis: "Short", Summary: "Other"},
			want: []string{"<plot>Long</plot>", "<outline>Short</outline>"},
		},
	} {
		t.Run(c.name, func(t *testing.T) {
			checkNFO(t, addTitlesAndDescriptionsToNFO(&c.in), c.want, c.not)
		})
	}
}

// TestNFOEpisodeNumber tests that yt-dlp's episode number is written, and marks the file as an episode.
func TestNFOEpisodeNumber(t *testing.T) {
	for _, c := range []struct {
		name string
		in   models.MetadataShowData
		root string
		want []string
		not  []string
	}{
		{
			name: "number with no season",
			in:   models.MetadataShowData{Show: "Show", EpisodeSort: "5"},
			root: models.NFORootEpisode,
			want: []string{"<showtitle>Show</showtitle>", "<episode>5</episode>"},
		},
		{
			name: "number is used over the ID",
			in:   models.MetadataShowData{EpisodeSort: "5", EpisodeID: "S02E05", SeasonNumber: "2"},
			root: models.NFORootEpisode,
			want: []string{"<episode>5</episode>", "<season>2</season>"},
			not:  []string{"S02E05"},
		},
		{
			name: "ID alone",
			in:   models.MetadataShowData{EpisodeID: "7"},
			root: models.NFORootEpisode,
			want: []string{"<episode>7</episode>"},
		},
		{
			name: "show alone stays a movie",
			in:   models.MetadataShowData{Show: "Show"},
			root: models.NFORootMovie,
			want: []string{"<set>Show</set>"},
			not:  []string{"<episode>"},
		},
	} {
		t.Run(c.name, func(t *testing.T) {
			if root := nfoRootFor(&c.in); root != c.root {
				t.Errorf("root = %q, want %q", root, c.root)
			}
			checkNFO(t, addShowDataToNFO(&c.in), c.want, c.not)
		})
	}
}

// TestNFOOtherData tests that genres and tags get an element each, and the runtime and unique ID are written.
func TestNFOOtherData(t *testing.T) {
	got := addOtherDataToNFO(&models.MetadataOtherData{
		Genre: "Music, Entertainment", Keywords: "a, b, a", Language: "en", Runtime: "19", VideoID: `x"1`, Site: "youtube",
	})
	checkNFO(t, got, []string{
		"<genre>Music</genre>", "<genre>Entertainment</genre>", "<tag>a</tag>", "<tag>b</tag>", "<runtime>19</runtime>",
		"<language>en</language>", "<originallanguage>en</originallanguage>",
		`<uniqueid type="youtube" default="true">x&#34;1</uniqueid>`,
	}, []string{"Music, Entertainment"})
	if n := strings.Count(got, "<tag>a</tag>"); n != 1 {
		t.Errorf("repeated tag written %d times, want 1 in:\n%s", n, got)
	}

	// A unique ID needs both its value and its type.
	for _, o := range []models.MetadataOtherData{{VideoID: "x"}, {Site: "youtube"}} {
		checkNFO(t, addOtherDataToNFO(&o), nil, []string{"<uniqueid"})
	}
	checkNFO(t, addOtherDataToNFO(&models.MetadataOtherData{}), nil, []string{"<genre>", "<tag>", "<runtime>", "language>"})
}

// TestNFODates tests that dates are written without a time of day, and that aired matches premiered.
func TestNFODates(t *testing.T) {
	checkNFO(t, addDatesToNFO(&models.MetadataDates{CreationTime: "2024-08-11T10:20:30.000000Z"}), []string{
		"<premiered>2024-08-11</premiered>", "<aired>2024-08-11</aired>", "<released>2024-08-11</released>",
	}, []string{"T10:20"})
	checkNFO(t, addDatesToNFO(&models.MetadataDates{}), nil, []string{"<premiered>", "<aired>", "<released>"})
}

// TestNFOCredits tests that writers are written in both forms, and that studios are written.
func TestNFOCredits(t *testing.T) {
	checkNFO(t, addCreditsToNFO(&models.MetadataCredits{Writer: "W", Studio: "S", Studios: []string{"S", "S2"}}), []string{
		"<writer>W</writer>", "<credits>W</credits>", "<studio>S</studio>", "<studio>S2</studio>",
	}, nil)
}

// TestNFOThumbnailForms tests the forms media servers read, and that Metarr can still read the thumbnail back.
func TestNFOThumbnailForms(t *testing.T) {
	const url = "http://x/y.jpg?a=1&b=2"
	const escaped = "http://x/y.jpg?a=1&amp;b=2"

	got, err := BuildNFOContents(&models.FileData{MetaFilePath: "v.json", MWebData: &models.MetadataWebData{Thumbnail: url}})
	if err != nil {
		t.Fatal(err)
	}
	checkNFO(t, got, []string{
		`<thumb aspect="poster">` + escaped + "</thumb>",
		"<thumb>" + escaped + "</thumb>",
		"<fanart>\n    <thumb>" + escaped + "</thumb>\n  </fanart>",
	}, []string{"<poster>"})

	var n models.NFOData
	if err := xml.Unmarshal([]byte(got), &n); err != nil {
		t.Fatal(err)
	}
	if n.Thumb != url {
		t.Errorf("read back thumb = %q, want %q", n.Thumb, url)
	}
}
