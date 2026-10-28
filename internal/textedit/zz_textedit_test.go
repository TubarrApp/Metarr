package textedit

import (
	"io"
	"metarr/internal/dates"
	"metarr/internal/domain/logger"
	"os"
	"testing"

	"github.com/TubarrApp/gocommon/sharedenums"
)

// TestMain gives the global logger a console writer, since the zero value has none and logging would dereference nil.
func TestMain(m *testing.M) {
	logger.Pl.Console = io.Discard
	os.Exit(m.Run())
}

// TestAddPrefixes tests that prefixes read in written order, sit after a date tag, and aren't added twice.
func TestAddPrefixes(t *testing.T) {
	prefixes := []string{"[A] ", "[B] "}
	tests := []struct{ in, want string }{
		{"Title", "[A] [B] Title"},
		{"[A] [B] Title", "[A] [B] Title"},
		{"[B] Title", "[A] [B] Title"},
		{"[2023-11-05] Title", "[2023-11-05] [A] [B] Title"},
		{"[2023-11-05] [A] [B] Title", "[2023-11-05] [A] [B] Title"},
	}
	for _, tt := range tests {
		got, changed := AddPrefixes(tt.in, prefixes)
		if got != tt.want || changed != (tt.in != tt.want) {
			t.Errorf("AddPrefixes(%q) = %q, %v, want %q", tt.in, got, changed, tt.want)
		}
	}
}

// TestAddSuffixes tests that suffixes read in written order, sit before a date tag, and aren't added twice.
func TestAddSuffixes(t *testing.T) {
	suffixes := []string{" (A)", " (B)"}
	tests := []struct{ in, want string }{
		{"Title", "Title (A) (B)"},
		{"Title (A) (B)", "Title (A) (B)"},
		{"Title (B)", "Title (A) (B)"},
		{"Title [2023-11-05]", "Title (A) (B) [2023-11-05]"},
		{"Title (A) (B) [2023-11-05]", "Title (A) (B) [2023-11-05]"},
	}
	for _, tt := range tests {
		got, changed := AddSuffixes(tt.in, suffixes)
		if got != tt.want || changed != (tt.in != tt.want) {
			t.Errorf("AddSuffixes(%q) = %q, %v, want %q", tt.in, got, changed, tt.want)
		}
	}
}

// TestReplacePrefixSuffix tests that only a match at the start or end is replaced.
func TestReplacePrefixSuffix(t *testing.T) {
	if got, ok := ReplacePrefix("[OLD] Title", "[OLD] ", "[NEW] "); !ok || got != "[NEW] Title" {
		t.Errorf("ReplacePrefix = %q, %v", got, ok)
	}
	if got, ok := ReplacePrefix("Title [OLD] ", "[OLD] ", "[NEW] "); ok || got != "Title [OLD] " {
		t.Errorf("ReplacePrefix without a match = %q, %v", got, ok)
	}
	if got, ok := ReplaceSuffix("Title DVD", " DVD", " UHD"); !ok || got != "Title UHD" {
		t.Errorf("ReplaceSuffix = %q, %v", got, ok)
	}
	if got, ok := ReplaceSuffix("DVD Title", " DVD", " UHD"); ok || got != "DVD Title" {
		t.Errorf("ReplaceSuffix without a match = %q, %v", got, ok)
	}
}

// TestCut tests keep-after and keep-before, which drop the delimiter, and a value without the delimiter.
func TestCut(t *testing.T) {
	const title = "Kino Casino 2024-03-01 | Episode name | Part 2"
	if got, ok := Cut(title, "| ", false); !ok || got != "Episode name | Part 2" {
		t.Errorf("keep-after = %q, %v", got, ok)
	}
	if got, ok := Cut(title, " |", true); !ok || got != "Kino Casino 2024-03-01" {
		t.Errorf("keep-before = %q, %v", got, ok)
	}
	if got, ok := Cut("No delimiter", "| ", false); ok || got != "No delimiter" {
		t.Errorf("without delimiter = %q, %v", got, ok)
	}
}

// TestStripDateTagsFormats tests a date tag in every format is recognised at either end and anywhere.
func TestStripDateTagsFormats(t *testing.T) {
	for _, tag := range []string{"[2024-05-01]", "[24-05-01]", "[01-05-2024]", "[05-01-24]", "[05-01]"} {
		if _, got := dates.StripDateTags(tag+" Title", sharedenums.DateTagLocPrefix); got != "Title" {
			t.Errorf("prefix %s: got %q", tag, got)
		}
		if _, got := dates.StripDateTags("Title "+tag, sharedenums.DateTagLocSuffix); got != "Title" {
			t.Errorf("suffix %s: got %q", tag, got)
		}
		if _, got := dates.StripDateTags("A "+tag+" B", sharedenums.DateTagLocAll); got != "A B" {
			t.Errorf("all %s: got %q", tag, got)
		}
	}
	if _, got := dates.StripDateTags("[Cats] Title [1]", sharedenums.DateTagLocAll); got != "[Cats] Title [1]" {
		t.Errorf("non-date brackets were stripped: %q", got)
	}
}

// TestAddDateTag tests that a date tag replaces one already at its location and isn't added twice.
func TestAddDateTag(t *testing.T) {
	const tag = "[2024-05-01]"
	tests := []struct {
		in   string
		loc  sharedenums.DateTagLocation
		want string
	}{
		{"Title", sharedenums.DateTagLocPrefix, "[2024-05-01] Title"},
		{"[2023-11-05] Title", sharedenums.DateTagLocPrefix, "[2024-05-01] Title"},
		{"[2024-05-01] Title", sharedenums.DateTagLocPrefix, "[2024-05-01] Title"},
		{"Title [2023-11-05]", sharedenums.DateTagLocSuffix, "Title [2024-05-01]"},
		{"Title [2024-05-01]", sharedenums.DateTagLocSuffix, "Title [2024-05-01]"},
		{"Title", sharedenums.DateTagLocAll, "Title"},
	}
	for _, tt := range tests {
		got, changed := dates.AddDateTag(tt.in, tag, tt.loc)
		if got != tt.want || changed != (tt.in != tt.want) {
			t.Errorf("AddDateTag(%q, %v) = %q, %v, want %q", tt.in, tt.loc, got, changed, tt.want)
		}
	}
}
