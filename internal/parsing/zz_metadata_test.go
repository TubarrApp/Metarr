package parsing

import (
	"io"
	"testing"

	"metarr/internal/dates"
	"metarr/internal/domain/logger"
	"metarr/internal/models"

	"github.com/TubarrApp/gocommon/sharedenums"
)

// TestFillMetaTemplateTag tests metadata tags are all filled, numbers included, and filled values aren't expanded again.
func TestFillMetaTemplateTag(t *testing.T) {
	logger.Pl.Console = io.Discard
	mtp := NewMetaTemplateParser("test.json")
	j := map[string]any{
		"title":     "Cats",
		"uploader":  "Bob",
		"duration":  float64(125),
		"rating":    4.5,
		"is_live":   false,
		"empty":     "",
		"recursive": "{{title}}",
		"tags":      []any{"a"},
	}

	for in, want := range map[string]string{
		"{{uploader}} - {{title}}":   "Bob - Cats",
		"{{missing}} {{title}}":      "{{missing}} Cats", // A missing tag doesn't stop later ones.
		"{{ title }}":                "Cats",
		"{{duration}}s, {{rating}}*": "125s, 4.5*",
		"live: {{is_live}}":          "live: false",
		"{{empty}}{{tags}}":          "{{empty}}{{tags}}",
		"{{recursive}}":              "{{title}}", // Values are literal, not re-filled.
		"no tags":                    "no tags",
	} {
		if got, _ := mtp.FillMetaTemplateTag(in, j); got != want {
			t.Errorf("FillMetaTemplateTag(%q) = %q, want %q", in, got, want)
		}
	}

	if _, isTemplate := mtp.FillMetaTemplateTag("plain", j); isTemplate {
		t.Error("plain text reported as a template")
	}
}

// TestMakeDateTagFormats tests a date tag is made in every format, including the two without a year.
func TestMakeDateTagFormats(t *testing.T) {
	fd := &models.FileData{MDates: &models.MetadataDates{FormattedDate: "2024-05-01"}}
	for format, want := range map[sharedenums.DateFormat]string{
		sharedenums.DateYyyyMmDd: "[2024-05-01]",
		sharedenums.DateYyMmDd:   "[24-05-01]",
		sharedenums.DateDdMmYyyy: "[01-05-2024]",
		sharedenums.DateMmDdYy:   "[05-01-24]",
		sharedenums.DateMmDd:     "[05-01]",
		sharedenums.DateDdMm:     "[01-05]",
	} {
		if got, err := dates.MakeDateTag(nil, fd, format); err != nil || got != want {
			t.Errorf("MakeDateTag(%v) = %q, %v, want %q", format, got, err, want)
		}
	}
}
