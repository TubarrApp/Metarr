package ffmpeg

import (
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"metarr/internal/domain/consts"
	"metarr/internal/models"

	"github.com/TubarrApp/gocommon/sharedconsts"
	"github.com/TubarrApp/gocommon/sharedvalidation"
)

// probedCodecs maps each codec to the start of the name FFprobe reports for it.
var probedCodecs = map[string]string{
	sharedconsts.VCodecAV1:    "av1",
	sharedconsts.VCodecH264:   "h264",
	sharedconsts.VCodecHEVC:   "hevc",
	sharedconsts.VCodecMPEG2:  "mpeg2video",
	sharedconsts.VCodecVP8:    "vp8",
	sharedconsts.VCodecVP9:    "vp9",
	sharedconsts.ACodecAAC:    "aac",
	sharedconsts.ACodecAC3:    "ac3",
	sharedconsts.ACodecALAC:   "alac",
	sharedconsts.ACodecDTS:    "dts",
	sharedconsts.ACodecEAC3:   "eac3",
	sharedconsts.ACodecFLAC:   "flac",
	sharedconsts.ACodecMP2:    "mp2",
	sharedconsts.ACodecMP3:    "mp3",
	sharedconsts.ACodecOpus:   "opus",
	sharedconsts.ACodecPCM:    "pcm_",
	sharedconsts.ACodecTrueHD: "truehd",
	sharedconsts.ACodecVorbis: "vorbis",
	sharedconsts.ACodecWAV:    "pcm_",
}

// TestContainerCodecsWithFFmpeg writes every video extension with every codec, using the installed FFmpeg and
// Metarr's own codec arguments, and checks the container tables list exactly what FFmpeg can write and FFprobe read.
func TestContainerCodecsWithFFmpeg(t *testing.T) {
	if os.Getenv("METARR_FFMPEG_CONTAINERS") == "" {
		t.Skip("set METARR_FFMPEG_CONTAINERS=1 to run against the installed FFmpeg")
	}
	encoders := ffmpegAvailableCodecs(t.Context())

	// Make short video-only and audio-only source clips.
	dir := t.TempDir()
	videoSrc := filepath.Join(dir, "video.mkv")
	audioSrc := filepath.Join(dir, "audio.mkv")
	sources := map[string][]string{
		videoSrc: {"-f", "lavfi", "-i", "testsrc=s=128x72:r=10:d=0.5", "-pix_fmt", "yuv420p", "-c:v", "ffv1"},
		audioSrc: {"-f", "lavfi", "-i", "sine=d=0.5:sample_rate=48000", "-ac", "2", "-c:a", "flac"},
	}
	for src, args := range sources {
		args = append([]string{"-v", "error"}, args...)
		if out, err := exec.Command("ffmpeg", append(args, src)...).CombinedOutput(); err != nil {
			t.Fatalf("making source clip %s: %v\n%s", src, err, out)
		}
	}

	for _, ext := range slices.Sorted(maps.Keys(sharedconsts.AllVidExtensions)) {
		// Metarr never writes extensions refused as output.
		if _, err := sharedvalidation.ValidateFFmpegOutputExt(ext); err != nil {
			continue
		}

		// Video codecs.
		for _, codec := range slices.Sorted(maps.Keys(sharedconsts.ValidVideoCodecs)) {
			if codec == sharedconsts.VCodecCopy {
				continue
			}
			t.Run(ext+"/video/"+codec, func(t *testing.T) {
				t.Parallel()
				checkContainerCodec(t, videoSrc, ext, codec, sharedvalidation.VideoCodecFits(ext, codec), func(b *ffCommandBuilder) error {
					err := b.setVideoSoftwareCodec("", codec, encoders)

					// Writing a container doesn't depend on the encoder, so use another AV1 one if Metarr's isn't installed.
					if err != nil && codec == sharedconsts.VCodecAV1 && strings.Contains(encoders, "libaom-av1") {
						b.videoCodecSoftware = []string{consts.FFmpegCV0, "libaom-av1"}
						return nil
					}
					return err
				})
			})
		}

		// Audio codecs.
		for _, codec := range slices.Sorted(maps.Keys(sharedconsts.ValidAudioCodecs)) {
			if codec == sharedconsts.ACodecCopy {
				continue
			}
			t.Run(ext+"/audio/"+codec, func(t *testing.T) {
				t.Parallel()
				checkContainerCodec(t, audioSrc, ext, codec, sharedvalidation.AudioCodecFits(ext, codec), func(b *ffCommandBuilder) error {
					return b.setAudioCodec("", codec, encoders)
				})
			})
		}
	}
}

// checkContainerCodec writes src as an ext file in codec, failing if the tables list the codec for ext but FFmpeg can't
// write it, and logging if FFmpeg can write it but the tables don't list it.
func checkContainerCodec(t *testing.T, src, ext, codec string, listed bool, setCodec func(*ffCommandBuilder) error) {
	t.Helper()
	out := filepath.Join(t.TempDir(), "out"+ext)
	b, err := newFfCommandBuilder(&models.FileData{OriginalVideoPath: src}, out)
	if err != nil {
		t.Fatal(err)
	}
	if err := setCodec(b); err != nil {
		t.Skipf("encoder not installed: %v", err)
	}

	// Build the command as Metarr does, without metadata or a thumbnail.
	b.setDefaultFormatFlagMap(ext)
	args, err := b.buildFinalCommand(b.setFormatFlags(), false)
	if err != nil {
		t.Fatal(err)
	}
	args = append([]string{"-v", "error"}, args...)
	output, runErr := exec.Command("ffmpeg", args...).CombinedOutput()

	// Read back the codec FFmpeg wrote.
	var probed string
	if runErr == nil {
		p, err := exec.Command("ffprobe", "-v", "error", "-show_entries", "stream=codec_name", "-of", "csv=p=0", out).CombinedOutput()
		if err != nil {
			runErr = err
			output = p
		}
		probed = strings.TrimSpace(string(p))
	}
	// MP4-family files label all MPEG-1 audio mp3, so MP2 reads back as mp3 there.
	works := runErr == nil && (strings.HasPrefix(probed, probedCodecs[codec]) || codec == sharedconsts.ACodecMP2 && probed == "mp3")

	switch {
	case listed && !works:
		t.Errorf("listed for %s, but FFmpeg can't write it (read back %q): %v\n%s", ext, probed, runErr, output)
	case !listed && works:
		t.Logf("%s isn't listed for %s, but FFmpeg writes it", codec, ext)
	}
}
