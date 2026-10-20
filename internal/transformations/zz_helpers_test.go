package transformations

import (
	"testing"

	"metarr/internal/domain/enums"
	"metarr/internal/models"
	"metarr/internal/parsing"
)

// TestFilenameOpsRerun tests that filename ops run again on a processed name don't stack prefixes or appends, and a
// date tag op replaces the tag already there.
func TestFilenameOpsRerun(t *testing.T) {
	fp := &fileProcessor{metatagParser: parsing.NewMetaTemplateParser("test.json")}
	if got := fp.prefixStrings("Title", []models.FOpPrefix{{Value: "[Cats] "}}); got != "[Cats] Title" {
		t.Errorf("first prefix = %q, want %q", got, "[Cats] Title")
	}

	const name = "[2023-11-05] [Cats] Title (HD)"
	if got := fp.prefixStrings(name, []models.FOpPrefix{{Value: "[Cats] "}}); got != name {
		t.Errorf("prefix again = %q, want %q unchanged", got, name)
	}
	if got := fp.appendStrings(name, []models.FOpAppend{{Value: " (HD)"}}); got != name {
		t.Errorf("append again = %q, want %q unchanged", got, name)
	}
	tag := models.FOpDateTag{Loc: enums.DateTagLocPrefix, DateFormat: enums.DateYyyyMmDd}
	if got := fp.addDateTag(name, tag, "[2024-05-01]"); got != "[2024-05-01] [Cats] Title (HD)" {
		t.Errorf("new date tag = %q, want the old one replaced", got)
	}
	if got := fp.addDateTag("[23-11-05] Title", tag, "[2023-11-05]"); got != "[2023-11-05] Title" {
		t.Errorf("date tag in a new format = %q, want the old one replaced", got)
	}
	suffix := models.FOpDateTag{Loc: enums.DateTagLocSuffix, DateFormat: enums.DateYyyyMmDd}
	if got := fp.addDateTag("Title [2023-11-05]", suffix, "[2024-05-01]"); got != "Title [2024-05-01]" {
		t.Errorf("new suffix date tag = %q, want the old one replaced", got)
	}
}

// TestFixContractions tests that every contraction is fixed, case is kept, and non-ASCII text stays intact.
func TestFixContractions(t *testing.T) {
	tests := []struct{ in, want string }{
		{"Don t Stop", "Dont Stop"},
		{"don t stop, won t stop", "dont stop, wont stop"},
		{"İstanbul Don t Go", "İstanbul Dont Go"},
	}
	for _, tt := range tests {
		got, _, err := fixContractions(tt.in, tt.in, "video.mp4", enums.RenamingSpaces, false)
		if err != nil {
			t.Fatalf("fixContractions(%q): %v", tt.in, err)
		}
		if got != tt.want {
			t.Errorf("fixContractions(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}
