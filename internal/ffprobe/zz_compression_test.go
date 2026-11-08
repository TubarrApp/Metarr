package ffprobe

import (
	"io"
	"math"
	"metarr/internal/domain/logger"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/TubarrApp/gocommon/sharedconsts"
)

// TestProbeCodecsMeasuresMKVAudio tests that an MKV's audio bitrate is measured from its packets only when asked, matching an MP4 copy's.
func TestProbeCodecsMeasuresMKVAudio(t *testing.T) {
	if _, err := exec.LookPath(sharedconsts.ProgramFFmpeg); err != nil {
		t.Skip("FFmpeg isn't installed")
	}
	logger.Pl.Console = io.Discard

	// An MKV, which stores no audio bitrate, and an MP4 copy of it, which does.
	dir := t.TempDir()
	mkv := filepath.Join(dir, "v.mkv")
	mp4 := filepath.Join(dir, "v.mp4")
	for _, args := range [][]string{
		{"-f", "lavfi", "-i", "color=black:s=64x64:d=30", "-f", "lavfi", "-i", "anoisesrc=d=30:sample_rate=48000", "-c:v", "mpeg4", "-c:a", "aac", "-b:a", "128k", "-ac", "2", mkv},
		{"-i", mkv, "-c", "copy", mp4},
	} {
		if out, err := exec.Command(sharedconsts.ProgramFFmpeg, append([]string{"-nostdin", "-v", "error", "-y"}, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("ffmpeg %v: %v\n%s", args, err, out)
		}
	}

	// The MP4's stored bitrate and level.
	out, err := exec.Command(sharedconsts.ProgramFFprobe, "-v", "error", "-select_streams", "a:0", "-show_entries", "stream=bit_rate", "-of", "csv=p=0", mp4).Output()
	if err != nil {
		t.Fatal(err)
	}
	stored, err := strconv.ParseFloat(strings.TrimSpace(string(out)), 64)
	if err != nil {
		t.Fatalf("MP4 audio bitrate %q: %v", out, err)
	}
	_, _, storedLevel, _, err := ProbeCodecs(t.Context(), mp4, false)
	if err != nil || storedLevel == sharedconsts.CompressionBlank {
		t.Fatalf("MP4 audio level %q: %v", storedLevel, err)
	}

	// The MKV's packets add up to the same bitrate.
	measured, err := strconv.ParseFloat(probeAudioBitrate(t.Context(), mkv), 64)
	if err != nil {
		t.Fatalf("measuring the MKV's audio: %v", err)
	}
	if math.Abs(measured-stored) > stored/100 {
		t.Errorf("measured %.0f bps, the MP4 stores %.0f", measured, stored)
	}

	// The level is only measured when asked.
	if _, _, level, _, err := ProbeCodecs(t.Context(), mkv, false); err != nil || level != sharedconsts.CompressionBlank {
		t.Errorf("not asked: got level %q (%v), want none", level, err)
	}
	if _, _, level, _, err := ProbeCodecs(t.Context(), mkv, true); err != nil || level != storedLevel {
		t.Errorf("asked: got level %q (%v), want the MP4's %q", level, err, storedLevel)
	}
}

// TestVideoCompressionLevelByCodec tests that the same bitrate gives each video codec its own compression level.
func TestVideoCompressionLevelByCodec(t *testing.T) {
	logger.Pl.Console = io.Discard

	tests := []struct {
		name    string
		codec   string
		bitRate string
		want    sharedconsts.CompressionLevel
	}{
		{name: "h264 at 2 Mbps", codec: "h264", bitRate: "2000000", want: sharedconsts.CompressionHigh},
		{name: "hevc at 2 Mbps", codec: "hevc", bitRate: "2000000", want: sharedconsts.CompressionMedium},
		{name: "vp9 at 1.4 Mbps", codec: "vp9", bitRate: "1400000", want: sharedconsts.CompressionHigh},
		{name: "av1 at 1.4 Mbps", codec: "av1", bitRate: "1400000", want: sharedconsts.CompressionMedium},
		{name: "h264 at 3 Mbps", codec: "h264", bitRate: "3000000", want: sharedconsts.CompressionMedium},
		{name: "av1 at 3 Mbps", codec: "av1", bitRate: "3000000", want: sharedconsts.CompressionLow},
		{name: "h264 at 8 Mbps", codec: "h264", bitRate: "8000000", want: sharedconsts.CompressionLow},
		{name: "mpeg2 under FFprobe's name at 8 Mbps", codec: "mpeg2video", bitRate: "8000000", want: sharedconsts.CompressionMedium},
		{name: "a codec with no scale uses h264's", codec: "mpeg4", bitRate: "2000000", want: sharedconsts.CompressionHigh},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			video := &ffprobeCodecOutputStream{CodecName: tt.codec, Width: 1920, Height: 1080, AvgFrameRate: "30/1", BitRate: tt.bitRate}
			got, err := getVideoCompressionLevel(video, "", "", "test.mp4")
			if err != nil {
				t.Fatalf("getVideoCompressionLevel: %v", err)
			}
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

// TestVideoCompressionLevelLessAudio tests that a video stream without a bitrate gets the file's, less the audio's.
func TestVideoCompressionLevelLessAudio(t *testing.T) {
	logger.Pl.Console = io.Discard

	// 2.6 Mbps at 1080p30 is medium compression for h264, and 2.4 Mbps without its audio is high.
	tests := []struct {
		name         string
		streamRate   string
		audioBitRate string
		want         sharedconsts.CompressionLevel
	}{
		{name: "file bitrate with the audio taken out", audioBitRate: "200000", want: sharedconsts.CompressionHigh},
		{name: "file bitrate when the audio's is unknown", want: sharedconsts.CompressionMedium},
		{name: "stream bitrate stays as it is", streamRate: "2600000", audioBitRate: "200000", want: sharedconsts.CompressionMedium},
		{name: "audio bitrate above the file's is ignored", audioBitRate: "3000000", want: sharedconsts.CompressionMedium},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			video := &ffprobeCodecOutputStream{CodecName: "h264", Width: 1920, Height: 1080, AvgFrameRate: "30/1", BitRate: tt.streamRate}
			got, err := getVideoCompressionLevel(video, "2600000", tt.audioBitRate, "test.mkv")
			if err != nil {
				t.Fatalf("getVideoCompressionLevel: %v", err)
			}
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

// TestAudioCompressionLevelByCodec tests that the same bitrate gives each audio codec its own compression level.
func TestAudioCompressionLevelByCodec(t *testing.T) {
	logger.Pl.Console = io.Discard

	tests := []struct {
		name       string
		codec      string
		bitRate    string
		sampleRate string
		channels   int
		want       sharedconsts.CompressionLevel
	}{
		{name: "mp3 stereo at 112k", codec: "mp3", bitRate: "112000", sampleRate: "48000", channels: 2, want: sharedconsts.CompressionHigh},
		{name: "aac stereo at 112k", codec: "aac", bitRate: "112000", sampleRate: "48000", channels: 2, want: sharedconsts.CompressionMedium},
		{name: "aac stereo at 160k", codec: "aac", bitRate: "160000", sampleRate: "48000", channels: 2, want: sharedconsts.CompressionMedium},
		{name: "opus stereo at 160k", codec: "opus", bitRate: "160000", sampleRate: "48000", channels: 2, want: sharedconsts.CompressionLow},
		{name: "aac stereo at 128k and 44.1 kHz", codec: "aac", bitRate: "128000", sampleRate: "44100", channels: 2, want: sharedconsts.CompressionMedium},
		{name: "mp3 stereo at 300k", codec: "mp3", bitRate: "300000", sampleRate: "48000", channels: 2, want: sharedconsts.CompressionLow},
		{name: "mp2 stereo at 300k", codec: "mp2", bitRate: "300000", sampleRate: "48000", channels: 2, want: sharedconsts.CompressionMedium},
		{name: "dts 5.1 at 768k", codec: "dts", bitRate: "768000", sampleRate: "48000", channels: 6, want: sharedconsts.CompressionMedium},
		{name: "flac stereo at 900k", codec: "flac", bitRate: "900000", sampleRate: "48000", channels: 2, want: sharedconsts.CompressionLow},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			audio := &ffprobeCodecOutputStream{CodecName: tt.codec, BitRate: tt.bitRate, SampleRate: tt.sampleRate, Channels: tt.channels}
			got, err := getAudioCompressionLevel(audio, "test.mp4")
			if err != nil {
				t.Fatalf("getAudioCompressionLevel: %v", err)
			}
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}
