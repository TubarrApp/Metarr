package metawriters

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"metarr/internal/models"
	"metarr/internal/parsing"

	"github.com/TubarrApp/gocommon/sharedenums"
	"github.com/TubarrApp/gocommon/sharedparsing"
)

// TestJSONOpsRerun tests that meta ops run again on edited JSON don't stack prefixes or suffixes, and a date tag op
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
	if rw.jsonPrefix(j, "test.json", prefix, mtp) || rw.jsonSuffix(j, "test.json", []models.MetaSuffix{{Field: "title", Suffix: " (HD)"}}, mtp) {
		t.Errorf("prefix or suffix applied again: %q", j["title"])
	}
	fd := &models.FileData{MDates: &models.MetadataDates{FormattedDate: "2024-05-01"}}
	edited, err := rw.jsonFieldAddDateTag(j, map[string]models.MetaDateTag{"title": {Loc: sharedenums.DateTagLocPrefix, Format: sharedenums.DateYyyyMmDd}}, fd)
	if err != nil || !edited || j["title"] != "[2024-05-01] [Cats] Title (HD)" {
		t.Errorf("new date tag: title = %q (edited %v, err %v), want the old tag replaced", j["title"], edited, err)
	}
}

// TestMakeJSONEditsOrder tests that a date tag is deleted before a prefix is added, and that a prefix replacement's
// template tags are filled.
func TestMakeJSONEditsOrder(t *testing.T) {
	parsed, _, err := sharedparsing.ParseMetaOps([]string{
		"title:prefix:[Cats] ",
		"title:delete-date-tag:prefix:Ymd",
		"description:replace-prefix:[OLD] :{{uploader}}: ",
	})
	if err != nil {
		t.Fatal(err)
	}
	ops, err := models.MetaOpsFromShared(parsed)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "v.json")
	if err := os.WriteFile(path, []byte(`{"title":"[2023-11-05] Title","description":"[OLD] Text","uploader":"Bob"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	f, err := os.OpenFile(path, os.O_RDWR, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = f.Close() })
	rw := NewJSONFileRW(context.Background(), f)
	if _, err := rw.DecodeJSON(f); err != nil {
		t.Fatal(err)
	}

	if _, err := rw.MakeJSONEdits(f, &models.FileData{MetaOps: ops}); err != nil {
		t.Fatal(err)
	}
	if got := rw.Meta["title"]; got != "[Cats] Title" {
		t.Errorf("title = %q, want the date tag deleted before the prefix", got)
	}
	if got := rw.Meta["description"]; got != "Bob: Text" {
		t.Errorf("description = %q, want the replacement's template filled", got)
	}
}
