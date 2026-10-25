package textedit

import "testing"

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
