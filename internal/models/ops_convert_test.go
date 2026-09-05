package models

import (
	"io"
	"os"
	"testing"

	"metarr/internal/domain/enums"
	"metarr/internal/domain/logger"

	"github.com/TubarrApp/gocommon/sharedmodels"
	"github.com/TubarrApp/gocommon/sharedparsing"
)

// TestMain gives the global logger a console writer, since the zero value has none and
// any warning would dereference nil.
func TestMain(m *testing.M) {
	logger.Pl.Console = io.Discard
	os.Exit(m.Run())
}

// parseMetaForTest runs the shared parser the way ValidateAndSetMetaOps does.
func parseMetaForTest(t *testing.T, in ...string) []sharedmodels.MetaOps {
	t.Helper()

	parsed, _, err := sharedparsing.ParseMetaOps(in)
	if err != nil {
		t.Fatalf("ParseMetaOps(%v): %v", in, err)
	}
	return parsed
}

func TestMetaOpsFromShared(t *testing.T) {
	ops := parseMetaForTest(t,
		"title:set:Cats",
		"all-credits:set:Mr Cat",
		"title:append: (new)",
		"title:prefix:[CATS] ",
		"director:copy-to:writer",
		"writer:paste-from:director",
		"title:date-tag:prefix:ymd",
		"title:delete-date-tag:all:Ymd",
		"title:replace:old:new",
		"title:replace-prefix:pre:post",
		"title:replace-suffix:suf:fix",
	)

	got, err := MetaOpsFromShared(ops)
	if err != nil {
		t.Fatalf("metaOpsFromShared: %v", err)
	}

	if len(got.SetFields) != 2 {
		t.Errorf("SetFields = %+v, want 2 entries", got.SetFields)
	}
	if v := got.SetOverrides[enums.OverrideMetaCredits]; v != "Mr Cat" {
		t.Errorf("credits override = %q, want %q", v, "Mr Cat")
	}
	if len(got.Appends) != 1 || got.Appends[0].Append != " (new)" {
		t.Errorf("Appends = %+v", got.Appends)
	}
	if len(got.Prefixes) != 1 || got.Prefixes[0].Prefix != "[CATS] " {
		t.Errorf("Prefixes = %+v", got.Prefixes)
	}
	if len(got.CopyToFields) != 1 || got.CopyToFields[0].Dest != "writer" {
		t.Errorf("CopyToFields = %+v", got.CopyToFields)
	}
	if len(got.PasteFromFields) != 1 || got.PasteFromFields[0].Origin != "director" {
		t.Errorf("PasteFromFields = %+v", got.PasteFromFields)
	}

	dt, ok := got.DateTags["title"]
	if !ok || dt.Loc != enums.DateTagLocPrefix || dt.Format != enums.DateYyMmDd {
		t.Errorf("DateTags[title] = %+v (ok=%v)", dt, ok)
	}
	ddt, ok := got.DeleteDateTags["title"]
	if !ok || ddt.Loc != enums.DateTagLocAll || ddt.Format != enums.DateYyyyMmDd {
		t.Errorf("DeleteDateTags[title] = %+v (ok=%v)", ddt, ok)
	}

	if len(got.Replaces) != 1 || got.Replaces[0].Value != "old" || got.Replaces[0].Replacement != "new" {
		t.Errorf("Replaces = %+v", got.Replaces)
	}
	if len(got.ReplacePrefixes) != 1 || got.ReplacePrefixes[0].Prefix != "pre" {
		t.Errorf("ReplacePrefixes = %+v", got.ReplacePrefixes)
	}
	if len(got.ReplaceSuffixes) != 1 || got.ReplaceSuffixes[0].Suffix != "suf" {
		t.Errorf("ReplaceSuffixes = %+v", got.ReplaceSuffixes)
	}
}

// TestMetaOpTypeIsCaseInsensitive guards the tolerance Metarr has always had, now
// provided by the shared parser so Tubarr accepts the same input.
func TestMetaOpTypeIsCaseInsensitive(t *testing.T) {
	ops := parseMetaForTest(t, "title:SET:Cats", "title:DATE-TAG:PREFIX:ymd")

	got, err := MetaOpsFromShared(ops)
	if err != nil {
		t.Fatalf("metaOpsFromShared: %v", err)
	}
	if len(got.SetFields) != 1 || got.SetFields[0].Value != "Cats" {
		t.Errorf("SetFields = %+v", got.SetFields)
	}
	if dt, ok := got.DateTags["title"]; !ok || dt.Loc != enums.DateTagLocPrefix {
		t.Errorf("DateTags[title] = %+v (ok=%v)", dt, ok)
	}
}

// TestMetaOpsEscapedValues checks that values holding grammar characters arrive intact.
func TestMetaOpsEscapedValues(t *testing.T) {
	ops := parseMetaForTest(t, `title:set:Cats\: The Sequel`, `title:replace:a\|b:c`)

	got, err := MetaOpsFromShared(ops)
	if err != nil {
		t.Fatalf("metaOpsFromShared: %v", err)
	}
	if got.SetFields[0].Value != "Cats: The Sequel" {
		t.Errorf("set value = %q, want %q", got.SetFields[0].Value, "Cats: The Sequel")
	}
	if got.Replaces[0].Value != "a|b" {
		t.Errorf("replace find = %q, want %q", got.Replaces[0].Value, "a|b")
	}
}

// TestMetaDateTagRejectsAll keeps date-tag restricted to prefix or suffix, unlike
// delete-date-tag.
func TestMetaDateTagRejectsAll(t *testing.T) {
	ops := parseMetaForTest(t, "title:date-tag:all:ymd")

	if _, err := MetaOpsFromShared(ops); err == nil {
		t.Error("date-tag with location 'all' should be rejected")
	}
}

func TestFilenameOpsFromShared(t *testing.T) {
	in := []string{
		"prefix:[DOG VIDEOS]",
		"append:(new)",
		"date-tag:prefix:ymd",
		"delete-date-tag:all:Ymd",
		"replace:old:new",
		"replace-prefix:pre:post",
		"replace-suffix:_1:",
	}

	parsed, _, err := sharedparsing.ParseFilenameOps(in)
	if err != nil {
		t.Fatalf("ParseFilenameOps: %v", err)
	}

	got, err := FilenameOpsFromShared(parsed)
	if err != nil {
		t.Fatalf("filenameOpsFromShared: %v", err)
	}

	if len(got.Prefixes) != 1 || got.Prefixes[0].Value != "[DOG VIDEOS]" {
		t.Errorf("Prefixes = %+v", got.Prefixes)
	}
	if len(got.Appends) != 1 || got.Appends[0].Value != "(new)" {
		t.Errorf("Appends = %+v", got.Appends)
	}
	if got.DateTag.Loc != enums.DateTagLocPrefix || got.DateTag.DateFormat != enums.DateYyMmDd {
		t.Errorf("DateTag = %+v", got.DateTag)
	}
	if got.DeleteDateTags.Loc != enums.DateTagLocAll {
		t.Errorf("DeleteDateTags = %+v", got.DeleteDateTags)
	}
	if len(got.Replaces) != 1 || got.Replaces[0].FindString != "old" {
		t.Errorf("Replaces = %+v", got.Replaces)
	}
	if len(got.ReplacePrefixes) != 1 || got.ReplacePrefixes[0].Prefix != "pre" {
		t.Errorf("ReplacePrefixes = %+v", got.ReplacePrefixes)
	}
	if len(got.ReplaceSuffixes) != 1 || got.ReplaceSuffixes[0].Suffix != "_1" || got.ReplaceSuffixes[0].Replacement != "" {
		t.Errorf("ReplaceSuffixes = %+v", got.ReplaceSuffixes)
	}
}

// TestFilenameOpsRejectsTwoSets keeps the one-set-per-run rule. Two sets differ in
// value, so the shared parser passes both through for the converter to reject.
func TestFilenameOpsRejectsTwoSets(t *testing.T) {
	in := []string{"set:one", "set:two"}

	parsed, _, err := sharedparsing.ParseFilenameOps(in)
	if err != nil {
		t.Fatalf("ParseFilenameOps(%v): %v", in, err)
	}
	if _, err := FilenameOpsFromShared(parsed); err == nil {
		t.Errorf("%v should be rejected as more than one set per run", in)
	}
}

// TestFilenameOpsCollapseSecondDateTag documents that a conflicting second date tag is
// dropped by the shared parser, whose uniqueness key for date tags is the operation
// type alone, rather than reaching the converter's one-per-run guard.
func TestFilenameOpsCollapseSecondDateTag(t *testing.T) {
	for _, in := range [][]string{
		{"date-tag:prefix:ymd", "date-tag:suffix:Ymd"},
		{"delete-date-tag:prefix:ymd", "delete-date-tag:suffix:Ymd"},
	} {
		parsed, warnings, err := sharedparsing.ParseFilenameOps(in)
		if err != nil {
			t.Fatalf("ParseFilenameOps(%v): %v", in, err)
		}
		if len(parsed) != 1 {
			t.Errorf("%v parsed to %d ops, want 1 (second collapsed)", in, len(parsed))
		}
		if len(warnings) == 0 {
			t.Errorf("%v should warn that the second entry was dropped", in)
		}
		if _, err := FilenameOpsFromShared(parsed); err != nil {
			t.Errorf("converting the surviving op failed: %v", err)
		}
	}
}

// TestMalformedOpIsFatal keeps Metarr failing fast: a typo must stop the run rather

// TestDuplicateOpIsTolerated keeps duplicates non-fatal, as they were before, since

// TestUnescapedPipeInValue covers titles such as "Cats | Dogs", which the channel URL
// prefix check previously swallowed.
func TestUnescapedPipeInValue(t *testing.T) {
	ops := parseMetaForTest(t, "title:set:Cats | Dogs")

	got, err := MetaOpsFromShared(ops)
	if err != nil {
		t.Fatalf("metaOpsFromShared: %v", err)
	}
	if len(got.SetFields) != 1 || got.SetFields[0].Value != "Cats | Dogs" {
		t.Errorf("SetFields = %+v, want value %q", got.SetFields, "Cats | Dogs")
	}
}

// TestMatchesChannel covers resolving a channel-scoped operation against a file, where
// the file's own URLs stand in for Metarr's missing channel concept.
func TestMatchesChannel(t *testing.T) {
	w := &MetadataWebData{
		WebpageURL: "https://www.google.com/watch?v=abc",
	}

	for _, target := range []string{
		"https://www.google.com/",
		"https://google.com",
		"google.com",
		"http://WWW.GOOGLE.COM/path",
	} {
		if !w.MatchesChannel(target) {
			t.Errorf("MatchesChannel(%q) = false, want true", target)
		}
	}

	for _, target := range []string{
		"https://www.youtube.com/",
		"notgoogle.com",
		"",
	} {
		if w.MatchesChannel(target) {
			t.Errorf("MatchesChannel(%q) = true, want false", target)
		}
	}
}

// TestResolveMetaOpsByChannel covers "https://www.google.com/|title:set:cat": the op
// applies to a google.com file and is dropped for anything else, while an unscoped op
// always applies.
func TestResolveMetaOpsByChannel(t *testing.T) {
	ops := []sharedmodels.MetaOps{
		{ChannelURL: "https://www.google.com/", Field: "title", OpType: "set", OpValue: "cat"},
		{Field: "director", OpType: "set", OpValue: "Mr Cat"},
	}

	tests := []struct {
		name         string
		webpageURL   string
		wantSetCount int
		wantTitle    bool
	}{
		{"matching channel", "https://www.google.com/watch?v=abc", 2, true},
		{"other channel", "https://www.youtube.com/watch?v=abc", 1, false},
		{"no url at all", "", 1, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fd := &FileData{MWebData: &MetadataWebData{WebpageURL: tt.webpageURL}}

			if err := fd.ResolveMetaOps(ops, nil, nil); err != nil {
				t.Fatalf("ResolveMetaOps: %v", err)
			}
			if len(fd.MetaOps.SetFields) != tt.wantSetCount {
				t.Fatalf("SetFields = %+v, want %d", fd.MetaOps.SetFields, tt.wantSetCount)
			}

			var sawTitle bool
			for _, f := range fd.MetaOps.SetFields {
				if f.Field == "title" {
					sawTitle = true
					if f.Value != "cat" {
						t.Errorf("title set to %q, want %q", f.Value, "cat")
					}
				}
			}
			if sawTitle != tt.wantTitle {
				t.Errorf("title op present = %v, want %v", sawTitle, tt.wantTitle)
			}
		})
	}
}

// TestResolveMetaOpsWithFilters covers the standalone filtered form:
// "title:contains:cat|title:prefix:[CAT VIDEOS] " applies the prefix only when the
// file's metadata title actually contains "cat".
func TestResolveMetaOpsWithFilters(t *testing.T) {
	filtered := []sharedmodels.FilteredMetaOps{{
		Filters: []sharedmodels.Filters{{Field: "title", FilterType: "contains", Value: "cat"}},
		MetaOps: []sharedmodels.MetaOps{{Field: "title", OpType: "prefix", OpValue: "[CAT VIDEOS] "}},
	}}

	tests := []struct {
		name       string
		meta       map[string]any
		wantPrefix bool
	}{
		{"filter matches", map[string]any{"title": "Cat Compilation"}, true},
		{"filter does not match", map[string]any{"title": "Dog Compilation"}, false},
		{"field absent", map[string]any{}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fd := &FileData{MWebData: &MetadataWebData{}}

			if err := fd.ResolveMetaOps(nil, filtered, tt.meta); err != nil {
				t.Fatalf("ResolveMetaOps: %v", err)
			}

			gotPrefix := len(fd.MetaOps.Prefixes) == 1
			if gotPrefix != tt.wantPrefix {
				t.Errorf("prefix applied = %v, want %v (prefixes: %+v)", gotPrefix, tt.wantPrefix, fd.MetaOps.Prefixes)
			}
			if gotPrefix && fd.MetaOps.Prefixes[0].Prefix != "[CAT VIDEOS] " {
				t.Errorf("prefix = %q", fd.MetaOps.Prefixes[0].Prefix)
			}
		})
	}
}

// TestResolveMetaOpsFilteredRespectsChannel keeps a filtered entry scoped to a channel
// from applying to a file from elsewhere, even when its filters match.
func TestResolveMetaOpsFilteredRespectsChannel(t *testing.T) {
	filtered := []sharedmodels.FilteredMetaOps{{
		Filters: []sharedmodels.Filters{{Field: "title", FilterType: "contains", Value: "cat"}},
		MetaOps: []sharedmodels.MetaOps{{ChannelURL: "https://www.google.com/", Field: "title", OpType: "prefix", OpValue: "[CAT] "}},
	}}
	meta := map[string]any{"title": "Cat Compilation"}

	matching := &FileData{MWebData: &MetadataWebData{WebpageURL: "https://www.google.com/watch?v=a"}}
	if err := matching.ResolveMetaOps(nil, filtered, meta); err != nil {
		t.Fatalf("ResolveMetaOps: %v", err)
	}
	if len(matching.MetaOps.Prefixes) != 1 {
		t.Errorf("expected the prefix for a google.com file, got %+v", matching.MetaOps.Prefixes)
	}

	other := &FileData{MWebData: &MetadataWebData{WebpageURL: "https://www.youtube.com/watch?v=a"}}
	if err := other.ResolveMetaOps(nil, filtered, meta); err != nil {
		t.Fatalf("ResolveMetaOps: %v", err)
	}
	if len(other.MetaOps.Prefixes) != 0 {
		t.Errorf("expected no prefix for a youtube.com file, got %+v", other.MetaOps.Prefixes)
	}
}

// TestResolveMetaOpsCombinesPlainAndFiltered checks unconditional and filter-gated
// operations both land in the same resolved model.
func TestResolveMetaOpsCombinesPlainAndFiltered(t *testing.T) {
	plain := []sharedmodels.MetaOps{{Field: "director", OpType: "set", OpValue: "Mr Cat"}}
	filtered := []sharedmodels.FilteredMetaOps{{
		Filters: []sharedmodels.Filters{{Field: "title", FilterType: "contains", Value: "cat"}},
		MetaOps: []sharedmodels.MetaOps{{Field: "title", OpType: "prefix", OpValue: "[CAT VIDEOS] "}},
	}}

	fd := &FileData{MWebData: &MetadataWebData{}}
	if err := fd.ResolveMetaOps(plain, filtered, map[string]any{"title": "Cat Compilation"}); err != nil {
		t.Fatalf("ResolveMetaOps: %v", err)
	}

	if len(fd.MetaOps.SetFields) != 1 || fd.MetaOps.SetFields[0].Field != "director" {
		t.Errorf("SetFields = %+v", fd.MetaOps.SetFields)
	}
	if len(fd.MetaOps.Prefixes) != 1 || fd.MetaOps.Prefixes[0].Field != "title" {
		t.Errorf("Prefixes = %+v", fd.MetaOps.Prefixes)
	}
}

// TestResolveFilenameOpsWithFilters covers the standalone filtered filename form:
// "title:contains:cat|prefix:[cat video] " prefixes the filename only when the file's
// metadata title contains "cat". The filter reads metadata; the operation acts on the
// filename.
func TestResolveFilenameOpsWithFilters(t *testing.T) {
	filtered := []sharedmodels.FilteredFilenameOps{{
		Filters:     []sharedmodels.Filters{{Field: "title", FilterType: "contains", Value: "cat"}},
		FilenameOps: []sharedmodels.FilenameOps{{OpType: "prefix", OpValue: "[cat video] "}},
	}}

	tests := []struct {
		name       string
		meta       map[string]any
		wantPrefix bool
	}{
		{"filter matches", map[string]any{"title": "Cat Compilation"}, true},
		{"filter does not match", map[string]any{"title": "Dog Compilation"}, false},
		{"field absent", map[string]any{}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fd := &FileData{MWebData: &MetadataWebData{}}

			if err := fd.ResolveFilenameOps(nil, filtered, tt.meta); err != nil {
				t.Fatalf("ResolveFilenameOps: %v", err)
			}

			gotPrefix := len(fd.FilenameOps.Prefixes) == 1
			if gotPrefix != tt.wantPrefix {
				t.Errorf("prefix applied = %v, want %v (prefixes: %+v)", gotPrefix, tt.wantPrefix, fd.FilenameOps.Prefixes)
			}
			if gotPrefix && fd.FilenameOps.Prefixes[0].Value != "[cat video] " {
				t.Errorf("prefix = %q", fd.FilenameOps.Prefixes[0].Value)
			}
		})
	}
}

// TestResolveFilenameOpsByChannel covers a channel-scoped plain filename operation,
// which Metarr previously ignored entirely.
func TestResolveFilenameOpsByChannel(t *testing.T) {
	ops := []sharedmodels.FilenameOps{
		{ChannelURL: "https://www.google.com/", OpType: "prefix", OpValue: "[G] "},
		{OpType: "append", OpValue: " (new)"},
	}

	matching := &FileData{MWebData: &MetadataWebData{WebpageURL: "https://www.google.com/watch?v=a"}}
	if err := matching.ResolveFilenameOps(ops, nil, nil); err != nil {
		t.Fatalf("ResolveFilenameOps: %v", err)
	}
	if len(matching.FilenameOps.Prefixes) != 1 || len(matching.FilenameOps.Appends) != 1 {
		t.Errorf("google.com file should get both ops, got prefixes=%+v appends=%+v",
			matching.FilenameOps.Prefixes, matching.FilenameOps.Appends)
	}

	other := &FileData{MWebData: &MetadataWebData{WebpageURL: "https://www.youtube.com/watch?v=a"}}
	if err := other.ResolveFilenameOps(ops, nil, nil); err != nil {
		t.Fatalf("ResolveFilenameOps: %v", err)
	}
	if len(other.FilenameOps.Prefixes) != 0 || len(other.FilenameOps.Appends) != 1 {
		t.Errorf("youtube.com file should get only the unscoped op, got prefixes=%+v appends=%+v",
			other.FilenameOps.Prefixes, other.FilenameOps.Appends)
	}
}

// TestResolveFilenameOpsSingletonConflict checks that a plain date tag plus a matching
// filtered date tag is reported rather than silently picking one.
func TestResolveFilenameOpsSingletonConflict(t *testing.T) {
	plain := []sharedmodels.FilenameOps{{OpType: "date-tag", OpLoc: "prefix", DateFormat: "ymd"}}
	filtered := []sharedmodels.FilteredFilenameOps{{
		Filters:     []sharedmodels.Filters{{Field: "title", FilterType: "contains", Value: "cat"}},
		FilenameOps: []sharedmodels.FilenameOps{{OpType: "date-tag", OpLoc: "suffix", DateFormat: "Ymd"}},
	}}

	fd := &FileData{MWebData: &MetadataWebData{}}
	if err := fd.ResolveFilenameOps(plain, filtered, map[string]any{"title": "Cat"}); err == nil {
		t.Error("two applicable date tags should be reported as a conflict")
	}
}
