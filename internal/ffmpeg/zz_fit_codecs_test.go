package ffmpeg

import (
	"testing"

	"github.com/TubarrApp/gocommon/sharedconsts"
)

// TestFitVideoCodecFromReadOnlyContainer tests codecs Metarr doesn't know are transcoded out of a .rm file rather than
// copied, while known ones that fit, and unknown ones between writable containers, are still copied.
func TestFitVideoCodecFromReadOnlyContainer(t *testing.T) {
	for _, c := range []struct {
		name           string
		inExt, current string
		desired, want  string
		wantErr        bool
	}{
		{name: "RealVideo", inExt: ".rm", current: "rv40", want: sharedconsts.VCodecH264},
		{name: "copy refused", inExt: ".rm", current: "rv40", desired: sharedconsts.VCodecCopy, wantErr: true},
		{name: "unknown codec in MKV", inExt: ".mkv", current: "prores"},
	} {
		got, err := fitVideoCodecToContainer(t.Context(), "in"+c.inExt, sharedconsts.ExtMKV, c.current, c.desired)
		switch {
		case c.wantErr && err == nil:
			t.Errorf("%s: got %q, want an error", c.name, got)
		case !c.wantErr && (err != nil || got != c.want):
			t.Errorf("%s: got %q, %v; want %q", c.name, got, err, c.want)
		}
	}
}

// TestFitAudioCodecFromReadOnlyContainer tests codecs Metarr doesn't know are transcoded out of a .rm file rather than
// copied, while known ones that fit, and unknown ones between writable containers, are still copied.
func TestFitAudioCodecFromReadOnlyContainer(t *testing.T) {
	for _, c := range []struct {
		name           string
		inExt, current string
		desired, want  string
		wantErr        bool
	}{
		{name: "Cook", inExt: ".rm", current: "cook", want: sharedconsts.ACodecAAC},
		{name: "AAC is copied", inExt: ".rm", current: "aac", want: ""},
		{name: "unknown codec in MKV", inExt: ".mkv", current: "aac"},
	} {
		got, err := fitAudioCodecToContainer(t.Context(), "in"+c.inExt, sharedconsts.ExtMKV, c.current, c.desired)
		switch {
		case c.wantErr && err == nil:
			t.Errorf("%s: got %q, want an error", c.name, got)
		case !c.wantErr && (err != nil || got != c.want):
			t.Errorf("%s: got %q, %v; want %q", c.name, got, err, c.want)
		}
	}
}
