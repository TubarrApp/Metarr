package transformations

import (
	"testing"

	"metarr/internal/domain/enums"
)

// TestFixContractions tests that every contraction is fixed, case is kept, and non-ASCII text stays intact.
func TestFixContractions(t *testing.T) {
	tests := []struct{ in, want string }{
		{"Don t Stop", "Dont Stop"},
		{"don t stop, won t stop", "dont stop, wont stop"},
		{"İstanbul Don t Go", "İstanbul Dont Go"},
	}
	for _, tt := range tests {
		got, _, err := fixContractions(tt.in, tt.in, "video.mp4", enums.RenamingSpaces)
		if err != nil {
			t.Fatalf("fixContractions(%q): %v", tt.in, err)
		}
		if got != tt.want {
			t.Errorf("fixContractions(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}
