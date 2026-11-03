package ffmpeg

import (
	"strings"
	"testing"

	"metarr/internal/abstractions"
	"metarr/internal/domain/consts"
	"metarr/internal/domain/keys"

	"github.com/TubarrApp/gocommon/sharedconsts"
	"github.com/TubarrApp/gocommon/sharedmodels"
)

// TestSetVideoEncoderArgsRefusesBadValues tests a value that can't work stops only when the encoder it can't work on is the one that runs.
func TestSetVideoEncoderArgsRefusesBadValues(t *testing.T) {
	t.Cleanup(func() { abstractions.Set(keys.TranscodeVideoArgsMap, map[string]sharedmodels.EncoderArgs{}) })
	abstractions.Set(keys.TranscodeVideoArgsMap, map[string]sharedmodels.EncoderArgs{
		sharedconsts.VCodecAV1: {Args: []string{"-preset", "p5"}},
	})

	for _, c := range []struct {
		name    string
		b       *ffCommandBuilder
		accel   string
		wantErr bool
	}{
		{"on the software encoder", &ffCommandBuilder{videoCodecSoftware: []string{consts.FFmpegCV0, consts.FFVCodecKeyAV1}}, "", true},
		{"on a GPU encoder, which takes it", &ffCommandBuilder{videoCodecGPU: []string{consts.FFmpegCV0, "av1_nvenc"}}, sharedconsts.AccelTypeCuda, false},
		{"on copied video", &ffCommandBuilder{videoCodecSoftware: []string{consts.FFmpegCV0, consts.FFVCodecKeyCopy}}, "", false},
	} {
		if err := c.b.setVideoEncoderArgs(t.Context(), c.accel, sharedconsts.VCodecAV1); (err != nil) != c.wantErr {
			t.Errorf("%s: got %v, want error %t", c.name, err, c.wantErr)
		}
	}
}

// TestCheckCopiedAudioFilter tests an audio filter stops only when it would stop FFmpeg: on a file's audio being copied.
func TestCheckCopiedAudioFilter(t *testing.T) {
	t.Cleanup(func() { abstractions.Set(keys.ExtraFFmpegArgs, "") })
	copied := []string{consts.FFmpegCA, sharedconsts.ACodecCopy}
	encoded := []string{consts.FFmpegCA, consts.FFACodecKeyAAC}

	for _, c := range []struct {
		name    string
		extra   string
		audio   []string
		current string
		wantErr bool
	}{
		{"filter on copied audio", "-af volume=0.5", copied, sharedconsts.ACodecAAC, true},
		{"long form on copied audio", "-filter:a volume=0.5", copied, sharedconsts.ACodecAAC, true},
		{"filter on encoded audio", "-af volume=0.5", encoded, sharedconsts.ACodecMP3, false},
		{"filter on a file with no audio", "-af volume=0.5", copied, "", false},
		{"filter on audio that couldn't be read", "-af volume=0.5", copied, sharedconsts.CodecCopy, false},
		{"an option FFmpeg only ignores", "-b:a 128k", copied, sharedconsts.ACodecAAC, false},
		{"no extra args", "", copied, sharedconsts.ACodecAAC, false},
	} {
		abstractions.Set(keys.ExtraFFmpegArgs, c.extra)
		b := &ffCommandBuilder{audioCodec: c.audio}
		err := b.checkCopiedAudioFilter(c.current)
		if (err != nil) != c.wantErr {
			t.Errorf("%s: got %v, want error %t", c.name, err, c.wantErr)
		}
		if err != nil && !strings.Contains(err.Error(), "only works on audio being encoded") {
			t.Errorf("%s: error %q doesn't say why", c.name, err)
		}
	}
}
