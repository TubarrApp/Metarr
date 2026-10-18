package validation

import (
	"io"
	"os"
	"strings"
	"testing"

	"metarr/internal/domain/logger"
)

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
		{"valid", "title:contains:cat|title:prefix:[CAT VIDEOS] ", false},
		{"unknown filter type", "title:explodes:cat|title:prefix:[CAT] ", true},
		{"non-numeric numeric filter", "duration:morethan:soon|title:prefix:[CAT] ", true},
		{"unknown operation type", "title:contains:cat|title:explodes:x", true},
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
		{"valid", "title:contains:cat|prefix:[cat video] ", false},
		{"valid date tag", "title:contains:cat|date-tag:prefix:ymd", false},
		{"field on the filename side", "title:contains:cat|title:prefix:[cat video] ", true},
		{"unknown filter type", "title:explodes:cat|prefix:[cat] ", true},
		{"unknown operation type", "title:contains:cat|explodes:x", true},
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
