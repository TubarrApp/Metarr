package ffmpeg

import (
	"testing"

	"metarr/internal/models"

	"github.com/TubarrApp/gocommon/sharedtags"
)

// newTestBuilder returns a builder with just the map addTitlesDescs writes into.
func newTestBuilder() *ffCommandBuilder {
	return &ffCommandBuilder{metadataMap: make(map[string]string)}
}

// TestFulltitlePreferredForContainer covers the rule for embedded video metadata:
// fulltitle wins whenever it exists, since a metafile title is often the truncated
// form. Meta operations do not change which field is preferred.
func TestFulltitlePreferredForContainer(t *testing.T) {
	const (
		truncated = "This is a video about ..."
		full      = "This is a video about cats"
	)

	tests := []struct {
		name             string
		title, fulltitle string
		titleOp          func(*models.MetaOps)
		want             string
	}{
		{"fulltitle wins over a truncated title", truncated, full, nil, full},
		{"title used when fulltitle absent", full, "", nil, full},
		{"fulltitle used when title absent", "", full, nil, full},
		{
			name:  "operating on fulltitle carries into the container",
			title: truncated, fulltitle: "[CATS] " + full,
			titleOp: func(o *models.MetaOps) {
				o.Prefixes = append(o.Prefixes, models.MetaPrefix{Field: sharedtags.JFulltitle, Prefix: "[CATS] "})
			},
			want: "[CATS] " + full,
		},
		{
			name:  "a title operation applies when there is no fulltitle",
			title: "[CATS] " + full, fulltitle: "",
			titleOp: func(o *models.MetaOps) {
				o.Prefixes = append(o.Prefixes, models.MetaPrefix{Field: sharedtags.JTitle, Prefix: "[CATS] "})
			},
			want: "[CATS] " + full,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ops := models.NewMetaOps()
			if tt.titleOp != nil {
				tt.titleOp(ops)
			}

			b := newTestBuilder()
			b.addTitlesDescs(&models.FileData{
				MTitleDesc: &models.MetadataTitlesDescs{Title: tt.title, Fulltitle: tt.fulltitle},
				MetaOps:    ops,
			})

			if got := b.metadataMap[sharedtags.JTitle]; got != tt.want {
				t.Errorf("embedded title = %q, want %q", got, tt.want)
			}
		})
	}
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

// TestDateFollowsYear tests that the date tag gives way to the year alone when the date is from another year.
func TestDateFollowsYear(t *testing.T) {
	for _, c := range []struct{ name, date, year, want string }{
		{"date in the year", "2021-05-06", "2021", "2021-05-06"},
		{"date from another year", "2021-05-06", "2019", "2019"},
		{"year with no date", "", "2019", "2019"},
		{"date with no year", "2021-05-06", "", "2021-05-06"},
	} {
		b := newTestBuilder()
		b.addDates(&models.MetadataDates{Date: c.date, Year: c.year})
		if got := b.metadataMap[sharedtags.JDate]; got != c.want {
			t.Errorf("%s: date = %q, want %q", c.name, got, c.want)
		}
	}
}

// TestLongDescriptionPreferred mirrors the title rule for descriptions.
func TestLongDescriptionPreferred(t *testing.T) {
	tests := []struct {
		name                           string
		desc, longDesc, longUnderscore string
		wantDesc, wantLong             string
	}{
		{"long wins", "short", "long", "", "long", "long"},
		{"underscore form used next", "short", "", "underscored", "underscored", "underscored"},
		{"falls back to description", "short", "", "", "short", "short"},
		{"all empty", "", "", "", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := newTestBuilder()
			b.addTitlesDescs(&models.FileData{
				MTitleDesc: &models.MetadataTitlesDescs{
					Description:               tt.desc,
					LongDescription:           tt.longDesc,
					LongUnderscoreDescription: tt.longUnderscore,
				},
				MetaOps: models.NewMetaOps(),
			})

			if got := b.metadataMap[sharedtags.JDescription]; got != tt.wantDesc {
				t.Errorf("description = %q, want %q", got, tt.wantDesc)
			}
			if got := b.metadataMap[sharedtags.JLongDesc]; got != tt.wantLong {
				t.Errorf("longdescription = %q, want %q", got, tt.wantLong)
			}
		})
	}
}
