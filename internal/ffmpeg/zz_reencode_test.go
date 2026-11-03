package ffmpeg

import (
	"context"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"metarr/internal/abstractions"
	"metarr/internal/domain/consts"
	"metarr/internal/domain/keys"
	"metarr/internal/ffprobe"
	"metarr/internal/models"

	"github.com/TubarrApp/gocommon/sharedconsts"
	"github.com/TubarrApp/gocommon/sharedmodels"
)

// TestPixFmtMatches tests which pixel formats count as the one asked, including those a video reads back as under another name.
func TestPixFmtMatches(t *testing.T) {
	for _, c := range []struct {
		name, codec, asked, source string
		want                       bool
	}{
		{"same", sharedconsts.VCodecH264, "yuv420p", "yuv420p", true},
		{"8-bit asked 10-bit", sharedconsts.VCodecH264, "yuv420p10le", "yuv420p", false},
		{"10-bit asked 8-bit", sharedconsts.VCodecHEVC, "yuv420p", "yuv420p10le", false},
		{"chroma differs", sharedconsts.VCodecHEVC, "yuv444p", "yuv420p", false},
		{"nv12 reads as planar", sharedconsts.VCodecH264, "nv12", "yuv420p", true},
		{"nv21 reads as planar", sharedconsts.VCodecH264, "nv21", "yuv420p", true},
		{"nv16 reads as planar", sharedconsts.VCodecH264, "nv16", "yuv422p", true},
		{"nv20le reads as planar", sharedconsts.VCodecH264, "nv20le", "yuv422p10le", true},
		{"p010le reads as planar", sharedconsts.VCodecHEVC, "p010le", "yuv420p10le", true},
		{"full range source", sharedconsts.VCodecH264, "yuv420p", "yuvj420p", true},
		{"full range asked", sharedconsts.VCodecHEVC, "yuvj444p", "yuv444p", true},
		{"H.264 gray reads as 4:2:0", sharedconsts.VCodecH264, "gray", "yuvj420p", true},
		{"H.264 10-bit gray reads as 4:2:0", sharedconsts.VCodecH264, "gray10le", "yuv420p10le", true},
		{"HEVC gray reads as gray", sharedconsts.VCodecHEVC, "gray", "gray", true},
		{"HEVC gray is checked", sharedconsts.VCodecHEVC, "gray", "yuv420p", false},
		{"VP9 alpha is stored apart", sharedconsts.VCodecVP9, "yuva420p", "yuv420p", true},
	} {
		if got := pixFmtMatches(c.codec, c.asked, c.source); got != c.want {
			t.Errorf("%s: got %t, want %t", c.name, got, c.want)
		}
	}
}

// TestReencodeVideo tests which videos already in a codec are encoded again, going by what was named for them.
func TestReencodeVideo(t *testing.T) {
	if !encoderInstalled(t.Context(), consts.FFVCodecKeyH264) {
		t.Skip("FFmpeg can't encode H.264")
	}
	t.Cleanup(func() {
		abstractions.Set(keys.TranscodeVideoCodecInput, []string{})
		abstractions.Set(keys.TranscodeVideoArgsMap, map[string]sharedmodels.EncoderArgs{})
	})
	clip := makeClip(t, "v.mkv", "-f", "lavfi", "-i", "color=black:s=64x64:d=0.2", "-c:v", consts.FFVCodecKeyH264, "-pix_fmt", "yuv420p")

	h264 := sharedmodels.NewFromCodec(sharedconsts.VCodecH264, sharedconsts.CompressionBlank)
	lowH264 := sharedmodels.NewFromCodec(sharedconsts.VCodecH264, sharedconsts.CompressionLow)
	highH264 := sharedmodels.NewFromCodec(sharedconsts.VCodecH264, sharedconsts.CompressionHigh)
	tenBit := sharedmodels.EncoderArgs{PixFmt: "yuv420p10le"}
	eightBit := sharedmodels.EncoderArgs{PixFmt: "yuv420p"}
	qualityOnly := sharedmodels.EncoderArgs{Quality: "30"}

	for _, c := range []struct {
		name    string
		source  sharedmodels.FromCodec
		entries []string
		args    sharedmodels.EncoderArgs
		want    bool
	}{
		// Nothing named for the video.
		{"nothing set", h264, nil, sharedmodels.EncoderArgs{}, false},
		{"pixel format differs", h264, nil, tenBit, true},
		{"pixel format matches", h264, nil, eightBit, false},
		{"args that can't be checked", h264, nil, qualityOnly, false},
		{"pair for another source, pixel format differs", h264, []string{"hevc:av1"}, tenBit, true},
		{"pair for another source, args that can't be checked", h264, []string{"hevc:h264"}, qualityOnly, false},
		{"pair to nothing, pixel format differs", h264, []string{"h264:none"}, tenBit, true},

		// A plain codec.
		{"plain own codec, pixel format differs", h264, []string{"h264"}, tenBit, true},
		{"plain own codec, pixel format matches", h264, []string{"h264"}, eightBit, false},
		{"plain own codec, args that can't be checked", h264, []string{"h264"}, qualityOnly, false},
		{"plain other codec", h264, []string{"hevc"}, tenBit, false},
		{"plain copy", h264, []string{"copy"}, tenBit, false},

		// A pair for the video.
		{"pair to own codec", h264, []string{"h264:h264"}, sharedmodels.EncoderArgs{}, true},
		{"pair to own codec, pixel format matches", h264, []string{"h264:h264"}, eightBit, true},
		{"pair to own codec beside a plain one", h264, []string{"h264", "h264:h264"}, sharedmodels.EncoderArgs{}, true},
		{"pair to own codec beside plain copy", h264, []string{"copy", "h264:h264"}, sharedmodels.EncoderArgs{}, true},
		{"pair to copy", h264, []string{"h264:copy"}, tenBit, false},
		{"pair to copy beside a plain own codec", h264, []string{"h264", "h264:copy"}, tenBit, false},
		{"pair to another codec", h264, []string{"h264:hevc"}, tenBit, false},

		// A pair for one compression level.
		{"level pair to own codec", lowH264, []string{"lc_h264:h264"}, sharedmodels.EncoderArgs{}, true},
		{"level pair for another level", highH264, []string{"lc_h264:h264"}, sharedmodels.EncoderArgs{}, false},
		{"level pair to copy over a pair to own codec", lowH264, []string{"lc_h264:copy", "h264:h264"}, sharedmodels.EncoderArgs{}, false},
		{"pair without a level for any level", highH264, []string{"h264:h264"}, sharedmodels.EncoderArgs{}, true},

		// No video to encode.
		{"no video stream", sharedmodels.NewFromCodec("", ""), []string{"h264:h264"}, tenBit, false},
		{"unreadable video stream", sharedmodels.NewFromCodec(sharedconsts.CodecCopy, ""), []string{"copy:copy"}, tenBit, false},
	} {
		abstractions.Set(keys.TranscodeVideoCodecInput, c.entries)
		abstractions.Set(keys.TranscodeVideoArgsMap, map[string]sharedmodels.EncoderArgs{sharedconsts.VCodecH264: c.args})
		if got := reencodeVideo(t.Context(), clip, c.source); got != c.want {
			t.Errorf("%s: got %t, want %t", c.name, got, c.want)
		}
	}
}

// TestBuildCommandReencode tests a video in the codec asked is only sent to an encoder when it's to be encoded again.
func TestBuildCommandReencode(t *testing.T) {
	if !encoderInstalled(t.Context(), consts.FFVCodecKeyH264) {
		t.Skip("FFmpeg can't encode H.264")
	}
	clip := makeClip(t, "v.mkv", "-f", "lavfi", "-i", "color=black:s=64x64:d=0.2", "-c:v", consts.FFVCodecKeyH264, "-pix_fmt", "yuv420p")

	for _, c := range []struct {
		name     string
		reencode bool
		want     string
	}{
		{"kept", false, consts.FFmpegCV0 + " " + consts.FFVCodecKeyCopy},
		{"encoded again", true, consts.FFmpegCV0 + " " + consts.FFVCodecKeyH264},
	} {
		fd := models.NewFileData()
		fd.OriginalVideoPath = clip
		b, err := newFfCommandBuilder(fd, filepath.Join(t.TempDir(), "out.mkv"))
		if err != nil {
			t.Fatal(err)
		}
		args, err := b.buildCommand(t.Context(), fd, sharedconsts.VCodecH264, "", sharedconsts.ExtMKV, c.reencode)
		if err != nil {
			t.Fatal(err)
		}
		if joined := strings.Join(args, " "); !strings.Contains(joined, c.want) {
			t.Errorf("%s: want %q in: %s", c.name, c.want, joined)
		}
	}
}

// TestPixFmtReadBackWithFFmpeg tests a video encoded in any format an encoder takes counts as in it afterwards, as it would
// otherwise be encoded again on every run. Formats FFmpeg refuses never get a video to check.
func TestPixFmtReadBackWithFFmpeg(t *testing.T) {
	if testing.Short() {
		t.Skip("encodes with every encoder and format, and what they read back as depends on the FFmpeg installed")
	}
	for codec, encoder := range consts.VCodecToFFVCodec {
		if codec == sharedconsts.VCodecCopy || !encoderInstalled(t.Context(), encoder) {
			continue
		}

		// Other tests fill the cache with a set list.
		encoderPixFmtsMu.Lock()
		delete(encoderPixFmtsCache, encoder)
		encoderPixFmtsMu.Unlock()

		for _, pixFmt := range encoderPixFmts(t.Context(), encoder) {
			t.Run(codec+"/"+pixFmt, func(t *testing.T) {
				t.Parallel()
				got, ok := encodeInPixFmt(t, encoder, pixFmt)
				if !ok {
					t.Skipf("FFmpeg refused %s with %s", pixFmt, encoder)
				}
				if !pixFmtMatches(codec, pixFmt, got) {
					t.Errorf("%s asked in %s read back as %s, which would be encoded again on every run", codec, pixFmt, got)
				}
			})
		}
	}
}

// encodeInPixFmt encodes a tiny clip, returning the pixel format it reads back as, or false if FFmpeg refused.
func encodeInPixFmt(t *testing.T, encoder, pixFmt string) (got string, ok bool) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "v.mkv")
	args := []string{"-v", "error", "-y", "-f", "lavfi", "-i", "testsrc2=s=64x64:d=0.2:r=10", "-c:v", encoder, "-pix_fmt", pixFmt}

	// The fastest settings, as the picture doesn't matter.
	switch encoder {
	case consts.FFVCodecKeyAV1:
		args = append(args, "-preset", "12")
	case consts.FFVCodecKeyH264, consts.FFVCodecKeyH265:
		args = append(args, "-preset", "ultrafast")
	case consts.FFVCodecKeyVP8, consts.FFVCodecKeyVP9:
		args = append(args, "-deadline", "realtime", "-cpu-used", "8", "-b:v", "200k")
	}
	if err := exec.CommandContext(t.Context(), sharedconsts.ProgramFFmpeg, append(args, path)...).Run(); err != nil {
		return "", false
	}

	// Read it back.
	_, _, got = ffprobe.ProbeVideoFrame(context.WithoutCancel(t.Context()), path)
	return got, got != ""
}
