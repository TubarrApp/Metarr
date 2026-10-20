package validation

import "testing"

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
