package validation

import (
	"testing"

	"metarr/internal/abstractions"
	"metarr/internal/domain/keys"

	"github.com/TubarrApp/gocommon/sharedconsts"
	"github.com/TubarrApp/gocommon/sharedmodels"
)

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
