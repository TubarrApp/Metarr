package metawriters

import (
	"encoding/xml"
	"os"
	"regexp"
	"strings"
	"testing"

	"metarr/internal/abstractions"
	"metarr/internal/domain/enums"
	"metarr/internal/domain/keys"
	"metarr/internal/models"
	"metarr/internal/parsing"

	"github.com/TubarrApp/gocommon/sharedenums"
)

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

	del := map[string]models.MetaDeleteDateTag{"description": {Loc: sharedenums.DateTagLocAll, Format: sharedenums.DateYyyyMmDd}}
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
	ops.DeleteDateTags = map[string]models.MetaDeleteDateTag{"plot": {Loc: sharedenums.DateTagLocAll, Format: sharedenums.DateYyyyMmDd}}

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
