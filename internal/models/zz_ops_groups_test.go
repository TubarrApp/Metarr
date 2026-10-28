package models

import (
	"slices"
	"testing"

	"github.com/TubarrApp/gocommon/sharedenums"
)

// TestExpandPlainFields tests that an edit to a plain field also covers its other forms, while a named form stays alone.
func TestExpandPlainFields(t *testing.T) {
	ops := NewMetaOps()
	ops.Prefixes = []MetaPrefix{{Field: "title", Prefix: "[A] "}, {Field: "fulltitle", Prefix: "[B] "}}
	ops.Cuts = []MetaCut{{Field: "description"}}
	ops.SetFields = []MetaSetField{{Field: "year", Value: "2020"}, {Field: "artist", Value: "Band"}}

	for _, c := range []struct {
		name                 string
		groups               map[string][]string
		prefixes, cuts, sets []string
	}{
		{
			name:     "json",
			groups:   JSONGroupFields,
			prefixes: []string{"title", "fulltitle", "fulltitle"},
			cuts:     []string{"longdescription", "long_description", "description", "synopsis", "summary", "comment"},
			sets:     []string{"year", "release_year", "artist", "album_artist"},
		},
		{
			name:     "nfo",
			groups:   NFOGroupFields,
			prefixes: []string{"title", "originaltitle", "fulltitle"},
			cuts:     []string{"plot", "outline", "description"},
			sets:     []string{"year", "artist"},
		},
	} {
		got := ops.ExpandGroups(c.groups)
		if f := fieldsOf(got.Prefixes, func(o MetaPrefix) string { return o.Field }); !slices.Equal(f, c.prefixes) {
			t.Errorf("%s prefixes = %v, want %v", c.name, f, c.prefixes)
		}
		if f := fieldsOf(got.Cuts, func(o MetaCut) string { return o.Field }); !slices.Equal(f, c.cuts) {
			t.Errorf("%s cuts = %v, want %v", c.name, f, c.cuts)
		}
		if f := fieldsOf(got.SetFields, func(o MetaSetField) string { return o.Field }); !slices.Equal(f, c.sets) {
			t.Errorf("%s sets = %v, want %v", c.name, f, c.sets)
		}
	}
}

// TestExpandGroupKeysNarrowestWins tests that a field's date tag beats a group's, and a smaller group's beats a larger one's.
func TestExpandGroupKeysNarrowestWins(t *testing.T) {
	ops := NewMetaOps()
	ops.DateTags["all-titles"] = MetaDateTag{Loc: sharedenums.DateTagLocPrefix}
	ops.DateTags["title"] = MetaDateTag{Loc: sharedenums.DateTagLocSuffix}
	ops.DateTags["fulltitle"] = MetaDateTag{Loc: sharedenums.DateTagLocAll}

	// Map order is random, so repeat to catch an order the result depends on.
	for range 50 {
		got := ops.ExpandGroups(JSONGroupFields).DateTags
		if got["subtitle"].Loc != sharedenums.DateTagLocPrefix || got["title"].Loc != sharedenums.DateTagLocSuffix ||
			got["fulltitle"].Loc != sharedenums.DateTagLocAll {
			t.Fatalf("DateTags = %+v, want subtitle from all-titles, title from title, fulltitle its own", got)
		}
	}
}

// fieldsOf returns the field each operation edits.
func fieldsOf[T any](ops []T, field func(T) string) []string {
	out := make([]string, 0, len(ops))
	for _, o := range ops {
		out = append(out, field(o))
	}
	return out
}
