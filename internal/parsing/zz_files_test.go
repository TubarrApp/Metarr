package parsing

import (
	"testing"

	"github.com/TubarrApp/gocommon/sharedconsts"
)

// TestOutputExt tests a video keeps its own extension, unless FFmpeg can't write it, when none is set.
func TestOutputExt(t *testing.T) {
	for _, c := range []struct {
		path, want string
		replaced   bool
	}{
		{"/videos/a.mp4", ".mp4", false},
		{"/videos/a.rm", sharedconsts.ExtMKV, true},
		{"/videos/a.RMVB", sharedconsts.ExtMKV, true},
		{"/videos/a.unknown", ".unknown", false},
	} {
		if got, replaced := OutputExt(c.path); got != c.want || replaced != c.replaced {
			t.Errorf("OutputExt(%q) = %q, %v; want %q, %v", c.path, got, replaced, c.want, c.replaced)
		}
	}
}

// TestReadOnlyExt tests only the video extensions FFmpeg can't write are read-only.
func TestReadOnlyExt(t *testing.T) {
	for ext, want := range map[string]bool{".rm": true, ".RMVB": true, ".mkv": false, ".ts": false, ".unknown": false} {
		if got := ReadOnlyExt(ext); got != want {
			t.Errorf("ReadOnlyExt(%q) = %v, want %v", ext, got, want)
		}
	}
}
