package dates

import (
	"io"
	"os"
	"testing"

	"metarr/internal/domain/enums"
	"metarr/internal/domain/logger"
	"metarr/internal/models"
)

// TestMain gives the global logger a console writer, since the zero value has none and logging would dereference nil.
func TestMain(m *testing.M) {
	logger.Pl.Console = io.Discard
	os.Exit(m.Run())
}

// TestMakeDateTagFormats tests a date tag is made in every format, including the two without a year.
func TestMakeDateTagFormats(t *testing.T) {
	fd := &models.FileData{MDates: &models.MetadataDates{FormattedDate: "2024-05-01"}}
	for format, want := range map[enums.DateFormat]string{
		enums.DateYyyyMmDd: "[2024-05-01]",
		enums.DateYyMmDd:   "[24-05-01]",
		enums.DateDdMmYyyy: "[01-05-2024]",
		enums.DateMmDdYy:   "[05-01-24]",
		enums.DateMmDd:     "[05-01]",
		enums.DateDdMm:     "[01-05]",
	} {
		if got, err := MakeDateTag(nil, fd, format); err != nil || got != want {
			t.Errorf("MakeDateTag(%v) = %q, %v, want %q", format, got, err, want)
		}
	}
}

// TestStripDateTagsFormats tests a date tag in every format is recognised at either end and anywhere.
func TestStripDateTagsFormats(t *testing.T) {
	for _, tag := range []string{"[2024-05-01]", "[24-05-01]", "[01-05-2024]", "[05-01-24]", "[05-01]"} {
		if _, got := StripDateTags(tag+" Title", enums.DateTagLocPrefix); got != "Title" {
			t.Errorf("prefix %s: got %q", tag, got)
		}
		if _, got := StripDateTags("Title "+tag, enums.DateTagLocSuffix); got != "Title" {
			t.Errorf("suffix %s: got %q", tag, got)
		}
		if _, got := StripDateTags("A "+tag+" B", enums.DateTagLocAll); got != "A B" {
			t.Errorf("all %s: got %q", tag, got)
		}
	}
	if _, got := StripDateTags("[Cats] Title [1]", enums.DateTagLocAll); got != "[Cats] Title [1]" {
		t.Errorf("non-date brackets were stripped: %q", got)
	}
}

// TestAddDateTag tests that a date tag replaces one already at its location and isn't added twice.
func TestAddDateTag(t *testing.T) {
	const tag = "[2024-05-01]"
	tests := []struct {
		in   string
		loc  enums.DateTagLocation
		want string
	}{
		{"Title", enums.DateTagLocPrefix, "[2024-05-01] Title"},
		{"[2023-11-05] Title", enums.DateTagLocPrefix, "[2024-05-01] Title"},
		{"[2024-05-01] Title", enums.DateTagLocPrefix, "[2024-05-01] Title"},
		{"Title [2023-11-05]", enums.DateTagLocSuffix, "Title [2024-05-01]"},
		{"Title [2024-05-01]", enums.DateTagLocSuffix, "Title [2024-05-01]"},
		{"Title", enums.DateTagLocAll, "Title"},
	}
	for _, tt := range tests {
		got, changed := AddDateTag(tt.in, tag, tt.loc)
		if got != tt.want || changed != (tt.in != tt.want) {
			t.Errorf("AddDateTag(%q, %v) = %q, %v, want %q", tt.in, tt.loc, got, changed, tt.want)
		}
	}
}
