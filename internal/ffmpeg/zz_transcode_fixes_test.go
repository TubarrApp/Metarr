package ffmpeg

import (
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"metarr/internal/abstractions"
	"metarr/internal/domain/consts"
	"metarr/internal/domain/keys"
	"metarr/internal/models"

	"github.com/TubarrApp/gocommon/sharedconsts"
	"github.com/TubarrApp/gocommon/sharedmodels"
)

// makeClip writes a short clip with FFmpeg, from these lavfi sources and arguments.
func makeClip(t *testing.T, name string, args ...string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if out, err := exec.Command(sharedconsts.ProgramFFmpeg, append(append([]string{"-v", "error", "-y"}, args...), path)...).CombinedOutput(); err != nil {
		t.Fatalf("making %s: %v\n%s", name, err, out)
	}
	return path
}

// TestFitRefusesUncopyableWithReason tests a refused copy of an unknown codec says the container can't hold it, not that the source is invalid.
func TestFitRefusesUncopyableWithReason(t *testing.T) {
	if _, err := fitVideoCodecToContainer(t.Context(), "in.rm", sharedconsts.ExtMP4, "rv40", sharedconsts.VCodecCopy); err == nil || !strings.Contains(err.Error(), "can't go in a") {
		t.Errorf("video: got %v, want it to say the codec can't go in the container", err)
	}
	if _, err := fitAudioCodecToContainer(t.Context(), "in.rm", sharedconsts.ExtMP4, "cook", sharedconsts.ACodecCopy); err == nil || !strings.Contains(err.Error(), "can't go in a") {
		t.Errorf("audio: got %v, want it to say the codec can't go in the container", err)
	}
}

// TestGetOutputCodecString tests a plain codec reaches sources the map doesn't list, without overriding a listed one or giving a codec to a missing stream.
func TestGetOutputCodecString(t *testing.T) {
	t.Cleanup(func() { abstractions.Set(keys.TranscodeVideoCodecMap, map[sharedmodels.FromCodec]string{}) })
	abstractions.Set(keys.TranscodeVideoCodecMap, map[sharedmodels.FromCodec]string{
		sharedmodels.NewFromCodec(sharedconsts.VCodecH264, ""): sharedconsts.VCodecCopy,
		sharedmodels.NewFromCodec(sharedconsts.VCodecVP9, ""):  "",
		sharedmodels.NewFromCodec("", ""):                      sharedconsts.VCodecHEVC,
	})

	for _, c := range []struct {
		name, source, want string
	}{
		{"listed source", sharedconsts.VCodecH264, sharedconsts.VCodecCopy},
		{"listed as nothing stays nothing", sharedconsts.VCodecVP9, ""},
		{"unlisted source gets the plain codec", "mpeg4", sharedconsts.VCodecHEVC},
		{"no such stream", "", ""},
		{"stream that couldn't be read", sharedconsts.CodecCopy, ""},
	} {
		got := getOutputCodecString(keys.TranscodeVideoCodecMap, sharedmodels.NewFromCodec(c.source, ""), sharedconsts.Video)
		if got != c.want {
			t.Errorf("%s: got %q, want %q", c.name, got, c.want)
		}
	}
}

// TestGetCodecsWithoutVideo tests a file with no video keeps its audio codec, and a thumbnail doesn't count as its video.
func TestGetCodecsWithoutVideo(t *testing.T) {
	audioOnly := makeClip(t, "audio.mp4", "-f", "lavfi", "-i", "sine=d=0.5", "-c:a", "aac")
	withThumbnail := makeClip(t, "thumb.mp4", "-f", "lavfi", "-i", "sine=d=0.5", "-i", makeTestThumbnail(t, "64x64"),
		"-map", "0", "-map", "1", "-c:a", "aac", "-c:v", "copy", "-disposition:v:0", "attached_pic")

	for _, path := range []string{audioOnly, withThumbnail} {
		video, audio, err := getCodecs(path)
		if err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		if video.Codec != "" || audio.Codec != sharedconsts.ACodecAAC {
			t.Errorf("%s: got video %q, audio %q; want no video, aac", path, video.Codec, audio.Codec)
		}
	}
}

// TestEmbedThumbnailIndex tests a thumbnail is marked at the right stream, assuming one video stream only if they can't be read.
func TestEmbedThumbnailIndex(t *testing.T) {
	audioOnly := makeClip(t, "audio.mp4", "-f", "lavfi", "-i", "sine=d=0.5", "-c:a", "aac")
	video := makeClip(t, "video.mp4", "-f", "lavfi", "-i", "color=black:s=64x64:d=0.2", "-c:v", "mpeg4")

	for _, c := range []struct {
		name, input, want string
	}{
		{"no video", audioOnly, "-disposition:v:0"},
		{"a video", video, "-disposition:v:1"},
		{"streams can't be read", filepath.Join(t.TempDir(), "missing.mp4"), "-disposition:v:1"},
	} {
		b := &ffCommandBuilder{inputFile: c.input}
		b.embedThumbnail(t.Context(), "thumb.jpg", sharedconsts.ExtMP4, thumbnailFilename, thumbnailMimetype)
		if !slices.Contains(b.thumbnail, c.want) {
			t.Errorf("%s: got %q, want %s", c.name, b.thumbnail, c.want)
		}
	}
}

// TestSetEvenSize tests only sizes the encoder would refuse are cut, and a video too small to lose a row is refused.
func TestSetEvenSize(t *testing.T) {
	if os.Getenv("METARR_FFMPEG_CONTAINERS") == "" {
		t.Skip("set METARR_FFMPEG_CONTAINERS=1 to run against the installed FFmpeg, as old versions round odd sizes off")
	}
	clip := func(size, pixFmt string) string {
		return makeClip(t, "in.mkv", "-f", "lavfi", "-i", "color=black:s="+size+":d=0.2", "-c:v", "ffv1", "-pix_fmt", pixFmt)
	}
	odd := clip("129x73", "yuv420p")

	for _, c := range []struct {
		name, input, encoder, filter, asked, want string
		wantErr                                   bool
	}{
		{name: "odd on x264", input: odd, encoder: consts.FFVCodecKeyH264, want: consts.EvenSizeFilter},
		{name: "odd on x265", input: odd, encoder: consts.FFVCodecKeyH265, want: consts.EvenSizeFilter},
		{name: "only the width odd", input: clip("129x72", "yuv420p"), encoder: consts.FFVCodecKeyH264, want: consts.EvenSizeFilter},
		{name: "after the user's filter", input: odd, encoder: consts.FFVCodecKeyH264, filter: "hflip", want: "hflip," + consts.EvenSizeFilter},
		{name: "even", input: clip("128x72", "yuv420p"), encoder: consts.FFVCodecKeyH264},
		{name: "other encoder", input: odd, encoder: consts.FFVCodecKeyVP9},
		{name: "4:4:4 source", input: clip("129x73", "yuv444p"), encoder: consts.FFVCodecKeyH264},
		{name: "4:4:4 asked", input: odd, encoder: consts.FFVCodecKeyH264, asked: "yuv444p10le"},
		{name: "4:2:0 asked of a 4:4:4 source", input: clip("129x73", "yuv444p"), encoder: consts.FFVCodecKeyH264, asked: "yuv420p", want: consts.EvenSizeFilter},
		{name: "too small to cut", input: clip("17x9", "yuv420p"), encoder: consts.FFVCodecKeyH264, wantErr: true},
		{name: "unreadable", input: filepath.Join(t.TempDir(), "missing.mkv"), encoder: consts.FFVCodecKeyH264},
	} {
		b := &ffCommandBuilder{inputFile: c.input, videoCodecSoftware: []string{consts.FFmpegCV0, c.encoder}, videoFilter: c.filter}
		want := c.want
		if want == "" {
			want = c.filter
		}
		err := b.setEvenSize(t.Context(), c.asked)
		if (err != nil) != c.wantErr || b.videoFilter != want {
			t.Errorf("%s: got filter %q, error %v; want filter %q, error %t", c.name, b.videoFilter, err, want, c.wantErr)
		}
	}
}

// TestSetAudioCodecArgs tests TrueHD copies can go in MP4, and Opus gets the filter that lets it take surround layouts.
func TestSetAudioCodecArgs(t *testing.T) {
	encoders := " " + consts.FFACodecKeyOpus + " " + sharedconsts.ACodecAAC + " "
	strict := consts.FFmpegStrictExperimental[0]

	for _, c := range []struct {
		name, current, desired string
		wantArg                string
		notWant                string
	}{
		{name: "TrueHD copied", current: sharedconsts.ACodecTrueHD, desired: sharedconsts.ACodecCopy, wantArg: strict},
		{name: "TrueHD kept", current: sharedconsts.ACodecTrueHD, desired: "", wantArg: strict},
		{name: "AAC copied", current: sharedconsts.ACodecAAC, desired: sharedconsts.ACodecCopy, notWant: strict},
		{name: "Opus encoded", current: sharedconsts.ACodecAAC, desired: sharedconsts.ACodecOpus, wantArg: consts.OpusLayoutFilter},
		{name: "Opus copied", current: sharedconsts.ACodecOpus, desired: sharedconsts.ACodecCopy, notWant: consts.OpusLayoutFilter},
		{name: "AAC encoded", current: sharedconsts.ACodecOpus, desired: sharedconsts.ACodecAAC, notWant: consts.OpusLayoutFilter},
	} {
		b := &ffCommandBuilder{}
		if err := b.setAudioCodec(c.current, c.desired, encoders); err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		if c.wantArg != "" && !slices.Contains(b.audioCodec, c.wantArg) {
			t.Errorf("%s: got %q, want %q in it", c.name, b.audioCodec, c.wantArg)
		}
		if c.notWant != "" && slices.Contains(b.audioCodec, c.notWant) {
			t.Errorf("%s: got %q, want no %q", c.name, b.audioCodec, c.notWant)
		}
	}
}

// TestFFmpegFailure tests the reason given is FFmpeg's first real error, not its noise or the errors following from it.
func TestFFmpegFailure(t *testing.T) {
	for _, c := range []struct {
		name, stderr, want string
	}{
		{
			"encoder error, then what follows from it",
			"[info] Stream mapping:\n[libx264 @ 0x64698b450300] [error] width not divisible by 2 (129x73)\n[vost#0:0/libx264 @ 0x6469] [enc:libx264 @ 0x6469] [error] Error while opening encoder\n",
			"[libx264] width not divisible by 2 (129x73)",
		},
		{"x265's own log", "x265 [warning]: Too few rows\nx265 [error]: Picture width must be an integer multiple\n", "x265 [error]: Picture width must be an integer multiple"},
		{"SVT's own log", "Svt[warn]: Failed to set thread priority\nSvt[error]: Target Bitrate only supported\n", "Svt[error]: Target Bitrate only supported"},
		{"no component", "[error] Unrecognized option 'vsync'.\n", "Unrecognized option 'vsync'."},
		{"fatal", "[fatal] Conversion failed!\n", "Conversion failed!"},
		{
			"hardware -hwaccel auto moved on from",
			"[CUDA @ 0x1] [error] Cannot load libcuda.so.1\n[AVHWDeviceContext @ 0x2] [error] Could not dynamically load CUDA\n[error] Device creation failed: -542398533.\n[webm @ 0x3] [error] Only VP8 or VP9\n",
			"[webm] Only VP8 or VP9",
		},
		{"no errors", "[info] Press [q] to stop\n", ""},
		{"nothing", "", ""},
	} {
		if got := ffmpegFailure(c.stderr); got != c.want {
			t.Errorf("%s: got %q, want %q", c.name, got, c.want)
		}
	}
}

// TestGeneratePTSBeforeInput tests timestamps are generated for the video only, which takes the option coming before its -i.
func TestGeneratePTSBeforeInput(t *testing.T) {
	b := &ffCommandBuilder{builder: &strings.Builder{}, inputFile: "in.mpg", outputFile: "out.mkv", thumbnailInput: []string{"-i", "thumb.jpg"}}
	args, err := b.buildFinalCommand(nil, false)
	if err != nil {
		t.Fatal(err)
	}
	if at, in := slices.Index(args, consts.FFmpegGeneratePTS[1]), slices.Index(args, "in.mpg"); at < 0 || at > in {
		t.Errorf("got %q, want %q before the video's -i", args, consts.FFmpegGeneratePTS)
	}
	if count := strings.Count(strings.Join(args, " "), consts.FFmpegGeneratePTS[1]); count != 1 {
		t.Errorf("got %q, want %q once", args, consts.FFmpegGeneratePTS)
	}
}

// TestAACIntoAVIArgs tests AAC staying AAC is encoded again only on the route where copying breaks it, and an explicit copy is refused there.
func TestAACIntoAVIArgs(t *testing.T) {
	encoders := " " + consts.FFACodecKeyAAC + " " + consts.FFACodecKeyMP3 + " "
	for _, c := range []struct {
		name, input, output, desired, want string
	}{
		{"AVI to AVI, nothing asked", "in.avi", "tmp_in.avi.avi", "", consts.FFACodecKeyAAC},
		{"AVI to AVI, AAC asked", "in.avi", "tmp_in.avi.avi", sharedconsts.ACodecAAC, consts.FFACodecKeyAAC},
		{"ASF to AVI", "in.asf", "tmp_in.asf.avi", "", consts.FFACodecKeyAAC},
		{"WMV to AVI", "IN.WMV", "tmp_in.WMV.AVI", "", consts.FFACodecKeyAAC},
		{"another codec asked", "in.avi", "tmp_in.avi.avi", sharedconsts.ACodecMP3, sharedconsts.ACodecMP3},
		{"MP4 to AVI copies fine", "in.mp4", "tmp_in.mp4.avi", "", sharedconsts.ACodecCopy},
		{"AVI to MKV copies fine", "in.avi", "tmp_in.avi.mkv", "", sharedconsts.ACodecCopy},
		{"ASF to ASF copies fine", "in.asf", "tmp_in.asf.asf", "", sharedconsts.ACodecCopy},
	} {
		b := &ffCommandBuilder{inputFile: c.input, outputFile: c.output}
		if err := b.setAudioCodec(sharedconsts.ACodecAAC, c.desired, encoders); err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		if len(b.audioCodec) < 2 || b.audioCodec[1] != c.want {
			t.Errorf("%s: got %q, want %s", c.name, b.audioCodec, c.want)
		}
	}

	// A copy asked for can't be honoured on the route, and other audio codecs aren't affected.
	if _, err := fitAudioCodecToContainer(t.Context(), "in.avi", sharedconsts.ExtAVI, sharedconsts.ACodecAAC, sharedconsts.ACodecCopy); err == nil || !strings.Contains(err.Error(), "can't go in a") {
		t.Errorf("copying AAC from AVI to AVI: got %v, want it refused", err)
	}
	if got, err := fitAudioCodecToContainer(t.Context(), "in.mp4", sharedconsts.ExtAVI, sharedconsts.ACodecAAC, sharedconsts.ACodecCopy); err != nil || got != sharedconsts.ACodecCopy {
		t.Errorf("copying AAC from MP4 to AVI: got %q, %v; want it allowed", got, err)
	}
	if got, err := fitAudioCodecToContainer(t.Context(), "in.avi", sharedconsts.ExtAVI, sharedconsts.ACodecMP3, sharedconsts.ACodecCopy); err != nil || got != sharedconsts.ACodecCopy {
		t.Errorf("copying MP3 from AVI to AVI: got %q, %v; want it allowed", got, err)
	}
}

// lastAudioTime returns when a file's first audio stream ends, going by its last packet.
func lastAudioTime(t *testing.T, path string) float64 {
	t.Helper()
	out, err := exec.Command(sharedconsts.ProgramFFprobe, "-v", "error", "-select_streams", "a:0", "-show_entries", "packet=pts_time", "-of", "csv=p=0", path).Output()
	if err != nil {
		t.Fatalf("probing %s: %v", path, err)
	}
	last := 0.0
	for line := range strings.Lines(string(out)) {
		if when, err := strconv.ParseFloat(strings.TrimRight(strings.TrimSpace(line), ","), 64); err == nil {
			last = when
		}
	}
	return last
}

// TestAACIntoAVIKeepsItsTimingWithFFmpeg tests AAC from AVI or ASF ends when it should in a new AVI, as AAC, stereo or surround.
func TestAACIntoAVIKeepsItsTimingWithFFmpeg(t *testing.T) {
	if os.Getenv("METARR_FFMPEG_CONTAINERS") == "" {
		t.Skip("set METARR_FFMPEG_CONTAINERS=1 to run against the installed FFmpeg")
	}
	for _, ext := range []string{sharedconsts.ExtAVI, sharedconsts.ExtASF, sharedconsts.ExtWMV} {
		for _, layout := range []string{"stereo", "5.1"} {
			source := makeClip(t, "in"+ext, "-f", "lavfi", "-i", "color=black:s=128x72:r=25:d=4", "-f", "lavfi", "-i", "sine=d=4:r=48000",
				"-c:v", "mpeg4", "-c:a", "aac", "-af", "aformat=channel_layouts="+layout)
			want := lastAudioTime(t, source)
			out := executeTestVideo(t, models.NewFileData(), source, sharedconsts.ExtAVI)
			if got := lastAudioTime(t, out); math.Abs(got-want) > 0.3 {
				t.Errorf("%s %s to .avi: audio ends at %.2fs, want about %.2fs", ext, layout, got, want)
			}
			if got := testProbe(t, out, "stream=codec_name"); !slices.Contains(got, sharedconsts.ACodecAAC) {
				t.Errorf("%s %s to .avi: got streams %v, want AAC", ext, layout, got)
			}
		}
	}
}

// TestH264FromASFKeepsVideoWithFFmpeg tests H.264 copied out of ASF into an MPEG file stays video, as its headers aren't in the stream.
func TestH264FromASFKeepsVideoWithFFmpeg(t *testing.T) {
	if os.Getenv("METARR_FFMPEG_CONTAINERS") == "" {
		t.Skip("set METARR_FFMPEG_CONTAINERS=1 to run against the installed FFmpeg")
	}
	for _, ext := range []string{sharedconsts.ExtASF, sharedconsts.ExtWMV} {
		source := makeClip(t, "in"+ext, "-f", "lavfi", "-i", "color=black:s=128x72:r=25:d=2", "-f", "lavfi", "-i", "sine=d=2:r=48000",
			"-c:v", "libx264", "-pix_fmt", "yuv420p", "-c:a", "aac")
		for _, out := range []string{sharedconsts.ExtVOB, sharedconsts.ExtMPEG} {
			got := testProbe(t, executeTestVideo(t, models.NewFileData(), source, out), "stream=codec_name")
			if !slices.Contains(got, sharedconsts.VCodecH264) {
				t.Errorf("%s to %s: got streams %v, want H.264", ext, out, got)
			}
		}
	}
}

// TestH264FromOtherContainersStaysReadableWithFFmpeg tests H.264 copied into an MPEG file can be opened, whatever its pixel format.
func TestH264FromOtherContainersStaysReadableWithFFmpeg(t *testing.T) {
	if os.Getenv("METARR_FFMPEG_CONTAINERS") == "" {
		t.Skip("set METARR_FFMPEG_CONTAINERS=1 to run against the installed FFmpeg")
	}
	for _, ext := range []string{sharedconsts.ExtTS, sharedconsts.ExtMP4, sharedconsts.ExtASF} {
		for _, pixFmt := range []string{"yuv420p", "yuv444p", "yuv420p10le"} {
			source := makeClip(t, "in"+ext, "-f", "lavfi", "-i", "color=black:s=128x72:r=25:d=2", "-f", "lavfi", "-i", "sine=d=2:r=48000",
				"-c:v", "libx264", "-pix_fmt", pixFmt, "-c:a", "aac")
			got := testProbe(t, executeTestVideo(t, models.NewFileData(), source, sharedconsts.ExtMPG), "stream=codec_name")
			if !slices.Contains(got, sharedconsts.VCodecH264) {
				t.Errorf("%s %s to .mpg: got streams %v, want H.264", ext, pixFmt, got)
			}
		}
	}
}

// TestCopySurvivesWithFFmpeg tests a trial copy of codecs the tables don't list against the installed FFmpeg, as what survives depends on it.
func TestCopySurvivesWithFFmpeg(t *testing.T) {
	if os.Getenv("METARR_FFMPEG_CONTAINERS") == "" {
		t.Skip("set METARR_FFMPEG_CONTAINERS=1 to run against the installed FFmpeg")
	}
	video := func(name, encoder string) string {
		return makeClip(t, name, "-f", "lavfi", "-i", "color=black:s=128x72:r=25:d=2", "-f", "lavfi", "-i", "sine=d=2", "-c:v", encoder, "-c:a", "aac")
	}
	audio := func(name, encoder string) string {
		return makeClip(t, name, "-f", "lavfi", "-i", "color=black:s=128x72:r=25:d=2", "-f", "lavfi", "-i", "sine=d=2:r=44100", "-c:v", "libx264", "-pix_fmt", "yuv420p", "-c:a", encoder)
	}
	mpeg4InMP4 := video("v.mp4", "mpeg4")
	mpeg4InAVI := video("v.avi", "mpeg4")
	wmv := video("v.wmv", "wmv2")
	mjpeg := video("mjpeg.avi", "mjpeg")
	swfAudio := audio("a.flv", "adpcm_swf")
	wmaAudio := audio("a.wmv", "wmav2")

	for _, c := range []struct {
		name, input, stream, outExt string
		want                        bool
	}{
		{"own container", mpeg4InMP4, "V", sharedconsts.ExtMP4, true},
		{"mpeg4 from MP4 can't go in TS", mpeg4InMP4, "V", sharedconsts.ExtTS, false},
		{"mpeg4 from AVI can go in TS", mpeg4InAVI, "V", sharedconsts.ExtTS, true},
		{"wmv2 leaves no video in TS", wmv, "V", sharedconsts.ExtTS, false},
		{"wmv2 can't go in MP4", wmv, "V", sharedconsts.ExtMP4, false},
		{"wmv2 can go in MKV", wmv, "V", sharedconsts.ExtMKV, true},
		{"mjpeg can go in MP4", mjpeg, "V", sharedconsts.ExtMP4, true},
		{"mjpeg leaves no video in TS", mjpeg, "V", sharedconsts.ExtTS, false},
		{"adpcm_swf leaves no audio in TS", swfAudio, "a", sharedconsts.ExtTS, false},
		{"adpcm_swf keeps a frame in MOV, which isn't enough", swfAudio, "a", sharedconsts.ExtMOV, false},
		{"wmav2 is unreadable in MOV", wmaAudio, "a", sharedconsts.ExtMOV, false},
		{"wmav2 can go in MKV", wmaAudio, "a", sharedconsts.ExtMKV, true},
	} {
		if got := copySurvives(t.Context(), c.input, c.stream, c.outExt); got != c.want {
			t.Errorf("%s: got %t, want %t", c.name, got, c.want)
		}
	}

	// What the trial decides: copy, transcode, or refuse.
	if got, err := fitVideoCodecToContainer(t.Context(), mjpeg, sharedconsts.ExtMP4, "mjpeg", ""); err != nil || got != "" {
		t.Errorf("mjpeg to MP4: got %q, %v; want it copied", got, err)
	}
	if got, err := fitVideoCodecToContainer(t.Context(), mpeg4InMP4, sharedconsts.ExtTS, "mpeg4", ""); err != nil || got != sharedconsts.VCodecH264 {
		t.Errorf("mpeg4 from MP4 to TS: got %q, %v; want H.264", got, err)
	}
	if _, err := fitVideoCodecToContainer(t.Context(), mpeg4InMP4, sharedconsts.ExtTS, "mpeg4", sharedconsts.VCodecCopy); err == nil {
		t.Error("copying mpeg4 from MP4 to TS: got no error")
	}
	if got, err := fitAudioCodecToContainer(t.Context(), swfAudio, sharedconsts.ExtTS, "adpcm_swf", ""); err != nil || got != sharedconsts.ACodecAAC {
		t.Errorf("adpcm_swf to TS: got %q, %v; want AAC", got, err)
	}
}
