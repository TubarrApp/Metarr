package metawriters

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"metarr/internal/domain/enums"
	"metarr/internal/models"

	"github.com/TubarrApp/gocommon/sharedenums"
)

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
	ops.DeleteDateTags["title"] = models.MetaDeleteDateTag{Loc: sharedenums.DateTagLocAll, Format: sharedenums.DateYyyyMmDd}
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
	if _, edited := rw.xmlPrefix(data, []models.MetaPrefix{{Field: "title", Prefix: "[Cats] "}}); edited {
		t.Error("prefix applied again")
	}
	if _, edited := rw.xmlSuffix(data, []models.MetaSuffix{{Field: "title", Suffix: " (HD)"}}); edited {
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
	ops.DeleteDateTags["title"] = models.MetaDeleteDateTag{Loc: sharedenums.DateTagLocPrefix, Format: sharedenums.DateYyyyMmDd}

	if edited, err := (&NFOFileRW{}).MakeMetaEdits(data, f, &models.FileData{MetaOps: ops}); err != nil || !edited {
		t.Fatalf("MakeMetaEdits edited=%v err=%v", edited, err)
	}
	got, _ := os.ReadFile(f.Name())
	if want := "<title>[A] [B] Cats</title>"; !strings.Contains(string(got), want) {
		t.Errorf("missing %q in:\n%s", want, got)
	}
}
