package parsing

import (
	"io"
	"testing"

	"metarr/internal/domain/logger"
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
