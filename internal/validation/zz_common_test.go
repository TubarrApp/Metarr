package validation

import (
	"bytes"
	"io"
	"slices"
	"strings"
	"testing"

	"metarr/internal/abstractions"
	"metarr/internal/domain/keys"
	"metarr/internal/domain/logger"
)

// TestPurgeMetafilesRefusesInvalid tests that a blank purge setting purges nothing and an invalid one is refused up
// front, rather than failing every video at the end.
func TestPurgeMetafilesRefusesInvalid(t *testing.T) {
	for _, v := range []string{"", "  ", "all", "json", ".JSON", "nfo"} {
		if err := ValidateAndSetPurgeMetafiles(v); err != nil {
			t.Errorf("ValidateAndSetPurgeMetafiles(%q): %v", v, err)
		}
	}
	for _, v := range []string{"-1", "%", "jsn", "all json"} {
		if err := ValidateAndSetPurgeMetafiles(v); err == nil {
			t.Errorf("ValidateAndSetPurgeMetafiles(%q) accepted it", v)
		}
	}
}

// TestOutputFiletypeRefusesInvalid tests that a blank output extension keeps containers and an invalid one is refused
// up front, rather than failing during processing.
func TestOutputFiletypeRefusesInvalid(t *testing.T) {
	for _, v := range []string{"", "  ", "mp4", ".MKV"} {
		if err := ValidateAndSetOutputFiletype(v); err != nil {
			t.Errorf("ValidateAndSetOutputFiletype(%q): %v", v, err)
		}
	}
	for _, v := range []string{"{{year}}", "xyz", "%"} {
		if err := ValidateAndSetOutputFiletype(v); err == nil {
			t.Errorf("ValidateAndSetOutputFiletype(%q) accepted it", v)
		}
	}
}

// TestInputFiletypes tests any known video extension is accepted, unknown entries are left out, and none given means all.
func TestInputFiletypes(t *testing.T) {
	for _, tc := range []struct {
		name         string
		video, meta  []string
		wantV, wantM []string
	}{
		{"none given", nil, nil, []string{"all"}, []string{"all"}},
		{"all", []string{"all"}, []string{"all"}, []string{"all"}, []string{"all"}},
		{"normalized", []string{"mp4", ".MKV", " avi "}, []string{"json", ".NFO"}, []string{".mp4", ".mkv", ".avi"}, []string{".json", ".nfo"}},
		{"unknown entries left out", []string{"mp4", "bogus"}, []string{"json", "bogus"}, []string{".mp4"}, []string{".json"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := ValidateAndSetInputFiletypes(tc.video, tc.meta); err != nil {
				t.Fatal(err)
			}
			if got := abstractions.GetStringSlice(keys.InputVExts); !slices.Equal(got, tc.wantV) {
				t.Errorf("video extensions = %v, want %v", got, tc.wantV)
			}
			if got := abstractions.GetStringSlice(keys.InputMExts); !slices.Equal(got, tc.wantM) {
				t.Errorf("metadata extensions = %v, want %v", got, tc.wantM)
			}
		})
	}

	// Entries with nothing valid are an error, rather than filtering nothing.
	if err := ValidateAndSetInputFiletypes([]string{"bogus"}, nil); err == nil {
		t.Error("no error for video extensions with none valid")
	}
	if err := ValidateAndSetInputFiletypes(nil, []string{"bogus"}); err == nil {
		t.Error("no error for metadata extensions with none valid")
	}
}

// TestBatchPairNamesBadEntry tests the warning for a batch pair with no separator says which entry it is.
func TestBatchPairNamesBadEntry(t *testing.T) {
	var console bytes.Buffer
	logger.Pl.Console = &console
	t.Cleanup(func() { logger.Pl.Console = io.Discard })

	if err := ValidateAndSetBatchPairs([]string{"nocolon"}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(console.String(), `"nocolon"`) {
		t.Errorf("warning doesn't name the entry: %q", console.String())
	}
}
