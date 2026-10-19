package cfg

import (
	"slices"
	"testing"

	"metarr/internal/domain/keys"

	"github.com/spf13/viper"
)

// TestPathFlagsKeepCommasAndQuotes tests file and directory flags take each path whole, even with commas and quotes,
// and are repeated for several.
func TestPathFlagsKeepCommasAndQuotes(t *testing.T) {
	paths := []string{`/videos/Hello, "World".mp4`, `/videos/Season 1, Part 2`, `/videos/plain.mp4`}
	for _, key := range []string{keys.VideoFiles, keys.MetaFiles, keys.VideoDirs, keys.MetaDirs, keys.BatchPairsInput} {
		var args []string
		for _, p := range paths {
			args = append(args, "--"+key, p)
		}
		if err := rootCmd.PersistentFlags().Parse(args); err != nil {
			t.Fatalf("--%s: %v", key, err)
		}
		if got := viper.GetStringSlice(key); !slices.Equal(got, paths) {
			t.Errorf("--%s read back as %q, want %q", key, got, paths)
		}
	}
}
