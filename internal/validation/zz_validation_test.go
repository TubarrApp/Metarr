package validation

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"metarr/internal/abstractions"
	"metarr/internal/domain/keys"
	"metarr/internal/domain/logger"

	"github.com/TubarrApp/gocommon/sharedconsts"
	"github.com/TubarrApp/gocommon/sharedmodels"
)

// ******************************** Transformations ***********************************************************************************

// TestMain gives the global logger a console writer, since the zero value has none and
// any warning would dereference nil.
func TestMain(m *testing.M) {
	logger.Pl.Console = io.Discard
	os.Exit(m.Run())
}

// TestMalformedOpIsFatal keeps Metarr failing fast: a typo must stop the run rather
// than quietly applying only the operations that happened to parse.
func TestMalformedOpIsFatal(t *testing.T) {
	err := ValidateAndSetMetaOps([]string{"title:prefix:[CATS] ", "titel:st"})
	if err == nil {
		t.Fatal("a malformed meta operation must abort, not be skipped")
	}
	if !strings.Contains(err.Error(), "titel:st") {
		t.Errorf("error should name the offending entry, got: %v", err)
	}

	if err := ValidateAndSetFilenameOps([]string{"prefix:[CATS] ", "not-an-op:a:b"}); err == nil {
		t.Fatal("a malformed filename operation must abort, not be skipped")
	}
}

// TestDuplicateOpIsTolerated keeps duplicates non-fatal, as they were before, since
// dropping one changes nothing about the result.
func TestDuplicateOpIsTolerated(t *testing.T) {
	if err := ValidateAndSetMetaOps([]string{"title:set:Cats", "title:set:Cats"}); err != nil {
		t.Errorf("duplicate meta operations should be tolerated, got: %v", err)
	}
	if err := ValidateAndSetFilenameOps([]string{"prefix:[CATS] ", "prefix:[CATS] "}); err != nil {
		t.Errorf("duplicate filename operations should be tolerated, got: %v", err)
	}
}

// TestFilteredMetaOpsValidateFilters keeps a bad filter failing at startup rather than
// surfacing per file at match time, which is what happened before filter validation
// was shared.
func TestFilteredMetaOpsValidateFilters(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		wantErr bool
	}{
		{"valid", "title:contains:cat||title:prefix:[CAT VIDEOS] ", false},
		{"valid with ':' and '|' in the filter value", "title:contains:Ep 5 | Show: Finale||title:prefix:[CAT VIDEOS] ", false},
		{"single '|' divider", "title:contains:cat|title:prefix:[CAT VIDEOS] ", true},
		{"unknown filter type", "title:explodes:cat||title:prefix:[CAT] ", true},
		{"non-numeric numeric filter", "duration:more-than:soon||title:prefix:[CAT] ", true},
		{"unknown operation type", "title:contains:cat||title:explodes:x", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateAndSetFilteredMetaOps([]string{tt.in})
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateAndSetFilteredMetaOps(%q) err = %v, wantErr %v", tt.in, err, tt.wantErr)
			}
		})
	}
}

// TestFilteredFilenameOpsValidation keeps bad entries failing at startup, and rejects a
// metadata field on the operation side, since filenames have no fields.
func TestFilteredFilenameOpsValidation(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		wantErr bool
	}{
		{"valid", "title:contains:cat||prefix:[cat video] ", false},
		{"valid date tag", "title:contains:cat||date-tag:prefix:ymd", false},
		{"field on the filename side", "title:contains:cat||title:prefix:[cat video] ", true},
		{"unknown filter type", "title:explodes:cat||prefix:[cat] ", true},
		{"unknown operation type", "title:contains:cat||explodes:x", true},
		{"no divider", "title:contains:cat", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateAndSetFilteredFilenameOps([]string{tt.in})
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateAndSetFilteredFilenameOps(%q) err = %v, wantErr %v", tt.in, err, tt.wantErr)
			}
		})
	}
}

// ******************************** Codec mapping *************************************************************************************

// TestPlainCodecKeptForUnlistedSources tests a plain codec is kept for sources the map doesn't list, and only a plain codec.
func TestPlainCodecKeptForUnlistedSources(t *testing.T) {
	t.Cleanup(func() {
		abstractions.Set(keys.TranscodeVideoCodecMap, map[sharedmodels.FromCodec]string{})
		abstractions.Set(keys.TranscodeAudioCodecMap, map[sharedmodels.FromCodec]string{})
	})
	blank := sharedmodels.NewFromCodec("", sharedconsts.CompressionBlank)

	for _, c := range []struct {
		entries []string
		want    string
		wantSet bool
	}{
		{[]string{"hevc"}, sharedconsts.VCodecHEVC, true},
		{[]string{"copy"}, sharedconsts.VCodecCopy, true},
		{[]string{"h264:hevc"}, "", false},
		{[]string{"none"}, "", false},
	} {
		if err := ValidateAndSetVideoCodec(c.entries); err != nil {
			t.Fatalf("%q: %v", c.entries, err)
		}
		got, ok := abstractions.Get(keys.TranscodeVideoCodecMap).(map[sharedmodels.FromCodec]string)
		if !ok {
			t.Fatalf("%q: got the wrong type", c.entries)
		}
		if value, set := got[blank]; set != c.wantSet || value != c.want {
			t.Errorf("video %q: got %q, %t; want %q, %t", c.entries, value, set, c.want, c.wantSet)
		}
	}

	if err := ValidateAndSetAudioCodec([]string{"aac"}); err != nil {
		t.Fatal(err)
	}
	got, ok := abstractions.Get(keys.TranscodeAudioCodecMap).(map[sharedmodels.FromCodec]string)
	if !ok {
		t.Fatal("audio: got the wrong type")
	}
	if got[blank] != sharedconsts.ACodecAAC {
		t.Errorf("audio: got %q, want aac", got[blank])
	}
}

// ******************************** Common ********************************************************************************************

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

// ******************************** Thumbnails ****************************************************************************************

// TestValidateThumbnail tests --thumbnail takes an http(s) URL or an existing file, and never alongside stripping.
func TestValidateThumbnail(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "cover.png")
	if err := os.WriteFile(file, []byte(sharedconsts.ITypePNG), 0o600); err != nil {
		t.Fatal(err)
	}

	for _, tt := range []struct {
		name  string
		in    string
		strip bool
		ok    bool
	}{
		{"file", file, false, true},
		{"https URL", "https://example.com/cover.jpg", false, true},
		{"missing file", filepath.Join(dir, "nope.png"), false, false},
		{"directory", dir, false, false},
		{"other scheme", "ftp://example.com/cover.jpg", false, false},
		{"with strip", file, true, false},
	} {
		if err := ValidateThumbnail(tt.in, tt.strip); (err == nil) != tt.ok {
			t.Errorf("%s: ValidateThumbnail(%q, %t) = %v, want ok %t", tt.name, tt.in, tt.strip, err, tt.ok)
		}
	}
}
