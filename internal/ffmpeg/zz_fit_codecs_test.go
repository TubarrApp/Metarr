package ffmpeg

import (
	"testing"

	"github.com/TubarrApp/gocommon/sharedconsts"
)

// TestFitCodecsFromReadOnlyContainer tests codecs Metarr doesn't know are transcoded out of a .rm file rather than
// copied, while known ones that fit, and unknown ones between writable containers, are still copied.
func TestFitCodecsFromReadOnlyContainer(t *testing.T) {
	for _, c := range []struct {
		name                      string
		inExt, currentV, currentA string
		desiredV, wantV, wantA    string
		wantErr                   bool
	}{
		{name: "RealVideo and Cook", inExt: ".rm", currentV: "rv40", currentA: "cook", wantV: sharedconsts.VCodecH264, wantA: sharedconsts.ACodecAAC},
		{name: "AAC is copied", inExt: ".rm", currentV: "rv20", currentA: "aac", wantV: sharedconsts.VCodecH264},
		{name: "copy refused", inExt: ".rm", currentV: "rv40", currentA: "aac", desiredV: sharedconsts.VCodecCopy, wantErr: true},
		{name: "unknown codec in MKV", inExt: ".mkv", currentV: "prores", currentA: "aac"},
	} {
		v, a, err := fitCodecsToContainer(t.Context(), c.inExt, sharedconsts.ExtMKV, c.currentV, c.desiredV, c.currentA, "")
		switch {
		case c.wantErr && err == nil:
			t.Errorf("%s: got %q, %q, want an error", c.name, v, a)
		case !c.wantErr && (err != nil || v != c.wantV || a != c.wantA):
			t.Errorf("%s: got %q, %q, %v; want %q, %q", c.name, v, a, err, c.wantV, c.wantA)
		}
	}
}
