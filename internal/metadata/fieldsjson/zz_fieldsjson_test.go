package fieldsjson

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"metarr/internal/domain/logger"
	"metarr/internal/ffmpeg"
	"metarr/internal/metadata/metawriters"
	"metarr/internal/models"

	"github.com/TubarrApp/gocommon/sharedconsts"
	"github.com/TubarrApp/gocommon/sharedtags"
)

// ******************************** Dates *********************************************************************************************

// TestMain gives the global logger a console writer, since the zero value has none.
func TestMain(m *testing.M) {
	logger.Pl.Console = io.Discard
	os.Exit(m.Run())
}

// TestVideoDateCombinations tests every combination of date fields, each from a different year so the one used shows.
func TestVideoDateCombinations(t *testing.T) {
	// The fields in order of precedence, each with its value and whether it's a full date.
	fields := []struct {
		key, value string
		full       bool
	}{
		{"release_date", `"2015-01-15"`, true},
		{"originally_available_at", `"2016-02-16"`, true},
		{"release_year", `2017`, false},
		{"date", `"2018-03-17"`, true},
		{"year", `"2019"`, false},
		{"upload_date", `"20200418"`, true},
	}

	var table strings.Builder
	for combo := 1; combo < 1<<len(fields); combo++ {
		// Build the metafile from the fields in this combination.
		var parts, names []string
		for i, f := range fields {
			if combo&(1<<i) != 0 {
				parts = append(parts, fmt.Sprintf("%q: %s", f.key, f.value))
				names = append(names, f.key)
			}
		}

		// The first field present gives the year, and the date tag is its date, or the year alone for a year field.
		var wantYear, wantDateTag string
		for i, f := range fields {
			if combo&(1<<i) == 0 {
				continue
			}
			wantYear = strings.Trim(f.value, `"`)[:4]
			wantDateTag = wantYear
			if f.full {
				wantDateTag = strings.Trim(f.value, `"`)
				if len(wantDateTag) == 8 {
					wantDateTag = wantDateTag[:4] + "-" + wantDateTag[4:6] + "-" + wantDateTag[6:]
				}
			}
			break
		}

		fd := fillDatesFrom(t, "{"+strings.Join(parts, ", ")+"}")
		gotDateTag := ffmpeg.MetadataTags(fd)[sharedtags.JDate]
		if fd.MDates.Year != wantYear || gotDateTag != wantDateTag {
			t.Errorf("%v: year %q, date tag %q; want %q, %q", names, fd.MDates.Year, gotDateTag, wantYear, wantDateTag)
		}
		fmt.Fprintf(&table, "%-82s year: %-5s date tag: %-11s creation time: %s\n",
			strings.Join(names, " + "), fd.MDates.Year, gotDateTag, fd.MDates.CreationTime)
	}
	t.Log("\n" + table.String())
}

// TestYearSameOnRerun tests that a worked-out originally available date isn't saved to pass for a release date later.
func TestYearSameOnRerun(t *testing.T) {
	path := filepath.Join(t.TempDir(), "video.json")
	if err := os.WriteFile(path, []byte(`{"release_year": 2019, "upload_date": "20210506"}`), 0o644); err != nil {
		t.Fatal(err)
	}

	// Run twice on the same metafile, as reprocessing does.
	for run := 1; run <= 2; run++ {
		if year := fillDatesFromFile(t, path).MDates.Year; year != "2019" {
			t.Errorf("run %d: year = %q, want 2019", run, year)
		}
	}

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(content), "originally_available_at") {
		t.Errorf("metafile has a worked-out originally available date: %s", content)
	}
}

// fillDatesFrom fills a file's dates from the given metafile content, as processing does.
func fillDatesFrom(t *testing.T, content string) *models.FileData {
	t.Helper()
	path := filepath.Join(t.TempDir(), "video.json")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return fillDatesFromFile(t, path)
}

// fillDatesFromFile fills a file's dates from the metafile at path, as processing does.
func fillDatesFromFile(t *testing.T, path string) *models.FileData {
	t.Helper()
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
	FillTimestamps(fd, data, rw)
	return fd
}

// TestVideoDates tests the year and video date tag taken from a metafile's dates, and that the creation time stays
// a real date.
func TestVideoDates(t *testing.T) {
	for _, c := range []struct{ name, json, wantYear, wantDateTag, wantCreation string }{
		{"release date wins over release year", `{"release_date": "2021-05-06", "release_year": 2019}`, "2021", "2021-05-06", "2021-05-06"},
		{"release date wins over year", `{"release_date": "2021-05-06", "year": "2019"}`, "2021", "2021-05-06", "2021-05-06"},
		{"release date wins over date", `{"date": "2021-05-06", "release_date": "2019-05-06"}`, "2019", "2019-05-06", "2019-05-06"},
		{"release year wins over date", `{"date": "2021-05-06", "release_year": 2019}`, "2019", "2019", "2021-05-06"},
		{"release year wins over year", `{"year": "2020", "release_year": 2019}`, "2019", "2019", ""},
		{"date wins over year", `{"date": "2021-05-06", "year": "2019"}`, "2021", "2021-05-06", "2021-05-06"},
		{"year used alone", `{"year": "2020"}`, "2020", "2020", ""},
		{"upload date used last", `{"upload_date": "20210506"}`, "2021", "2021-05-06", "2021-05-06"},
		{"upload date kept for a release year", `{"release_year": 2019, "upload_date": "20210506"}`, "2019", "2019", "2021-05-06"},
	} {
		t.Run(c.name, func(t *testing.T) {
			// Write the metafile and decode it as processing does.
			path := filepath.Join(t.TempDir(), "video.json")
			if err := os.WriteFile(path, []byte(c.json), 0o644); err != nil {
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

			// Fill the dates and check what the video gets.
			fd := models.NewFileData()
			fd.MetaFilePath = path
			FillTimestamps(fd, data, rw)
			if fd.MDates.Year != c.wantYear {
				t.Errorf("year = %q, want %q", fd.MDates.Year, c.wantYear)
			}
			if got := ffmpeg.MetadataTags(fd)[sharedtags.JDate]; got != c.wantDateTag {
				t.Errorf("video date tag = %q, want %q", got, c.wantDateTag)
			}
			if !strings.HasPrefix(fd.MDates.CreationTime, c.wantCreation) {
				t.Errorf("creation time = %q, want it to start %q", fd.MDates.CreationTime, c.wantCreation)
			}
		})
	}
}

// ******************************** Credits *******************************************************************************************

// TestCreditKeysCoverEveryCredit tests that every credit field has a place in the importance order, once.
func TestCreditKeysCoverEveryCredit(t *testing.T) {
	credits := models.JSONGroupFields[sharedconsts.MetaOpAllCredits]
	if len(creditKeys) != len(credits) {
		t.Errorf("creditKeys has %d fields, the all-credits group has %d", len(creditKeys), len(credits))
	}
	for _, k := range credits {
		if !slices.Contains(creditKeys, k) {
			t.Errorf("creditKeys lacks %q", k)
		}
	}
}

// ******************************** Fill chains ***************************************************************************************

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

// ******************************** Other *********************************************************************************************

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
		sharedtags.JDuration: 1132.0, sharedtags.JID: "abc123", sharedtags.JExtractor: "youtube",
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
		{"runtime", fd.MOther.Runtime, "19"},
		{"video ID", fd.MOther.VideoID, "abc123"},
		{"site", fd.MOther.Site, "youtube"},
	} {
		if c.got != c.want {
			t.Errorf("%s = %q, want %q", c.name, c.got, c.want)
		}
	}
}

// TestRuntimeFromDuration tests that seconds become whole minutes, with short videos still counting as a minute.
func TestRuntimeFromDuration(t *testing.T) {
	for _, c := range []struct {
		duration any
		want     string
	}{
		{1132.0, "19"}, {24.0, "1"}, {90.0, "2"}, {3600.0, "60"}, {0.0, ""}, {"1132", ""}, {nil, ""},
	} {
		if got := runtimeFromDuration(c.duration); got != c.want {
			t.Errorf("runtimeFromDuration(%v) = %q, want %q", c.duration, got, c.want)
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
