package ffmpeg

import (
	"fmt"
	"strings"
	"testing"

	"metarr/internal/models"

	"github.com/TubarrApp/gocommon/sharedtags"
)

// newTestBuilder returns a builder with just what the metadata functions write into.
func newTestBuilder() *ffCommandBuilder {
	return &ffCommandBuilder{builder: &strings.Builder{}, metadataMap: make(map[string]string)}
}

// TestModelNotMutated is what stops the container's preference leaking elsewhere: the
// model's title feeds the video-title directory template, so it must keep holding the
// file's actual title rather than the resolved container value.
func TestModelNotMutated(t *testing.T) {
	td := &models.MetadataTitlesDescs{
		Title:           "This is a video about ...",
		Fulltitle:       "This is a video about cats",
		Description:     "short",
		LongDescription: "long",
	}
	before := *td

	b := newTestBuilder()
	b.addTitlesDescs(&models.FileData{MTitleDesc: td, MetaOps: models.NewMetaOps()})

	if *td != before {
		t.Errorf("addTitlesDescs mutated the shared model:\n before %+v\n after  %+v", before, *td)
	}
}

// TestTitlePrecedence tests every combination of title fields: fulltitle wins, as a metafile title is often cut short.
func TestTitlePrecedence(t *testing.T) {
	const (
		truncated = "This is a video about ..."
		full      = "This is a video about cats"
	)

	var table strings.Builder
	for _, c := range []struct {
		// Described behaviour.
		name string

		// Fields in the model.
		title,
		fulltitle string

		// Desired title tag.
		want string
	}{
		{"fulltitle wins over title",
			truncated, full,
			full},

		{"title used without fulltitle",
			truncated, "",
			truncated},

		{"fulltitle used without title",
			"", full,
			full},

		{"neither",
			"", "",
			""},
	} {
		b := newTestBuilder()
		b.addTitlesDescs(&models.FileData{
			MTitleDesc: &models.MetadataTitlesDescs{Title: c.title, Fulltitle: c.fulltitle},
			MetaOps:    models.NewMetaOps(),
		})

		got := b.metadataMap[sharedtags.JTitle]
		fmt.Fprintf(&table, "\n%s %-30s \ngot: %-28q title: %-28q fulltitle: %q\n", mark(got == c.want), c.name, got, c.title, c.fulltitle)
		if got != c.want {
			t.Errorf("%s: title tag = %q, want %q", c.name, got, c.want)
		}
	}
	t.Log("\nTitle tag: fulltitle > title\n" + table.String())
}

// TestDescriptionPrecedence tests every combination of description fields.
//
// The first of longdescription, long_description and description goes into both the description and
// longdescription tags. long_description is another spelling of longdescription, so it's never a tag of its own.
func TestDescriptionPrecedence(t *testing.T) {
	values := [3]string{"long", "underscored", "short"} // longdescription, long_description, description.

	var table strings.Builder
	for combo := range 1 << len(values) {
		// Fill the fields in this combination.
		var present [3]string
		for i, v := range values {
			if combo&(1<<i) != 0 {
				present[i] = v
			}
		}

		// The first field present wins.
		want := ""
		for _, v := range present {
			if v != "" {
				want = v
				break
			}
		}

		b := newTestBuilder()
		b.addTitlesDescs(&models.FileData{
			MTitleDesc: &models.MetadataTitlesDescs{
				LongDescription:           present[0],
				LongUnderscoreDescription: present[1],
				Description:               present[2],
			},
			MetaOps: models.NewMetaOps(),
		})

		gotDesc := b.metadataMap[sharedtags.JDescription]
		gotLong := b.metadataMap[sharedtags.JLongDesc]
		_, gotUnderscore := b.metadataMap[sharedtags.JLongUnderscoreDesc]
		ok := gotDesc == want && gotLong == want && !gotUnderscore
		fmt.Fprintf(&table, "%s got: %-14q longdescription: %-7q long_description: %-13q description: %q\n",
			mark(ok), gotDesc, present[0], present[1], present[2])
		if !ok {
			t.Errorf("%v: description %q, longdescription %q, long_description written %v; want %q, %q, false",
				present, gotDesc, gotLong, gotUnderscore, want, want)
		}
	}
	t.Log("\nDescription and longdescription tags: longdescription > long_description > description\n" + table.String())
}

// TestDateFollowsYear tests that the date tag is the first full date in the year, or the year alone if none is.
func TestDateFollowsYear(t *testing.T) {
	var table strings.Builder
	for _, c := range []struct {
		// Described behaviour.
		name string

		// Dates in the model.
		releaseDate,
		originallyAvailable,
		date,
		uploadDate,
		year string

		// Desired date tag.
		want string
	}{
		{"release date first",
			"2021-01-02", "2021-02-03", "2021-03-04", "2021-04-05", "2021",
			"2021-01-02"},

		{"originally available next",
			"", "2021-02-03", "2021-03-04", "2021-04-05", "2021",
			"2021-02-03"},

		{"date next",
			"", "", "2021-03-04", "2021-04-05", "2021",
			"2021-03-04"},

		{"upload date last",
			"", "", "", "2021-04-05", "2021",
			"2021-04-05"},

		{"dates from other years skipped",
			"2019-01-02", "", "2021-03-04", "", "2021",
			"2021-03-04"},

		{"no full date in the year, as with a release year",
			"", "", "2021-03-04", "2021-04-05", "2019",
			"2019"},

		{"year with no date",
			"", "", "", "", "2019",
			"2019"},

		{"date with no year",
			"", "", "2021-03-04", "", "",
			"2021-03-04"},
	} {
		b := newTestBuilder()
		b.addDates(&models.MetadataDates{
			ReleaseDate:           c.releaseDate,
			OriginallyAvailableAt: c.originallyAvailable,
			Date:                  c.date,
			UploadDate:            c.uploadDate,
			Year:                  c.year,
		})

		got := b.metadataMap[sharedtags.JDate]
		fmt.Fprintf(&table,
			"\n%s %-50s \ngot: %-13q release_date: %-13q originally_available_at: %-13q date: %-13q upload_date: %-13q year: %q\n\n",
			mark(got == c.want), c.name, got, c.releaseDate, c.originallyAvailable, c.date, c.uploadDate, c.year)
		if got != c.want {
			t.Errorf("%s: date tag = %q, want %q", c.name, got, c.want)
		}
	}
	t.Log("\nDate tag: first full date in the year (release date > originally available > date > upload date), else the year\n" +
		table.String())
}

// TestCreditArrays tests that a single credit wins over a list of them, and a list is joined when there's no single one.
func TestCreditArrays(t *testing.T) {
	var table strings.Builder
	for _, c := range []struct {
		// Described behaviour.
		name string

		// Credits in the model.
		single string
		list   []string

		// Desired director tag.
		want string
	}{
		{"single value only",
			"Ann", nil,
			"Ann"},

		{"list only",
			"", []string{"Ann", "Bob"},
			"Ann; Bob"},

		{"single value wins over list",
			"Ann", []string{"Bob", "Cat"},
			"Ann"},

		{"neither",
			"", nil,
			""},
	} {
		b := newTestBuilder()
		b.addCredits(&models.MetadataCredits{Director: c.single, Directors: c.list})

		got := b.metadataMap[sharedtags.JDirector]
		fmt.Fprintf(&table, "  %s %-28s got: %-11q director: %-6q directors: %q\n",
			mark(got == c.want), c.name, got, c.single, strings.Join(c.list, ", "))
		if got != c.want {
			t.Errorf("%s: director tag = %q, want %q", c.name, got, c.want)
		}
	}
	t.Log("\nDirector tag: the single value, else the list joined\n" + table.String())
}

// mark returns a tick for a passing case and a cross for a failing one.
func mark(ok bool) string {
	if ok {
		return "✓"
	}
	return "✗"
}
