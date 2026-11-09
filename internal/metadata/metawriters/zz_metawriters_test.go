package metawriters

import (
	"context"
	"encoding/xml"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"metarr/internal/abstractions"
	"metarr/internal/domain/enums"
	"metarr/internal/domain/keys"
	"metarr/internal/domain/logger"
	"metarr/internal/models"
	"metarr/internal/parsing"

	"github.com/TubarrApp/gocommon/sharedenums"
	"github.com/TubarrApp/gocommon/sharedparsing"
)

// ******************************** NFO ***********************************************************************************************

// TestMain gives the global logger a console writer, since the zero value has none.
func TestMain(m *testing.M) {
	logger.Pl.Console = io.Discard
	os.Exit(m.Run())
}

// TestOverrideXMLFields tests that "all-x" overrides set every matching tag, including nested actor names.
func TestOverrideXMLFields(t *testing.T) {
	rw := &NFOFileRW{}
	data := `<movie>
    <title>Old</title>
    <originaltitle>Old Original</originaltitle>
    <year>1999</year>
    <premiered>1999-01-01</premiered>
    <director>A</director>
    <director>B</director>
    <cast>
        <actor><name>X</name><role>Lead</role></actor>
        <actor><name>Y</name></actor>
    </cast>
    <plot>Old plot</plot>
</movie>`

	got, edited := rw.overrideXMLFields(data, map[enums.OverrideMetaType]string{
		enums.OverrideMetaTitles:  "Cats & Dogs",
		enums.OverrideMetaDates:   "2024-05-01",
		enums.OverrideMetaCredits: "Mr Cat",
	}, parsing.NewMetaTemplateParser("test.nfo"))
	if !edited {
		t.Fatal("expected edits")
	}

	for _, want := range []string{
		"<title>Cats &amp; Dogs</title>",
		"<originaltitle>Cats &amp; Dogs</originaltitle>",
		"<year>2024</year>",
		"<premiered>2024-05-01</premiered>",
		"<released>2024-05-01</released>",
		"<actor><name>Mr Cat</name><role>Lead</role></actor>",
		"<actor><name>Mr Cat</name></actor>",
		"<producer>Mr Cat</producer>",
		"<plot>Old plot</plot>",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in:\n%s", want, got)
		}
	}
	if n := strings.Count(got, "<director>Mr Cat</director>"); n != 2 {
		t.Errorf("director overrides = %d, want 2 in:\n%s", n, got)
	}
	for _, added := range []string{"<studio>Mr Cat</studio>", "<aired>2024-05-01</aired>", "<subtitle>Cats &amp; Dogs</subtitle>"} {
		if !strings.Contains(got, added) {
			t.Errorf("missing group tag %q in:\n%s", added, got)
		}
	}

	if _, edited := rw.overrideXMLFields(got, map[enums.OverrideMetaType]string{enums.OverrideMetaCredits: "Mr Cat"}, parsing.NewMetaTemplateParser("test.nfo")); edited {
		t.Error("re-applying the same override should report no edit")
	}
}

// TestOverrideXMLFieldsAddsActor tests that a credits override adds an actor when the NFO has none.
func TestOverrideXMLFieldsAddsActor(t *testing.T) {
	got, _ := (&NFOFileRW{}).overrideXMLFields("<movie>\n</movie>", map[enums.OverrideMetaType]string{enums.OverrideMetaCredits: "Mr Cat"}, parsing.NewMetaTemplateParser("test.nfo"))
	if !strings.Contains(got, "<name>Mr Cat</name>") {
		t.Errorf("expected an actor named Mr Cat in:\n%s", got)
	}
}

// TestNFODecodesBothActorForms tests that actors are read both bare (Kodi) and wrapped in <cast>.
func TestNFODecodesBothActorForms(t *testing.T) {
	var n models.NFOData
	data := `<movie><actor><name>Bare</name></actor><cast><actor><name>Wrapped</name></actor></cast></movie>`
	if err := xml.Unmarshal([]byte(data), &n); err != nil {
		t.Fatal(err)
	}
	if len(n.Actors) != 1 || n.Actors[0].Name != "Bare" {
		t.Errorf("Actors = %+v, want Bare", n.Actors)
	}
	if len(n.CastActors) != 1 || n.CastActors[0].Name != "Wrapped" {
		t.Errorf("CastActors = %+v, want Wrapped", n.CastActors)
	}
}

// TestOverrideJSONFields tests that "all-x" overrides set and add every field in the group.
func TestOverrideJSONFields(t *testing.T) {
	j := map[string]any{"title": "Old", "upload_date": "19990101", "duration": 60.0}
	if !overrideJSONFields(j, map[enums.OverrideMetaType]string{
		enums.OverrideMetaTitles: "Cats",
		enums.OverrideMetaDates:  "2024-05-01",
	}, parsing.NewMetaTemplateParser("test.json")) {
		t.Fatal("expected edits")
	}

	want := map[string]string{
		"title":         "Cats",
		"fulltitle":     "Cats",
		"subtitle":      "Cats",
		"upload_date":   "2024-05-01",
		"release_date":  "2024-05-01",
		"year":          "2024",
		"release_year":  "2024",
		"creation_time": "2024-05-01T00:00:00Z",
	}
	for k, v := range want {
		if j[k] != v {
			t.Errorf("%s = %v, want %q", k, j[k], v)
		}
	}
	if j["duration"] != 60.0 {
		t.Errorf("unrelated field changed: duration = %v", j["duration"])
	}
}

// ******************************** JSON ops ******************************************************************************************

// TestJSONOpsFillTemplates tests meta ops read template tags from the JSON, and are skipped if they can't all be filled.
func TestJSONOpsFillTemplates(t *testing.T) {
	const data = `{"title":"Title","studio":"A & B","plot":"old text"}`
	plotRegex := regexp.MustCompile("old")

	for _, tc := range []struct {
		name, field, want string
		op                func(ops *models.MetaOps, value string)
	}{
		{"set", "artist", "A & B", func(ops *models.MetaOps, value string) {
			ops.SetFields = []models.MetaSetField{{Field: "artist", Value: value}}
		}},
		{"prefix", "title", "A & BTitle", func(ops *models.MetaOps, value string) {
			ops.Prefixes = []models.MetaPrefix{{Field: "title", Prefix: value}}
		}},
		{"suffix", "title", "TitleA & B", func(ops *models.MetaOps, value string) {
			ops.Suffixes = []models.MetaSuffix{{Field: "title", Suffix: value}}
		}},
		{"replace", "plot", "A & B text", func(ops *models.MetaOps, value string) {
			ops.Replaces = []models.MetaReplace{{Field: "plot", Value: "old", Replacement: value}}
		}},
		{"replace-prefix", "plot", "A & B text", func(ops *models.MetaOps, value string) {
			ops.ReplacePrefixes = []models.MetaReplacePrefix{{Field: "plot", Prefix: "old", Replacement: value}}
		}},
		{"replace-suffix", "plot", "old A & B", func(ops *models.MetaOps, value string) {
			ops.ReplaceSuffixes = []models.MetaReplaceSuffix{{Field: "plot", Suffix: "text", Replacement: value}}
		}},
		{"regex-replace", "plot", "A & B text", func(ops *models.MetaOps, value string) {
			ops.RegexReplaces = []models.MetaRegexReplace{{Field: "plot", Regex: plotRegex, Replacement: value}}
		}},
		{"all-titles set", "title", "A & B", func(ops *models.MetaOps, value string) {
			ops.SetOverrides = map[enums.OverrideMetaType]string{enums.OverrideMetaTitles: value}
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// A tag the JSON has, then one it doesn't, alone or with one it has.
			for _, value := range []string{"{{studio}}", "{{nope}}", "{{studio}} {{nope}}"} {
				path := filepath.Join(t.TempDir(), "v.json")
				if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
					t.Fatal(err)
				}
				f, err := os.OpenFile(path, os.O_RDWR, 0o600)
				if err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() { _ = f.Close() })
				rw := NewJSONFileRW(t.Context(), f)
				if _, err := rw.DecodeJSON(f); err != nil {
					t.Fatal(err)
				}

				ops := models.NewMetaOps()
				tc.op(ops, value)
				edited, err := rw.MakeJSONEdits(f, &models.FileData{MetaOps: ops, ModelMOverwrite: true})
				if err != nil {
					t.Fatal(err)
				}

				if value == "{{studio}}" {
					if got := rw.Meta[tc.field]; !edited || got != tc.want {
						t.Errorf("%s = %q (edited %v), want %q", tc.field, got, edited, tc.want)
					}
					continue
				}
				got, err := os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
				if edited || string(got) != data {
					t.Errorf("value %q wasn't skipped, edited %v:\n%s", value, edited, got)
				}
			}
		})
	}
}

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
		"title:delete-date-tag",
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

// TestJSONFlush tests that edits reach the file only on Flush, and that Flush leaves an unedited file alone.
func TestJSONFlush(t *testing.T) {
	const original = `{"title":"Title"}`
	path := filepath.Join(t.TempDir(), "v.json")
	if err := os.WriteFile(path, []byte(original), 0o600); err != nil {
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
	read := func() string {
		t.Helper()
		b, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		return string(b)
	}

	// Unedited: nothing written.
	if err := rw.Flush(); err != nil {
		t.Fatal(err)
	}
	if got := read(); got != original {
		t.Fatalf("file = %q after flushing no edits, want it untouched", got)
	}

	// Edited: written on Flush only.
	desc := "Text"
	if _, err := rw.WriteJSON(map[string]*string{"description": &desc}); err != nil {
		t.Fatal(err)
	}
	if got := read(); got != original {
		t.Fatalf("file = %q before Flush, want it untouched", got)
	}
	if err := rw.Flush(); err != nil {
		t.Fatal(err)
	}
	if got := read(); !strings.Contains(got, `"description": "Text"`) || !strings.Contains(got, `"title": "Title"`) {
		t.Errorf("file = %q after Flush, want the new field and the old one", got)
	}
}

// ******************************** NFO ops *******************************************************************************************

// nfoTestFile writes data to a temporary NFO file and returns it open for reading and writing.
func nfoTestFile(t *testing.T, data string) *os.File {
	t.Helper()
	path := filepath.Join(t.TempDir(), "test.nfo")
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
	f, err := os.OpenFile(path, os.O_RDWR, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { f.Close() })
	return f
}

// TestMakeMetaEditsNFO tests that NFO edits apply to every occurrence, nested actor names and
// grouped fields, building on the override and escaping text.
func TestMakeMetaEditsNFO(t *testing.T) {
	data := `<movie>
    <title>Cats &amp; Dogs</title>
    <originaltitle>Cats</originaltitle>
    <director>A</director>
    <director>B</director>
    <actor><name>X</name><role>Lead</role></actor>
    <plot>[OLD] Plot</plot>
    <premiered>2022-05-06</premiered>
</movie>`
	f := nfoTestFile(t, data)

	ops := models.NewMetaOps()
	ops.Suffixes = []models.MetaSuffix{
		{Field: "all-titles", Suffix: " & more"},
		{Field: "director", Suffix: "!"},
		{Field: "actor", Suffix: " Jr"},
	}
	ops.ReplacePrefixes = []models.MetaReplacePrefix{{Field: "plot", Prefix: "[OLD] ", Replacement: "[NEW] "}}
	ops.PasteFromFields = []models.PasteFromField{{Field: "outline", Origin: "premiered"}}
	fd := &models.FileData{MetaOps: ops}

	edited, err := (&NFOFileRW{}).MakeMetaEdits(data, f, fd)
	if err != nil || !edited {
		t.Fatalf("MakeMetaEdits edited=%v err=%v", edited, err)
	}
	got, err := os.ReadFile(f.Name())
	if err != nil {
		t.Fatal(err)
	}

	for _, want := range []string{
		"<title>Cats &amp; Dogs &amp; more</title>",
		"<originaltitle>Cats &amp; more</originaltitle>",
		"<director>A!</director>",
		"<director>B!</director>",
		"<actor><name>X Jr</name><role>Lead</role></actor>",
		"<plot>[NEW] Plot</plot>",
		"<outline>2022-05-06</outline>",
	} {
		if !strings.Contains(string(got), want) {
			t.Errorf("missing %q in:\n%s", want, got)
		}
	}
}

// TestMakeMetaEditsNFODatesFrom tests that all-dates pasted from a field overrides every date tag.
func TestMakeMetaEditsNFODatesFrom(t *testing.T) {
	data := "<movie>\n<premiered>20220506</premiered>\n<year>1999</year>\n</movie>"
	f := nfoTestFile(t, data)

	ops := models.NewMetaOps()
	ops.DatesFrom = "premiered"
	fd := &models.FileData{MetaOps: ops}

	if _, err := (&NFOFileRW{}).MakeMetaEdits(data, f, fd); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(f.Name())
	for _, want := range []string{"<premiered>2022-05-06</premiered>", "<year>2022</year>", "<released>2022-05-06</released>"} {
		if !strings.Contains(string(got), want) {
			t.Errorf("missing %q in:\n%s", want, got)
		}
	}
	if fd.MetaOps.SetOverrides[enums.OverrideMetaDates] != "2022-05-06" {
		t.Errorf("dates override = %q, want 2022-05-06", fd.MetaOps.SetOverrides[enums.OverrideMetaDates])
	}
}

// TestResolveDatesFromRejectsNonDate tests that a non-date source leaves the dates override unset.
func TestResolveDatesFromRejectsNonDate(t *testing.T) {
	ops := models.NewMetaOps()
	ops.DatesFrom = "title"
	resolveDatesFrom(ops, "Cats")
	if _, ok := ops.SetOverrides[enums.OverrideMetaDates]; ok {
		t.Error("non-date value should not set the dates override")
	}
}

// TestDateTagEditsNFO tests that NFO date tags strip old tags and add new ones to grouped fields.
func TestDateTagEditsNFO(t *testing.T) {
	data := "<movie>\n<title>[1999-01-01] Cats</title>\n<originaltitle>Cats</originaltitle>\n</movie>"
	f := nfoTestFile(t, data)

	ops := models.NewMetaOps()
	ops.DeleteDateTags["title"] = struct{}{}
	ops.DateTags["all-titles"] = models.MetaDateTag{Loc: sharedenums.DateTagLocPrefix, Format: sharedenums.DateYyyyMmDd}
	fd := &models.FileData{MetaOps: ops, MDates: &models.MetadataDates{FormattedDate: "2024-05-01"}}

	edited, err := (&NFOFileRW{Meta: data}).DateTagEdits(f, fd)
	if err != nil || !edited {
		t.Fatalf("DateTagEdits edited=%v err=%v", edited, err)
	}
	got, _ := os.ReadFile(f.Name())
	for _, want := range []string{"<title>[2024-05-01] Cats</title>", "<originaltitle>[2024-05-01] Cats</originaltitle>"} {
		if !strings.Contains(string(got), want) {
			t.Errorf("missing %q in:\n%s", want, got)
		}
	}
}

// TestNFOOpsRerun tests that meta ops run again on an edited NFO don't stack prefixes or suffixes, and a date tag op
// replaces the tag already there.
func TestNFOOpsRerun(t *testing.T) {
	data := "<movie>\n<title>[2023-11-05] [Cats] Cats (HD)</title>\n</movie>"
	rw := &NFOFileRW{}
	mtp := parsing.NewMetaTemplateParser("test.nfo")
	if _, edited := rw.xmlPrefix(data, []models.MetaPrefix{{Field: "title", Prefix: "[Cats] "}}, mtp); edited {
		t.Error("prefix applied again")
	}
	if _, edited := rw.xmlSuffix(data, []models.MetaSuffix{{Field: "title", Suffix: " (HD)"}}, mtp); edited {
		t.Error("suffix applied again")
	}

	f := nfoTestFile(t, data)
	ops := models.NewMetaOps()
	ops.DateTags["title"] = models.MetaDateTag{Loc: sharedenums.DateTagLocPrefix, Format: sharedenums.DateYyyyMmDd}
	fd := &models.FileData{MetaOps: ops, MDates: &models.MetadataDates{FormattedDate: "2024-05-01"}}
	if edited, err := (&NFOFileRW{Meta: data}).DateTagEdits(f, fd); err != nil || !edited {
		t.Fatalf("DateTagEdits edited=%v err=%v", edited, err)
	}
	got, _ := os.ReadFile(f.Name())
	if want := "<title>[2024-05-01] [Cats] Cats (HD)</title>"; !strings.Contains(string(got), want) {
		t.Errorf("missing %q in:\n%s", want, got)
	}
}

// TestMakeMetaEditsNFOOrder tests that a date tag is deleted before prefixes are added, and that prefixes read in the
// order they were written.
func TestMakeMetaEditsNFOOrder(t *testing.T) {
	data := "<movie>\n<title>[2023-11-05] Cats</title>\n</movie>"
	f := nfoTestFile(t, data)
	ops := models.NewMetaOps()
	ops.Prefixes = []models.MetaPrefix{{Field: "title", Prefix: "[A] "}, {Field: "title", Prefix: "[B] "}}
	ops.DeleteDateTags["title"] = struct{}{}

	if edited, err := (&NFOFileRW{}).MakeMetaEdits(data, f, &models.FileData{MetaOps: ops}); err != nil || !edited {
		t.Fatalf("MakeMetaEdits edited=%v err=%v", edited, err)
	}
	got, _ := os.ReadFile(f.Name())
	if want := "<title>[A] [B] Cats</title>"; !strings.Contains(string(got), want) {
		t.Errorf("missing %q in:\n%s", want, got)
	}
}

// ******************************** Set and lines *************************************************************************************

// TestSetJSONFieldTemplates tests a set whose template tags can't all be filled is skipped, rather than writing the tags in.
func TestSetJSONFieldTemplates(t *testing.T) {
	mtp := parsing.NewMetaTemplateParser("test.json")
	for _, tc := range []struct {
		name, value, want string
		edited            bool
	}{
		{"plain text", "Someone", "Someone", true},
		{"filled tag", "By {{title}}", "By Title", true},
		{"missing tag", "{{nope}}", "", false},
		{"partly filled", "{{title}} {{nope}}", "", false},
		{"lone braces", "a {{ b", "a {{ b", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			j := map[string]any{"title": "Title"}
			rw := &JSONFileRW{ctx: t.Context()}
			edited, err := rw.setJSONField(j, "test.json", true, []models.MetaSetField{{Field: "artist", Value: tc.value}}, mtp)
			if err != nil {
				t.Fatal(err)
			}
			got, _ := j["artist"].(string)
			if edited != tc.edited || got != tc.want {
				t.Errorf("artist = %q (edited %v), want %q (edited %v)", got, edited, tc.want, tc.edited)
			}
		})
	}
}

// TestJSONDateTagsKeepLines tests adding or deleting a date tag leaves the field's line breaks alone.
func TestJSONDateTagsKeepLines(t *testing.T) {
	const text = "line one\n\nline two"
	rw := &JSONFileRW{}
	fd := &models.FileData{MDates: &models.MetadataDates{FormattedDate: "2024-05-01"}}

	j := map[string]any{"description": text}
	add := map[string]models.MetaDateTag{"description": {Loc: sharedenums.DateTagLocPrefix, Format: sharedenums.DateYyyyMmDd}}
	if edited, err := rw.jsonFieldAddDateTag(j, add, fd); err != nil || !edited || j["description"] != "[2024-05-01] "+text {
		t.Errorf("after adding: %q (edited %v, err %v)", j["description"], edited, err)
	}

	del := map[string]struct{}{"description": {}}
	if edited, err := rw.jsonFieldDeleteDateTag(j, del, fd); err != nil || !edited || j["description"] != text {
		t.Errorf("after deleting: %q (edited %v, err %v)", j["description"], edited, err)
	}

	// With no tag there's nothing to do.
	if edited, err := rw.jsonFieldDeleteDateTag(j, del, fd); err != nil || edited || j["description"] != text {
		t.Errorf("with no tag: %q (edited %v, err %v)", j["description"], edited, err)
	}
}

// TestNFODateTagsKeepLines tests deleting a date tag from an NFO field leaves its line breaks alone.
func TestNFODateTagsKeepLines(t *testing.T) {
	data := "<movie>\n<plot>[2024-05-01] line one\n\nline two</plot>\n</movie>"
	f := nfoTestFile(t, data)
	ops := models.NewMetaOps()
	ops.DeleteDateTags = map[string]struct{}{"plot": {}}

	edited, err := (&NFOFileRW{}).MakeMetaEdits(data, f, &models.FileData{MetaOps: ops})
	if err != nil || !edited {
		t.Fatalf("edited=%v err=%v", edited, err)
	}
	got, err := os.ReadFile(f.Name())
	if err != nil {
		t.Fatal(err)
	}

	// Read back as XML, which may write a line break as a character reference.
	var parsed models.NFOData
	if err := xml.Unmarshal(got, &parsed); err != nil {
		t.Fatalf("not valid XML: %v\n%s", err, got)
	}
	if want := "line one\n\nline two"; parsed.Plot != want {
		t.Errorf("plot = %q, want %q", parsed.Plot, want)
	}
}

// TestNFOSetOverwrites tests the overwrite setting replaces a field already there, with no prompt.
func TestNFOSetOverwrites(t *testing.T) {
	t.Cleanup(func() { abstractions.Set(keys.MOverwrite, false) })
	abstractions.Set(keys.MOverwrite, true)

	// A prompt would need the context and file left unset here, and fail the test.
	mtp := parsing.NewMetaTemplateParser("test.nfo")
	got, edited, err := (&NFOFileRW{}).addNewXMLFields("<movie>\n<title>Old</title>\n</movie>", false, []models.MetaSetField{{Field: "title", Value: "New"}}, mtp)
	if err != nil || !edited || !strings.Contains(got, "<title>New</title>") {
		t.Errorf("got %q (edited %v, err %v)", got, edited, err)
	}
}

// TestNFOSetEscapes tests set values are escaped however they're written in, keeping the file valid XML.
func TestNFOSetEscapes(t *testing.T) {
	rw := &NFOFileRW{}
	mtp := parsing.NewMetaTemplateParser("test.nfo")
	got, edited, err := rw.addNewXMLFields("<movie>\n<title>Old</title>\n</movie>", true, []models.MetaSetField{
		{Field: "title", Value: "Cats & Dogs"},
		{Field: "studio", Value: "A < B"},
		{Field: "actor", Value: "Tom & Jerry"},
	}, mtp)
	if err != nil || !edited {
		t.Fatalf("edited=%v err=%v", edited, err)
	}
	for _, want := range []string{"<title>Cats &amp; Dogs</title>", "<studio>A &lt; B</studio>", "<name>Tom &amp; Jerry</name>"} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in:\n%s", want, got)
		}
	}
	var parsed models.NFOData
	if err := xml.Unmarshal([]byte(got), &parsed); err != nil {
		t.Errorf("not valid XML: %v\n%s", err, got)
	}

	// The same actor isn't added twice.
	if _, edited, err := rw.addNewXMLFields(got, true, []models.MetaSetField{{Field: "actor", Value: "Tom & Jerry"}}, mtp); err != nil || edited {
		t.Errorf("adding the actor again: edited %v, err %v", edited, err)
	}
}

// TestNFORootsKeptOnEdit tests an NFO is read and edited under its own root element, whichever it is, with no other root added.
func TestNFORootsKeptOnEdit(t *testing.T) {
	for _, root := range []string{models.NFORootMovie, models.NFORootEpisode, "musicvideo"} {
		t.Run(root, func(t *testing.T) {
			f := nfoTestFile(t, "<"+root+">\n<title>Old</title>\n</"+root+">\n")
			rw := NewNFOFileRW(t.Context(), f)
			decoded, err := rw.DecodeMetadata(f)
			if err != nil {
				t.Fatal(err)
			}
			if decoded.Title.PlainText != "Old" {
				t.Errorf("title = %q", decoded.Title.PlainText)
			}

			ops := models.NewMetaOps()
			ops.SetFields = []models.MetaSetField{{Field: "studio", Value: "Studio"}, {Field: "actor", Value: "Actor"}}
			edited, err := rw.MakeMetaEdits(rw.Meta, f, &models.FileData{MetaOps: ops, ModelMOverwrite: true})
			if err != nil || !edited {
				t.Fatalf("edited=%v err=%v", edited, err)
			}
			got, err := os.ReadFile(f.Name())
			if err != nil {
				t.Fatal(err)
			}

			// One root only, as xml.Unmarshal wouldn't notice a second.
			closers := strings.Count(string(got), "</movie>") + strings.Count(string(got), "</episodedetails>") + strings.Count(string(got), "</musicvideo>")
			if closers != 1 {
				t.Errorf("%d roots in:\n%s", closers, got)
			}
			var parsed models.NFOData
			if err := xml.Unmarshal(got, &parsed); err != nil {
				t.Fatalf("not valid XML: %v\n%s", err, got)
			}
			if parsed.XMLName.Local != root || len(parsed.Studios) != 1 || len(parsed.CastActors) != 1 {
				t.Errorf("root %q, studios %v, actors %v, in:\n%s", parsed.XMLName.Local, parsed.Studios, parsed.CastActors, got)
			}
		})
	}
}

// TestNFOOpsFillTemplates tests meta ops read template tags from the NFO, and are skipped if they can't all be filled.
func TestNFOOpsFillTemplates(t *testing.T) {
	const data = "<movie>\n<title>Title</title>\n<studio>A &amp; B</studio>\n<plot>old text</plot>\n</movie>\n"
	plotRegex := regexp.MustCompile("old")

	for _, tc := range []struct {
		name string
		op   func(ops *models.MetaOps, value string)
		want string
	}{
		{"set", func(ops *models.MetaOps, value string) {
			ops.SetFields = []models.MetaSetField{{Field: "artist", Value: value}}
		}, "<artist>A &amp; B</artist>"},
		{"prefix", func(ops *models.MetaOps, value string) {
			ops.Prefixes = []models.MetaPrefix{{Field: "title", Prefix: value}}
		}, "<title>A &amp; BTitle</title>"},
		{"suffix", func(ops *models.MetaOps, value string) {
			ops.Suffixes = []models.MetaSuffix{{Field: "title", Suffix: value}}
		}, "<title>TitleA &amp; B</title>"},
		{"replace", func(ops *models.MetaOps, value string) {
			ops.Replaces = []models.MetaReplace{{Field: "plot", Value: "old", Replacement: value}}
		}, "<plot>A &amp; B text</plot>"},
		{"replace-prefix", func(ops *models.MetaOps, value string) {
			ops.ReplacePrefixes = []models.MetaReplacePrefix{{Field: "plot", Prefix: "old", Replacement: value}}
		}, "<plot>A &amp; B text</plot>"},
		{"replace-suffix", func(ops *models.MetaOps, value string) {
			ops.ReplaceSuffixes = []models.MetaReplaceSuffix{{Field: "plot", Suffix: "text", Replacement: value}}
		}, "<plot>old A &amp; B</plot>"},
		{"regex-replace", func(ops *models.MetaOps, value string) {
			ops.RegexReplaces = []models.MetaRegexReplace{{Field: "plot", Regex: plotRegex, Replacement: value}}
		}, "<plot>A &amp; B text</plot>"},
		{"all-titles set", func(ops *models.MetaOps, value string) {
			ops.SetOverrides = map[enums.OverrideMetaType]string{enums.OverrideMetaTitles: value}
		}, "<title>A &amp; B</title>"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// A tag the NFO has, then one it doesn't, alone or with one it has.
			for _, value := range []string{"{{studio}}", "{{nope}}", "{{studio}} {{nope}}"} {
				f := nfoTestFile(t, data)
				ops := models.NewMetaOps()
				tc.op(ops, value)
				edited, err := (&NFOFileRW{}).MakeMetaEdits(data, f, &models.FileData{MetaOps: ops, ModelMOverwrite: true})
				if err != nil {
					t.Fatal(err)
				}
				got, err := os.ReadFile(f.Name())
				if err != nil {
					t.Fatal(err)
				}

				if value == "{{studio}}" {
					if !edited || !strings.Contains(string(got), tc.want) {
						t.Errorf("edited %v, missing %q in:\n%s", edited, tc.want, got)
					}
					continue
				}
				if edited || string(got) != data {
					t.Errorf("value %q wasn't skipped, edited %v:\n%s", value, edited, got)
				}
			}
		})
	}
}

// TestNFOFields tests each tag gives its text unescaped, a repeated tag's joined by commas, with an actor's being their name.
func TestNFOFields(t *testing.T) {
	data := `<?xml version="1.0" encoding="UTF-8"?>
<movie>
    <title>Cats &amp; Dogs</title>
    <director>First</director>
    <director>Second</director>
    <cast>
        <actor><name>Actor One</name><role>Lead</role></actor>
        <actor><name>Actor Two</name></actor>
    </cast>
    <plot></plot>
</movie>`
	got := NFOFields(data)
	for tag, want := range map[string]string{
		"title":    "Cats & Dogs",
		"director": "First, Second",
		"actor":    "Actor One, Actor Two",
		"role":     "Lead",
	} {
		if got[tag] != want {
			t.Errorf("%s = %v, want %q", tag, got[tag], want)
		}
	}
	for _, tag := range []string{"plot", "name", "movie", "cast"} {
		if val, found := got[tag]; found {
			t.Errorf("%s = %v, want none", tag, val)
		}
	}

	if got := NFOFields("not xml"); len(got) != 0 {
		t.Errorf("fields from text that isn't XML: %v", got)
	}
}

// TestNFOWithNoRootGetsMovieRoot tests a file with no root element is given a movie's, as before.
func TestNFOWithNoRootGetsMovieRoot(t *testing.T) {
	for _, data := range []string{"", `<?xml version="1.0" encoding="UTF-8"?>`, "not xml"} {
		got := (&NFOFileRW{}).ensureXMLStructure(data)
		if !strings.Contains(got, "<movie>\n</movie>") {
			t.Errorf("ensureXMLStructure(%q) = %q, want a movie root", data, got)
		}
	}
}
