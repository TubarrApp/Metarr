package metawriters

import (
	"encoding/xml"
	"io"
	"os"
	"strings"
	"testing"

	"metarr/internal/domain/enums"
	"metarr/internal/domain/logger"
	"metarr/internal/models"
)

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
	})
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

	if _, edited := rw.overrideXMLFields(got, map[enums.OverrideMetaType]string{enums.OverrideMetaCredits: "Mr Cat"}); edited {
		t.Error("re-applying the same override should report no edit")
	}
}

// TestOverrideXMLFieldsAddsActor tests that a credits override adds an actor when the NFO has none.
func TestOverrideXMLFieldsAddsActor(t *testing.T) {
	got, _ := (&NFOFileRW{}).overrideXMLFields("<movie>\n</movie>", map[enums.OverrideMetaType]string{enums.OverrideMetaCredits: "Mr Cat"})
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
	}) {
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
