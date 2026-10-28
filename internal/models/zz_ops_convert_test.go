package models

import (
	"io"
	"os"
	"slices"
	"testing"

	"metarr/internal/domain/enums"
	"metarr/internal/domain/logger"

	"github.com/TubarrApp/gocommon/sharedenums"
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

// TestMetaOpsFromSharedDatesFrom tests that pasting into or copying to all-dates records its source field.
func TestMetaOpsFromSharedDatesFrom(t *testing.T) {
	for _, op := range []string{"all-dates:paste-from:upload_date", "upload_date:copy-to:all-dates"} {
		got, err := MetaOpsFromShared(parseMetaForTest(t, op))
		if err != nil {
			t.Fatalf("%s: %v", op, err)
		}
		if got.DatesFrom != "upload_date" || len(got.PasteFromFields) != 0 || len(got.CopyToFields) != 0 {
			t.Errorf("%s: DatesFrom = %q, paste = %+v, copy = %+v", op, got.DatesFrom, got.PasteFromFields, got.CopyToFields)
		}
	}
}

// TestExpandGroups tests that group fields and copy-to destinations expand per format, with
// single-field date tags taking precedence over their group's.
func TestExpandGroups(t *testing.T) {
	ops := NewMetaOps()
	ops.Suffixes = []MetaSuffix{{Field: "all-titles", Suffix: "!"}, {Field: "writer", Suffix: "?"}}
	ops.CopyToFields = []CopyToField{{Field: "release_date", Dest: "all-titles"}}
	ops.DateTags["all-titles"] = MetaDateTag{Loc: sharedenums.DateTagLocPrefix}
	ops.DateTags["title"] = MetaDateTag{Loc: sharedenums.DateTagLocSuffix}

	got := ops.ExpandGroups(JSONGroupFields)

	var suffixFields []string
	for _, s := range got.Suffixes {
		suffixFields = append(suffixFields, s.Field)
	}
	if want := []string{"title", "fulltitle", "subtitle", "writer"}; !slices.Equal(suffixFields, want) {
		t.Errorf("Suffixes fields = %v, want %v", suffixFields, want)
	}
	if len(got.CopyToFields) != 3 || got.CopyToFields[1].Dest != "fulltitle" || got.CopyToFields[1].Field != "release_date" {
		t.Errorf("CopyToFields = %+v", got.CopyToFields)
	}
	if got.DateTags["title"].Loc != sharedenums.DateTagLocSuffix || got.DateTags["subtitle"].Loc != sharedenums.DateTagLocPrefix {
		t.Errorf("DateTags = %+v", got.DateTags)
	}
	if _, ok := got.DateTags["all-titles"]; ok {
		t.Error("group key should not survive expansion")
	}
	if len(ops.Suffixes) != 2 {
		t.Errorf("original Suffixes modified: %+v", ops.Suffixes)
	}

	nfo := ops.ExpandGroups(NFOGroupFields)
	if len(nfo.Suffixes) != 5 || nfo.Suffixes[1].Field != "originaltitle" {
		t.Errorf("NFO Suffixes = %+v", nfo.Suffixes)
	}
}

func TestMetaOpsFromShared(t *testing.T) {
	ops := parseMetaForTest(t,
		"title:set:Cats",
		"all-credits:set:Mr Cat",
		"title:suffix: (new)",
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

	if len(got.SetFields) != 1 || got.SetFields[0].Field != "title" {
		t.Errorf("SetFields = %+v, want only the title entry", got.SetFields)
	}
	if v := got.SetOverrides[enums.OverrideMetaCredits]; v != "Mr Cat" {
		t.Errorf("credits override = %q, want %q", v, "Mr Cat")
	}
	if len(got.Suffixes) != 1 || got.Suffixes[0].Suffix != " (new)" {
		t.Errorf("Suffixes = %+v", got.Suffixes)
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
	if !ok || dt.Loc != sharedenums.DateTagLocPrefix || dt.Format != sharedenums.DateYyMmDd {
		t.Errorf("DateTags[title] = %+v (ok=%v)", dt, ok)
	}
	ddt, ok := got.DeleteDateTags["title"]
	if !ok || ddt.Loc != sharedenums.DateTagLocAll || ddt.Format != sharedenums.DateYyyyMmDd {
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
	if dt, ok := got.DateTags["title"]; !ok || dt.Loc != sharedenums.DateTagLocPrefix {
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
	// Built directly, since parsing now rejects this before conversion.
	ops := []sharedmodels.MetaOps{{Field: "title", OpType: "date-tag", OpLoc: "all", DateFormat: "ymd"}}

	if _, err := MetaOpsFromShared(ops); err == nil {
		t.Error("date-tag with location 'all' should be rejected")
	}
}

func TestFilenameOpsFromShared(t *testing.T) {
	in := []string{
		"prefix:[DOG VIDEOS]",
		"suffix:(new)",
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
	if len(got.Suffixes) != 1 || got.Suffixes[0].Value != "(new)" {
		t.Errorf("Suffixes = %+v", got.Suffixes)
	}
	if got.DateTag.Loc != sharedenums.DateTagLocPrefix || got.DateTag.DateFormat != sharedenums.DateYyMmDd {
		t.Errorf("DateTag = %+v", got.DateTag)
	}
	if got.DeleteDateTags.Loc != sharedenums.DateTagLocAll {
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

// TestResolveMetaOpsByChannelURLFilter covers "channel_url:contains:<URL>||op", how an op is limited to one channel's
// videos: it applies only where the metadata's channel_url holds the URL.
func TestResolveMetaOpsByChannelURLFilter(t *testing.T) {
	filtered, _, err := sharedparsing.ParseFilteredMetaOps([]string{"channel_url:contains:youtube.com/@ChannelA||title:set:cat"})
	if err != nil {
		t.Fatalf("ParseFilteredMetaOps: %v", err)
	}

	for _, tt := range []struct {
		channelURL string
		want       bool
	}{
		{"https://www.youtube.com/@ChannelA", true},
		{"https://www.youtube.com/@ChannelB", false},
	} {
		fd := &FileData{MWebData: &MetadataWebData{}}
		if err := fd.ResolveMetaOps(nil, filtered, map[string]any{"channel_url": tt.channelURL}); err != nil {
			t.Fatalf("ResolveMetaOps: %v", err)
		}
		if got := len(fd.MetaOps.SetFields) == 1; got != tt.want {
			t.Errorf("channel_url %q: op applied = %v, want %v", tt.channelURL, got, tt.want)
		}
	}
}

// TestResolveMetaOpsWithFilters covers the standalone filtered form:
// "title:contains:cat||title:prefix:[CAT VIDEOS] " applies the prefix only when the
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
// "title:contains:cat||prefix:[cat video] " prefixes the filename only when the file's
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

// TestTextOpsKinoCasino converts the new text operations from their string form and
// applies them to a title whose prefix varies per video.
func TestTextOpsKinoCasino(t *testing.T) {
	const title = "Kino Casino 2024-03-01 | Episode name | Part 2"

	parsed, _, err := sharedparsing.ParseMetaOps([]string{
		`title:regex-replace:[[^Kino Casino [^|]*\| ]]:`,
		"title:keep-after:| ",
		"title:keep-before: |",
	})
	if err != nil {
		t.Fatalf("ParseMetaOps: %v", err)
	}
	ops, err := MetaOpsFromShared(parsed)
	if err != nil {
		t.Fatalf("MetaOpsFromShared: %v", err)
	}

	if got := ops.RegexReplaces[0].Regex.ReplaceAllString(title, ops.RegexReplaces[0].Replacement); got != "Episode name | Part 2" {
		t.Errorf("regex-replace = %q", got)
	}
	if c := ops.Cuts[0]; c.Field != "title" || c.Delim != "| " || c.KeepBefore {
		t.Errorf("keep-after = %+v", c)
	}
	if c := ops.Cuts[1]; c.Delim != " |" || !c.KeepBefore {
		t.Errorf("keep-before = %+v", c)
	}

	fparsed, _, err := sharedparsing.ParseFilenameOps([]string{`regex-replace:[[(?i)^kino casino (\d{4})\S* \| (.*)]]:$2 ($1)`, "keep-before: |"})
	if err != nil {
		t.Fatalf("ParseFilenameOps: %v", err)
	}
	fops, err := FilenameOpsFromShared(fparsed)
	if err != nil {
		t.Fatalf("FilenameOpsFromShared: %v", err)
	}
	r := fops.RegexReplaces[0]
	if got := r.Regex.ReplaceAllString(title, r.Replacement); got != "Episode name | Part 2 (2024)" {
		t.Errorf("filename regex-replace = %q", got)
	}
	if !fops.Cuts[0].KeepBefore {
		t.Errorf("keep-before not marked After: %+v", fops.Cuts[0])
	}
}
