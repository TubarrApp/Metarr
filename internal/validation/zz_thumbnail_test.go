package validation

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/TubarrApp/gocommon/sharedconsts"
)

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
