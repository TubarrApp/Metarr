package ffmpeg

import (
	"metarr/internal/abstractions"
	"metarr/internal/domain/keys"
	"os/exec"
	"testing"

	"github.com/TubarrApp/gocommon/sharedconsts"
	"github.com/TubarrApp/gocommon/sharedmodels"
)

// TestGetCodecsMeasuresAudioOnlyForLevelLines tests that an MKV's audio level is only worked out when a codec line uses compression levels.
func TestGetCodecsMeasuresAudioOnlyForLevelLines(t *testing.T) {
	if _, err := exec.LookPath(sharedconsts.ProgramFFmpeg); err != nil {
		t.Skip("FFmpeg isn't installed")
	}
	clip := makeClip(t, "v.mkv", "-f", "lavfi", "-i", "color=black:s=64x64:d=1", "-f", "lavfi", "-i", "anoisesrc=d=1:sample_rate=48000",
		"-c:v", "mpeg4", "-c:a", "aac", "-ac", "2")
	t.Cleanup(func() {
		abstractions.Set(keys.TranscodeVideoCodecMap, nil)
		abstractions.Set(keys.TranscodeAudioCodecMap, nil)
	})

	tests := []struct {
		name      string
		videoMap  map[sharedmodels.FromCodec]string
		audioMap  map[sharedmodels.FromCodec]string
		wantLevel bool
	}{
		{
			name:     "no level lines",
			audioMap: map[sharedmodels.FromCodec]string{sharedmodels.NewFromCodec(sharedconsts.ACodecAAC, ""): sharedconsts.ACodecOpus},
		},
		{
			name:      "an audio level line",
			audioMap:  map[sharedmodels.FromCodec]string{sharedmodels.NewFromCodec(sharedconsts.ACodecAAC, sharedconsts.CompressionMedium): sharedconsts.ACodecOpus},
			wantLevel: true,
		},
		{
			name:      "a video level line",
			videoMap:  map[sharedmodels.FromCodec]string{sharedmodels.NewFromCodec(sharedconsts.VCodecH264, sharedconsts.CompressionLow): sharedconsts.VCodecAV1},
			wantLevel: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			abstractions.Set(keys.TranscodeVideoCodecMap, tt.videoMap)
			abstractions.Set(keys.TranscodeAudioCodecMap, tt.audioMap)
			_, audio, err := getCodecs(t.Context(), clip)
			if err != nil {
				t.Fatal(err)
			}
			if got := audio.Compression != sharedconsts.CompressionBlank; got != tt.wantLevel {
				t.Errorf("audio level %q, want a level: %v", audio.Compression, tt.wantLevel)
			}
		})
	}
}
