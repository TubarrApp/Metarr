package metawriters

import (
	"testing"

	"metarr/internal/domain/enums"
	"metarr/internal/models"
	"metarr/internal/parsing"
)

// TestJSONOpsRerun tests that meta ops run again on edited JSON don't stack prefixes or appends, and a date tag op
// replaces the tag already there.
func TestJSONOpsRerun(t *testing.T) {
	rw := &JSONFileRW{}
	mtp := parsing.NewMetaTemplateParser("test.json")
	prefix := []models.MetaPrefix{{Field: "title", Prefix: "[Cats] "}}

	first := map[string]any{"title": "Title"}
	if !rw.jsonPrefix(first, "test.json", prefix, mtp) || first["title"] != "[Cats] Title" {
		t.Errorf("first prefix = %q, want %q", first["title"], "[Cats] Title")
	}

	j := map[string]any{"title": "[2023-11-05] [Cats] Title (HD)"}
	if rw.jsonPrefix(j, "test.json", prefix, mtp) || rw.jsonAppend(j, "test.json", []models.MetaAppend{{Field: "title", Append: " (HD)"}}, mtp) {
		t.Errorf("prefix or append applied again: %q", j["title"])
	}
	fd := &models.FileData{MDates: &models.MetadataDates{FormattedDate: "2024-05-01"}}
	edited, err := rw.jsonFieldAddDateTag(j, map[string]models.MetaDateTag{"title": {Loc: enums.DateTagLocPrefix, Format: enums.DateYyyyMmDd}}, fd)
	if err != nil || !edited || j["title"] != "[2024-05-01] [Cats] Title (HD)" {
		t.Errorf("new date tag: title = %q (edited %v, err %v), want the old tag replaced", j["title"], edited, err)
	}
}
