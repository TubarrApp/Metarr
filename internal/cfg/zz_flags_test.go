package cfg

import (
	"io"
	"slices"
	"testing"

	"metarr/internal/abstractions"
	"metarr/internal/domain/consts"
	"metarr/internal/domain/keys"
	"metarr/internal/domain/logger"

	"github.com/spf13/viper"
)

// TestMinFreeMemFlagIsRead tests the --min-free-mem value reaches the setting the resource check reads.
func TestMinFreeMemFlagIsRead(t *testing.T) {
	logger.Pl.Console = io.Discard
	if err := rootCmd.PersistentFlags().Parse([]string{"--" + keys.MinFreeMemInput, "1K"}); err != nil {
		t.Fatal(err)
	}

	// Settings checked later may fail with none given, but this one's read before them.
	_ = execute()
	if got := abstractions.GetUint64(keys.MinFreeMem); got != consts.KB {
		t.Errorf("min free memory = %d, want %d", got, consts.KB)
	}
}

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
