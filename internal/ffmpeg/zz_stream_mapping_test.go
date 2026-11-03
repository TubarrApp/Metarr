package ffmpeg

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"metarr/internal/abstractions"
	"metarr/internal/domain/consts"
	"metarr/internal/domain/keys"
	"metarr/internal/models"

	"github.com/TubarrApp/gocommon/sharedconsts"
	"github.com/TubarrApp/gocommon/sharedmodels"
)

// TestSetStreamMapping tests which streams are kept for each pair of containers.
func TestSetStreamMapping(t *testing.T) {
	for _, tc := range []struct {
		name, in, out  string
		keepThumbnails bool
		want           string
	}{
		{"mkv to mkv", "v.mkv", sharedconsts.ExtMKV, true, "-map 0:v? -map 0:a? -map 0:s? -map 0:t? -c copy"},
		{"mp4 to mp4", "v.mp4", sharedconsts.ExtMP4, true, "-map 0:v? -map 0:a? -map 0:s? -c copy"},
		{"extension case differs", "v.MP4", sharedconsts.ExtMP4, true, "-map 0:v? -map 0:a? -map 0:s? -c copy"},
		{"thumbnail replaced", "v.mp4", sharedconsts.ExtMP4, false, "-map 0:V? -map 0:a? -map 0:s? -c copy"},
		{"mp4 to mkv", "v.mp4", sharedconsts.ExtMKV, true, "-map 0:V? -map 0:a? -map 0:s? -map 0:t? -c copy"},
		{"unreadable file to avi", "missing.mkv", sharedconsts.ExtAVI, true, "-map 0:V? -map 0:a? -c copy"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			b := &ffCommandBuilder{inputFile: tc.in}
			b.setStreamMapping(t.Context(), tc.out, tc.keepThumbnails)
			if got := strings.Join(b.streamMapping, " "); got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

// TestSubtitleEncoderFor tests subtitles are converted only where the output container can't take them as they are.
func TestSubtitleEncoderFor(t *testing.T) {
	for _, tc := range []struct{ in, out, want string }{
		{sharedconsts.ExtMKV, sharedconsts.ExtMKV, ""},
		{sharedconsts.ExtMP4, sharedconsts.ExtMP4, ""},
		{sharedconsts.ExtMKV, sharedconsts.ExtMP4, subtitleEncoderMovText},
		{sharedconsts.ExtWEBM, sharedconsts.ExtMOV, subtitleEncoderMovText},
		{sharedconsts.ExtMKV, sharedconsts.ExtWEBM, subtitleEncoderWebVTT},
		{sharedconsts.ExtMP4, sharedconsts.ExtMKV, subtitleEncoderSRT},
		{sharedconsts.ExtMOV, sharedconsts.ExtMKV, subtitleEncoderSRT},
		{sharedconsts.ExtWEBM, sharedconsts.ExtMKV, ""},
		{sharedconsts.ExtMKV, sharedconsts.ExtAVI, ""},
	} {
		if got := subtitleEncoderFor(tc.in, tc.out); got != tc.want {
			t.Errorf("subtitleEncoderFor(%q, %q) = %q, want %q", tc.in, tc.out, got, tc.want)
		}
	}
}

// TestDefaultFormatFlagsKeepPreset tests a subtitle encoder goes on a copy of the shared preset, not the preset itself.
func TestDefaultFormatFlagsKeepPreset(t *testing.T) {
	b := &ffCommandBuilder{inputFile: "v.mkv"}
	b.setDefaultFormatFlagMap(sharedconsts.ExtMP4)
	if got := b.formatFlagsMap[consts.FFmpegCS]; got != subtitleEncoderMovText {
		t.Errorf("subtitle codec = %q, want %q", got, subtitleEncoderMovText)
	}
	if got := copyPreset[consts.FFmpegCS]; got != sharedconsts.VCodecCopy {
		t.Errorf("preset's subtitle codec changed to %q", got)
	}
}

// TestTempOutputName tests a long base name is hashed to fit the filesystem's limit, the same way each time.
func TestTempOutputName(t *testing.T) {
	if got, want := tempOutputName("video", sharedconsts.ExtMP4, sharedconsts.ExtMKV), sharedconsts.TempTag+"video.mp4.mkv"; got != want {
		t.Errorf("short name = %q, want %q", got, want)
	}

	long := strings.Repeat("a", sharedconsts.MaxNameBytes-len(sharedconsts.ExtMP4))
	got := tempOutputName(long, sharedconsts.ExtMP4, sharedconsts.ExtMP4)
	if len(got) > sharedconsts.MaxNameBytes {
		t.Errorf("long name is %d bytes, over the limit of %d", len(got), sharedconsts.MaxNameBytes)
	}
	if !strings.HasPrefix(got, sharedconsts.TempTag) || !strings.HasSuffix(got, ".mp4.mp4") {
		t.Errorf("long name %q lost its tag or extensions", got)
	}
	if again := tempOutputName(long, sharedconsts.ExtMP4, sharedconsts.ExtMP4); again != got {
		t.Errorf("long name changed from %q to %q", got, again)
	}
	if other := tempOutputName(strings.Repeat("b", len(long)), sharedconsts.ExtMP4, sharedconsts.ExtMP4); other == got {
		t.Errorf("two long names share the temp name %q", got)
	}
}

// TestSkipProcessingExtensionCase tests a video isn't rewritten only because its extension is in capitals, and keeps its path.
func TestSkipProcessingExtensionCase(t *testing.T) {
	const path = "/videos/v.MP4"
	for _, tc := range []struct {
		outExt string
		want   bool
	}{
		{".MP4", true},
		{sharedconsts.ExtMP4, true},
		{sharedconsts.ExtMKV, false},
	} {
		fd := models.NewFileData()
		fd.OriginalVideoPath = path
		fd.VideoDirectory = "/videos"
		fd.MetaAlreadyExists = true
		if got := skipProcessing(fd, sharedmodels.FromCodec{}, sharedmodels.FromCodec{}, "", "", tc.outExt); got != tc.want {
			t.Errorf("skipProcessing to %q = %v, want %v", tc.outExt, got, tc.want)
		}
		if tc.want && fd.PostFFmpegVideoPath != path {
			t.Errorf("skipping to %q gives the path %q, want %q", tc.outExt, fd.PostFFmpegVideoPath, path)
		}
	}
}

// ---- Tests running FFmpeg ----------------------------------------------------------------------------------------

// testVideoInputs returns FFmpeg inputs for a short video (0), two audio tracks (1, 2), two subtitle tracks (3, 4) and a thumbnail (5).
func testVideoInputs(t *testing.T) []string {
	t.Helper()
	if _, err := exec.LookPath(sharedconsts.ProgramFFmpeg); err != nil {
		t.Skip("FFmpeg isn't installed")
	}
	dir := t.TempDir()
	srt := filepath.Join(dir, "subs.srt")
	if err := os.WriteFile(srt, []byte("1\n00:00:00,000 --> 00:00:00,200\nhi\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	thumbnail := filepath.Join(dir, "thumbnail.jpg")
	runTestFFmpeg(t, "-f", "lavfi", "-i", "testsrc=d=0.1:s=32x32", "-frames:v", "1", thumbnail)
	return []string{
		"-f", "lavfi", "-i", "testsrc=s=64x36:r=5:d=0.4",
		"-f", "lavfi", "-i", "sine=d=0.4",
		"-f", "lavfi", "-i", "sine=d=0.4:f=600",
		"-i", srt, "-i", srt, "-i", thumbnail,
	}
}

// runTestFFmpeg runs FFmpeg with args, failing the test if it fails.
func runTestFFmpeg(t *testing.T, args ...string) {
	t.Helper()
	if out, err := exec.Command(sharedconsts.ProgramFFmpeg, append([]string{"-nostdin", "-v", "error", "-y"}, args...)...).CombinedOutput(); err != nil {
		t.Fatalf("ffmpeg %v: %v\n%s", args, err, out)
	}
}

// makeTestThumbnail makes a JPG of a given size at a new path.
func makeTestThumbnail(t *testing.T, size string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "made.jpg")
	runTestFFmpeg(t, "-f", "lavfi", "-i", "testsrc=d=0.1:s="+size, "-frames:v", "1", path)
	return path
}

// makeThumbnailVideo makes a video with a thumbnail at a new path of the given name.
func makeThumbnailVideo(t *testing.T, inputs []string, name string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	runTestFFmpeg(t, append(slices.Clone(inputs), "-map", "0", "-map", "1", "-map", "5",
		"-c:v:0", "mpeg4", "-c:a", "aac", "-c:v:1", "mjpeg", "-disposition:v:1", "attached_pic", path)...)
	return path
}

// testProbe returns FFprobe's answer for a file's streams, one sorted entry per stream that has one.
func testProbe(t *testing.T, path, entries string) []string {
	t.Helper()
	out, err := exec.Command(sharedconsts.ProgramFFprobe, "-v", "error", "-show_entries", entries, "-of", "csv=p=0", path).CombinedOutput()
	if err != nil {
		t.Fatalf("probing %s: %v\n%s", path, err, out)
	}
	found := strings.Fields(string(out))
	for i := range found {
		found[i] = strings.TrimRight(found[i], ",") // FFmpeg 9 can end an entry with a comma.
	}
	slices.Sort(found)
	return found
}

// testStreams returns a file's streams as "codec,type,is a thumbnail", sorted.
func testStreams(t *testing.T, path string) []string {
	t.Helper()
	return testProbe(t, path, "stream=codec_name,codec_type:stream_disposition=attached_pic")
}

// executeTestVideo runs a video through ExecuteVideo as outExt, returning the new video's path.
func executeTestVideo(t *testing.T, fd *models.FileData, path, outExt string) string {
	t.Helper()
	t.Cleanup(func() {
		abstractions.Set(keys.OutputFiletype, "")
		abstractions.Set(keys.Thumbnail, "")
		abstractions.Set(keys.StripThumbnails, false)
	})
	abstractions.Set(keys.OutputFiletype, outExt)
	fd.VideoDirectory = filepath.Dir(path)
	fd.OriginalVideoPath = path
	if err := ExecuteVideo(t.Context(), fd); err != nil {
		t.Fatal(err)
	}
	return fd.PostFFmpegVideoPath
}

// TestStreamsKeptWithFFmpeg tests every audio and subtitle track reaches the new video, subtitles converted as needed.
func TestStreamsKeptWithFFmpeg(t *testing.T) {
	const (
		aac     = "aac,audio,0"
		mpeg4   = "mpeg4,video,0"
		movText = "mov_text,subtitle,0"
		subrip  = "subrip,subtitle,0"
	)
	inputs := testVideoInputs(t)

	for _, tc := range []struct {
		name, file, subtitleCodec, outExt string
		want                              []string
	}{
		{"mkv to mkv", "v.mkv", "srt", sharedconsts.ExtMKV, []string{aac, aac, mpeg4, subrip, subrip}},
		{"mkv to mp4", "v.mkv", "srt", sharedconsts.ExtMP4, []string{aac, aac, movText, movText, mpeg4}},
		{"mp4 to mp4", "v.mp4", "mov_text", sharedconsts.ExtMP4, []string{aac, aac, movText, movText, mpeg4}},
		{"mp4 to mkv", "v.mp4", "mov_text", sharedconsts.ExtMKV, []string{aac, aac, mpeg4, subrip, subrip}},
		{"mkv to avi leaves subtitles out", "v.mkv", "srt", sharedconsts.ExtAVI, []string{aac, aac, mpeg4}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), tc.file)
			args := append(slices.Clone(inputs), "-map", "0", "-map", "1", "-map", "2", "-map", "3", "-map", "4",
				"-c:v", "mpeg4", "-c:a", "aac", "-c:s", tc.subtitleCodec, path)
			runTestFFmpeg(t, args...)

			got := testStreams(t, executeTestVideo(t, models.NewFileData(), path, tc.outExt))
			if !slices.Equal(got, tc.want) {
				t.Errorf("streams = %v, want %v", got, tc.want)
			}
		})
	}
}

// TestTimecodeTrackWithFFmpeg tests a video with a timecode track can be rewritten, which copying the track would fail.
func TestTimecodeTrackWithFFmpeg(t *testing.T) {
	inputs := testVideoInputs(t)
	path := filepath.Join(t.TempDir(), "v.mp4")
	runTestFFmpeg(t, append(slices.Clone(inputs), "-map", "0", "-map", "1", "-c:v", "mpeg4", "-c:a", "aac", "-timecode", "00:00:00:00", path)...)

	got := testStreams(t, executeTestVideo(t, models.NewFileData(), path, sharedconsts.ExtMP4))
	if !slices.Contains(got, "aac,audio,0") || !slices.Contains(got, "mpeg4,video,0") {
		t.Errorf("streams = %v, want the audio and video kept", got)
	}
}

// TestSubtitlesToWebMWithFFmpeg tests subtitles convert to the only kind WebM holds.
func TestSubtitlesToWebMWithFFmpeg(t *testing.T) {
	inputs := testVideoInputs(t)
	encoders := availableEncoders(t.Context())
	if !strings.Contains(encoders, " libvpx-vp9 ") || !strings.Contains(encoders, " libopus ") {
		t.Skip("FFmpeg can't encode VP9 and Opus")
	}

	path := filepath.Join(t.TempDir(), "v.mkv")
	runTestFFmpeg(t, append(slices.Clone(inputs), "-map", "0", "-map", "1", "-map", "3", "-map", "4",
		"-c:v", "libvpx-vp9", "-deadline", "realtime", "-c:a", "libopus", "-c:s", "srt", path)...)

	got := testStreams(t, executeTestVideo(t, models.NewFileData(), path, sharedconsts.ExtWEBM))
	if want := []string{"opus,audio,0", "vp9,video,0", "webvtt,subtitle,0", "webvtt,subtitle,0"}; !slices.Equal(got, want) {
		t.Errorf("streams = %v, want %v", got, want)
	}
}

// TestCopyToContainersNeedingConversionWithFFmpeg tests copied H.264 stays readable in the containers FFmpeg won't convert it for.
func TestCopyToContainersNeedingConversionWithFFmpeg(t *testing.T) {
	inputs := testVideoInputs(t)
	if !strings.Contains(availableEncoders(t.Context()), " libx264 ") {
		t.Skip("FFmpeg can't encode H.264")
	}

	for _, file := range []string{"v.mkv", "v.mp4"} {
		for _, outExt := range []string{sharedconsts.ExtAVI, sharedconsts.ExtMPEG, sharedconsts.ExtMPG, sharedconsts.ExtVOB} {
			t.Run(file+" to "+outExt, func(t *testing.T) {
				path := filepath.Join(t.TempDir(), file)
				runTestFFmpeg(t, append(slices.Clone(inputs), "-map", "0", "-map", "1", "-c:v", "libx264", "-pix_fmt", "yuv420p", "-c:a", "aac", path)...)

				// Count frames, as the file's still written when FFmpeg can't convert it, with no video that plays.
				out := executeTestVideo(t, models.NewFileData(), path, outExt)
				probe, err := exec.Command(sharedconsts.ProgramFFprobe, "-v", "error", "-count_frames", "-select_streams", "v:0",
					"-show_entries", "stream=codec_name,nb_read_frames", "-of", "csv=p=0", out).Output()
				if err != nil {
					t.Fatal(err)
				}
				if got, want := strings.TrimSpace(string(probe)), "h264,2"; got != want {
					t.Errorf("video stream = %q, want %q", got, want)
				}
			})
		}
	}
}

// TestEncodedVideoGetsNoFilterWithFFmpeg tests a filter meant for copied H.264 isn't put on video encoded to something else.
func TestEncodedVideoGetsNoFilterWithFFmpeg(t *testing.T) {
	inputs := testVideoInputs(t)
	if !strings.Contains(availableEncoders(t.Context()), " libx264 ") {
		t.Skip("FFmpeg can't encode H.264")
	}
	t.Cleanup(func() { abstractions.Set(keys.TranscodeVideoCodecMap, nil) })
	abstractions.Set(keys.TranscodeVideoCodecMap, map[sharedmodels.FromCodec]string{
		sharedmodels.NewFromCodec(sharedconsts.VCodecH264, ""): sharedconsts.VCodecMPEG2,
	})

	path := filepath.Join(t.TempDir(), "v.mkv")
	runTestFFmpeg(t, append(slices.Clone(inputs), "-map", "0", "-map", "1", "-c:v", "libx264", "-pix_fmt", "yuv420p", "-c:a", "aac", path)...)

	got := testProbe(t, executeTestVideo(t, models.NewFileData(), path, sharedconsts.ExtMPG), "stream=codec_name")
	if !slices.Contains(got, "mpeg2video") {
		t.Errorf("codecs = %v, want mpeg2video", got)
	}
}

// TestThumbnailsWithFFmpeg tests an MP4's thumbnail stays a thumbnail, unless it's stripped or replaced, and is never doubled up.
func TestThumbnailsWithFFmpeg(t *testing.T) {
	const (
		aac       = "aac,audio,0"
		thumbnail = "mjpeg,video,1"
		mpeg4     = "mpeg4,video,0"
	)
	inputs := testVideoInputs(t)
	newThumbnail := makeTestThumbnail(t, "48x48")

	for _, tc := range []struct {
		name, thumbnail string
		strip           bool
		want            []string
	}{
		{"kept", "", false, []string{aac, thumbnail, mpeg4}},
		{"stripped", "", true, []string{aac, mpeg4}},
		{"replaced", newThumbnail, false, []string{aac, thumbnail, mpeg4}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := makeThumbnailVideo(t, inputs, "v.mp4")
			abstractions.Set(keys.Thumbnail, tc.thumbnail)
			abstractions.Set(keys.StripThumbnails, tc.strip)
			got := testStreams(t, executeTestVideo(t, models.NewFileData(), path, sharedconsts.ExtMP4))
			if !slices.Equal(got, tc.want) {
				t.Errorf("streams = %v, want %v", got, tc.want)
			}
		})
	}
}

// TestThumbnailToAnotherContainerWithFFmpeg tests a video's own thumbnail goes with it wherever a container can hold one.
func TestThumbnailToAnotherContainerWithFFmpeg(t *testing.T) {
	const (
		aac       = "aac,audio,0"
		thumbnail = "mjpeg,video,1"
		mpeg4     = "mpeg4,video,0"
	)
	inputs := testVideoInputs(t)

	for _, tc := range []struct {
		name, file, outExt string
		strip              bool
		want               []string
	}{
		{"mp4 to mkv", "v.mp4", sharedconsts.ExtMKV, false, []string{aac, thumbnail, mpeg4}},
		{"mp4 to m4v", "v.mp4", sharedconsts.ExtM4V, false, []string{aac, thumbnail, mpeg4}},
		{"mkv to mp4", "v.mkv", sharedconsts.ExtMP4, false, []string{aac, thumbnail, mpeg4}},
		{"mp4 to mkv, stripped", "v.mp4", sharedconsts.ExtMKV, true, []string{aac, mpeg4}},
		{"mp4 to mov, which can't hold one", "v.mp4", sharedconsts.ExtMOV, false, []string{aac, mpeg4}},
		{"mp4 to avi, which can't hold one", "v.mp4", sharedconsts.ExtAVI, false, []string{aac, mpeg4}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tmp := t.TempDir()
			t.Setenv("TMPDIR", tmp)

			// An MKV's thumbnail is attached, and shows as a video stream once read.
			path := makeThumbnailVideo(t, inputs, "v.mp4")
			if tc.file == "v.mkv" {
				source := filepath.Join(t.TempDir(), tc.file)
				runTestFFmpeg(t, append(slices.Clone(inputs), "-map", "0", "-map", "1", "-c:v", "mpeg4", "-c:a", "aac",
					"-attach", makeTestThumbnail(t, "32x32"), "-metadata:s:t:0", "mimetype=image/jpeg", source)...)
				path = source
			}

			abstractions.Set(keys.StripThumbnails, tc.strip)
			got := testStreams(t, executeTestVideo(t, models.NewFileData(), path, tc.outExt))
			if !slices.Equal(got, tc.want) {
				t.Errorf("streams = %v, want %v", got, tc.want)
			}

			left, err := os.ReadDir(tmp)
			if err != nil {
				t.Fatal(err)
			}
			for _, entry := range left {
				t.Errorf("left in the temp dir: %s", entry.Name())
			}
		})
	}
}

// takeOutStream returns the bytes of a video's stream, as FFmpeg stores them.
func takeOutStream(t *testing.T, video, stream string) []byte {
	t.Helper()
	out := filepath.Join(t.TempDir(), "out.jpg")
	runTestFFmpeg(t, "-i", video, "-map", stream, "-c", "copy", "-f", "image2", "-update", "1", out)
	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

// takeOutThumbnail returns the bytes of a video's second video stream, its thumbnail.
func takeOutThumbnail(t *testing.T, video string) []byte {
	t.Helper()
	return takeOutStream(t, video, "0:v:1")
}

// TestMKVThumbnailsStayWithFFmpeg tests rewriting an MKV keeps every image attached as it was, on every run, unless stripped.
func TestMKVThumbnailsStayWithFFmpeg(t *testing.T) {
	inputs := testVideoInputs(t)
	dir := t.TempDir()
	jpg := filepath.Join(dir, "first.jpg")
	runTestFFmpeg(t, "-f", "lavfi", "-i", "testsrc2=s=64x36:d=0.04,noise=alls=12:allf=t", "-frames:v", "1", "-q:v", "2", jpg)
	png := filepath.Join(dir, "second.png")
	runTestFFmpeg(t, "-f", "lavfi", "-i", "testsrc2=s=64x36:d=0.04,noise=alls=12:allf=t", "-frames:v", "1", png)
	font := filepath.Join(dir, "font.ttf")
	if err := os.WriteFile(font, []byte("not really a font"), 0o644); err != nil {
		t.Fatal(err)
	}
	wantJPG, err := os.ReadFile(jpg)
	if err != nil {
		t.Fatal(err)
	}
	wantPNG, err := os.ReadFile(png)
	if err != nil {
		t.Fatal(err)
	}

	// makeVideo makes an MKV with a font and two images attached, under their own names.
	makeVideo := func(t *testing.T) string {
		t.Helper()
		path := filepath.Join(t.TempDir(), "v.mkv")
		runTestFFmpeg(t, append(slices.Clone(inputs), "-map", "0", "-map", "1", "-c:v", "mpeg4", "-c:a", "aac",
			"-attach", font, "-metadata:s:t:0", "mimetype=font/ttf",
			"-attach", jpg, "-metadata:s:t:1", "mimetype=image/jpeg", "-metadata:s:t:1", "filename=my_cover.jpg",
			"-attach", png, "-metadata:s:t:2", "mimetype=image/png", "-metadata:s:t:2", "filename=small_cover.png", path)...)
		return path
	}

	t.Run("kept", func(t *testing.T) {
		path := makeVideo(t)
		for run := 1; run <= 2; run++ {
			out := executeTestVideo(t, models.NewFileData(), path, sharedconsts.ExtMKV)
			streams := testStreams(t, out)
			for _, want := range []string{"aac,audio,0", "mjpeg,video,1", "mpeg4,video,0", "png,video,1"} {
				if !slices.Contains(streams, want) {
					t.Errorf("run %d: streams = %v, missing %s", run, streams, want)
				}
			}
			if slices.Contains(streams, "mjpeg,video,0") || slices.Contains(streams, "png,video,0") {
				t.Errorf("run %d: an image became a plain video track: %v", run, streams)
			}
			tags := testProbe(t, out, "stream_tags=filename,mimetype")
			for _, want := range []string{"my_cover.jpg,image/jpeg", "small_cover.png,image/png", "font.ttf,font/ttf"} {
				if !slices.Contains(tags, want) {
					t.Errorf("run %d: attachments = %v, missing %s", run, tags, want)
				}
			}
			if got := takeOutStream(t, out, "0:v:1"); !bytes.Equal(got, wantJPG) {
				t.Errorf("run %d: JPG is %d bytes, want the original %d", run, len(got), len(wantJPG))
			}
			if got := takeOutStream(t, out, "0:v:2"); !bytes.Equal(got, wantPNG) {
				t.Errorf("run %d: PNG is %d bytes, want the original %d", run, len(got), len(wantPNG))
			}
		}
	})

	t.Run("stripped, font kept", func(t *testing.T) {
		abstractions.Set(keys.StripThumbnails, true)
		out := executeTestVideo(t, models.NewFileData(), makeVideo(t), sharedconsts.ExtMKV)
		streams := testStreams(t, out)
		if slices.Contains(streams, "mjpeg,video,1") || slices.Contains(streams, "png,video,1") {
			t.Errorf("streams = %v, want no thumbnails", streams)
		}
		if tags := testProbe(t, out, "stream_tags=filename,mimetype"); !slices.Contains(tags, "font.ttf,font/ttf") {
			t.Errorf("attachments = %v, want the font kept", tags)
		}
	})

	t.Run("both go into an MP4", func(t *testing.T) {
		out := executeTestVideo(t, models.NewFileData(), makeVideo(t), sharedconsts.ExtMP4)
		got := testStreams(t, out)
		if want := []string{"aac,audio,0", "mjpeg,video,1", "mjpeg,video,1", "mpeg4,video,0"}; !slices.Equal(got, want) {
			t.Errorf("streams = %v, want %v", got, want)
		}
		if first := takeOutStream(t, out, "0:v:1"); !bytes.Equal(first, wantJPG) {
			t.Errorf("first thumbnail is %d bytes, want the original %d", len(first), len(wantJPG))
		}
		if second := takeOutStream(t, out, "0:v:2"); bytes.Equal(second, wantJPG) {
			t.Error("the first image went in twice")
		}
	})
}

// TestRetriedCommandSameThumbnailsWithFFmpeg tests building the command again, as a retry does, doesn't add the thumbnail twice.
func TestRetriedCommandSameThumbnailsWithFFmpeg(t *testing.T) {
	inputs := testVideoInputs(t)
	t.Setenv("TMPDIR", t.TempDir())
	newThumbnail := makeTestThumbnail(t, "48x48")
	mkv := filepath.Join(t.TempDir(), "v.mkv")
	runTestFFmpeg(t, append(slices.Clone(inputs), "-map", "0", "-map", "1", "-c:v", "mpeg4", "-c:a", "aac",
		"-attach", makeTestThumbnail(t, "32x32"), "-metadata:s:t:0", "mimetype=image/jpeg", mkv)...)
	mp4 := filepath.Join(t.TempDir(), "v.mp4")
	runTestFFmpeg(t, append(slices.Clone(inputs), "-map", "0", "-map", "1", "-c:v", "mpeg4", "-c:a", "aac", mp4)...)

	for _, tc := range []struct {
		name, path, thumbnail, outExt string
		once                          []string
	}{
		{"own thumbnail, mkv", mkv, "", sharedconsts.ExtMKV, []string{"-attach "}},
		{"new thumbnail, mkv", mkv, newThumbnail, sharedconsts.ExtMKV, []string{"-attach "}},
		{"new thumbnail, mp4", mp4, newThumbnail, sharedconsts.ExtMP4, []string{"-i " + newThumbnail, "-disposition:v:1 attached_pic"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Cleanup(func() { abstractions.Set(keys.Thumbnail, "") })
			abstractions.Set(keys.Thumbnail, tc.thumbnail)
			fd := models.NewFileData()
			fd.OriginalVideoPath = tc.path
			b, err := newFfCommandBuilder(fd, filepath.Join(t.TempDir(), "out"+tc.outExt))
			if err != nil {
				t.Fatal(err)
			}
			defer b.removeTempThumbnails()

			for attempt := 1; attempt <= 2; attempt++ {
				args, err := b.buildCommand(t.Context(), fd, "", "", tc.outExt, false)
				if err != nil {
					t.Fatal(err)
				}
				joined := strings.Join(args, " ")
				for _, once := range tc.once {
					if strings.Count(joined, once) != 1 {
						t.Errorf("attempt %d: want %q once in: %s", attempt, once, joined)
					}
				}
			}
		})
	}
}

// TestThumbnailUnchangedWithFFmpeg tests a JPG thumbnail keeps its exact bytes, new or kept, in MP4 and MKV.
func TestThumbnailUnchangedWithFFmpeg(t *testing.T) {
	inputs := testVideoInputs(t)

	// Detail a low-quality re-encode would lose.
	detailed := filepath.Join(t.TempDir(), "detailed.jpg")
	runTestFFmpeg(t, "-f", "lavfi", "-i", "testsrc2=s=640x360:d=0.04,noise=alls=12:allf=t", "-frames:v", "1", "-q:v", "2", detailed)
	want, err := os.ReadFile(detailed)
	if err != nil {
		t.Fatal(err)
	}

	for _, outExt := range []string{sharedconsts.ExtMP4, sharedconsts.ExtM4V, sharedconsts.ExtMKV} {
		t.Run("new, into "+outExt, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "v.mp4")
			runTestFFmpeg(t, append(slices.Clone(inputs), "-map", "0", "-map", "1", "-c:v", "mpeg4", "-c:a", "aac", path)...)
			abstractions.Set(keys.Thumbnail, detailed)
			got := takeOutThumbnail(t, executeTestVideo(t, models.NewFileData(), path, outExt))
			if !bytes.Equal(got, want) {
				t.Errorf("thumbnail is %d bytes, want the original %d", len(got), len(want))
			}
		})
	}

	for _, tc := range []struct{ name, file, outExt string }{
		{"kept, mp4 to mkv", "v.mp4", sharedconsts.ExtMKV},
		{"kept, mkv to mp4", "v.mkv", sharedconsts.ExtMP4},
		{"kept, mkv to mkv", "v.mkv", sharedconsts.ExtMKV},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), tc.file)
			args := append(slices.Clone(inputs), "-i", detailed, "-map", "0", "-map", "1", "-c:v:0", "mpeg4", "-c:a", "aac")
			if tc.file == "v.mkv" {
				args = append(args, "-attach", detailed, "-metadata:s:t:0", "mimetype=image/jpeg", path)
			} else {
				args = append(args, "-map", "6", "-c:v:1", "copy", "-disposition:v:1", "attached_pic", path)
			}
			runTestFFmpeg(t, args...)

			got := takeOutThumbnail(t, executeTestVideo(t, models.NewFileData(), path, tc.outExt))
			if !bytes.Equal(got, want) {
				t.Errorf("thumbnail is %d bytes, want the original %d", len(got), len(want))
			}
		})
	}
}

// TestThumbnailConvertedByContentWithFFmpeg tests a thumbnail is converted for what it holds, whatever its name says.
func TestThumbnailConvertedByContentWithFFmpeg(t *testing.T) {
	inputs := testVideoInputs(t)
	dir := t.TempDir()
	png := filepath.Join(dir, "thumbnail.png")
	runTestFFmpeg(t, "-f", "lavfi", "-i", "testsrc=d=0.1:s=48x48", "-frames:v", "1", png)
	data, err := os.ReadFile(png)
	if err != nil {
		t.Fatal(err)
	}
	misnamed := filepath.Join(dir, "misnamed.jpg")
	if err := os.WriteFile(misnamed, data, 0o644); err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.FileServer(http.Dir(dir)))
	defer server.Close()

	for _, tc := range []struct{ name, flag, metafile string }{
		{"PNG file", png, ""},
		{"misnamed file", misnamed, ""},
		{"PNG download", "", server.URL + "/thumbnail.png"},
		{"misnamed download", "", server.URL + "/misnamed.jpg"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tmp := t.TempDir()
			t.Setenv("TMPDIR", tmp)

			path := filepath.Join(t.TempDir(), "v.mp4")
			runTestFFmpeg(t, append(slices.Clone(inputs), "-map", "0", "-map", "1", "-c:v", "mpeg4", "-c:a", "aac", path)...)
			abstractions.Set(keys.Thumbnail, tc.flag)
			fd := models.NewFileData()
			fd.MWebData.Thumbnail = tc.metafile
			got := testStreams(t, executeTestVideo(t, fd, path, sharedconsts.ExtMP4))
			if want := []string{"aac,audio,0", "mjpeg,video,1", "mpeg4,video,0"}; !slices.Equal(got, want) {
				t.Errorf("streams = %v, want %v", got, want)
			}

			left, err := os.ReadDir(tmp)
			if err != nil {
				t.Fatal(err)
			}
			for _, entry := range left {
				t.Errorf("left in the temp dir: %s", entry.Name())
			}
		})
	}
}

// TestConvertToJPGQualityWithFFmpeg tests a conversion keeps more detail than FFmpeg's default quality.
func TestConvertToJPGQualityWithFFmpeg(t *testing.T) {
	testVideoInputs(t)
	dir := t.TempDir()
	png := filepath.Join(dir, "detailed.png")
	runTestFFmpeg(t, "-f", "lavfi", "-i", "testsrc2=s=640x360:d=0.04,noise=alls=12:allf=t", "-frames:v", "1", png)
	plain := filepath.Join(dir, "plain.jpg")
	runTestFFmpeg(t, "-i", png, plain)

	converted, err := convertToJPG(png, filepath.Join(dir, "converted.jpg"))
	if err != nil {
		t.Fatal(err)
	}
	plainInfo, err := os.Stat(plain)
	if err != nil {
		t.Fatal(err)
	}
	convertedInfo, err := os.Stat(converted)
	if err != nil {
		t.Fatal(err)
	}
	if convertedInfo.Size() <= 2*plainInfo.Size() {
		t.Errorf("converted JPG is %d bytes, against %d at FFmpeg's default quality", convertedInfo.Size(), plainInfo.Size())
	}
}

// TestNewThumbnailKeepsAttachmentsWithFFmpeg tests a thumbnail added to an MKV leaves its other attachments and their mimetypes alone.
func TestNewThumbnailKeepsAttachmentsWithFFmpeg(t *testing.T) {
	inputs := testVideoInputs(t)
	dir := t.TempDir()
	font := filepath.Join(dir, "font.ttf")
	if err := os.WriteFile(font, []byte("not really a font"), 0o644); err != nil {
		t.Fatal(err)
	}

	path := filepath.Join(dir, "v.mkv")
	runTestFFmpeg(t, append(slices.Clone(inputs), "-map", "0", "-map", "1", "-c:v", "mpeg4", "-c:a", "aac",
		"-attach", font, "-metadata:s:t:0", "mimetype=font/ttf", path)...)

	abstractions.Set(keys.Thumbnail, makeTestThumbnail(t, "48x48"))
	out := executeTestVideo(t, models.NewFileData(), path, sharedconsts.ExtMKV)
	got := testProbe(t, out, "stream_tags=filename,mimetype")
	if want := []string{"cover.jpg,image/jpeg", "font.ttf,font/ttf"}; !slices.Equal(got, want) {
		t.Errorf("attachments = %v, want %v", got, want)
	}
}

// TestNewThumbnailKeepsSecondVideoTrackWithFFmpeg tests a new thumbnail doesn't take the place of a second video track.
func TestNewThumbnailKeepsSecondVideoTrackWithFFmpeg(t *testing.T) {
	inputs := testVideoInputs(t)
	path := filepath.Join(t.TempDir(), "v.mp4")
	runTestFFmpeg(t, append(slices.Clone(inputs), "-map", "0", "-map", "0", "-map", "1", "-c:v", "mpeg4", "-c:a", "aac", path)...)

	abstractions.Set(keys.Thumbnail, makeTestThumbnail(t, "48x48"))
	got := testStreams(t, executeTestVideo(t, models.NewFileData(), path, sharedconsts.ExtMP4))
	if want := []string{"aac,audio,0", "mjpeg,video,1", "mpeg4,video,0", "mpeg4,video,0"}; !slices.Equal(got, want) {
		t.Errorf("streams = %v, want %v", got, want)
	}
}

// TestLongNameWithFFmpeg tests a name at the filesystem's limit goes through, with a backup and a thumbnail that needs converting.
func TestLongNameWithFFmpeg(t *testing.T) {
	inputs := testVideoInputs(t)
	t.Setenv("TMPDIR", t.TempDir())
	dir := t.TempDir()
	png := filepath.Join(dir, "thumbnail.png")
	runTestFFmpeg(t, "-f", "lavfi", "-i", "testsrc=d=0.1:s=48x48", "-frames:v", "1", png)

	path := filepath.Join(dir, strings.Repeat("a", sharedconsts.MaxNameBytes-len(sharedconsts.ExtMP4))+sharedconsts.ExtMP4)
	runTestFFmpeg(t, append(slices.Clone(inputs), "-map", "0", "-map", "1", "-c:v", "mpeg4", "-c:a", "aac", path)...)

	abstractions.Set(keys.Thumbnail, png)
	fd := models.NewFileData()
	got := testStreams(t, executeTestVideo(t, fd, path, sharedconsts.ExtMP4))
	if want := []string{"aac,audio,0", "mjpeg,video,1", "mpeg4,video,0"}; !slices.Equal(got, want) {
		t.Errorf("streams = %v, want %v", got, want)
	}
	if _, err := os.Stat(fd.BackupVideoPath); err != nil {
		t.Errorf("backup of the original: %v", err)
	}
}

// TestThumbnailTempFilesRemovedWithFFmpeg tests a downloaded thumbnail and its JPG copy are gone once the video is done.
func TestThumbnailTempFilesRemovedWithFFmpeg(t *testing.T) {
	inputs := testVideoInputs(t)
	tmp := t.TempDir()
	t.Setenv("TMPDIR", tmp)
	dir := t.TempDir()
	png := filepath.Join(dir, "thumbnail.png")
	runTestFFmpeg(t, "-f", "lavfi", "-i", "testsrc=d=0.1:s=48x48", "-frames:v", "1", png)
	server := httptest.NewServer(http.FileServer(http.Dir(dir)))
	defer server.Close()

	path := filepath.Join(dir, "v.mp4")
	runTestFFmpeg(t, append(slices.Clone(inputs), "-map", "0", "-map", "1", "-c:v", "mpeg4", "-c:a", "aac", path)...)

	fd := models.NewFileData()
	fd.MWebData.Thumbnail = server.URL + "/thumbnail.png"
	got := testStreams(t, executeTestVideo(t, fd, path, sharedconsts.ExtMP4))
	if want := []string{"aac,audio,0", "mjpeg,video,1", "mpeg4,video,0"}; !slices.Equal(got, want) {
		t.Errorf("streams = %v, want %v", got, want)
	}

	left, err := os.ReadDir(tmp)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range left {
		t.Errorf("left in the temp dir: %s", entry.Name())
	}
}

// TestGetCodecsUsesMappingNames tests FFprobe's codec names come back as the ones codec mappings use.
func TestGetCodecsUsesMappingNames(t *testing.T) {
	inputs := testVideoInputs(t)
	path := filepath.Join(t.TempDir(), "v.mkv")
	runTestFFmpeg(t, append(slices.Clone(inputs), "-map", "0", "-map", "1", "-c:v", "mpeg2video", "-c:a", "pcm_s16le", path)...)

	video, audio, err := getCodecs(path)
	if err != nil {
		t.Fatal(err)
	}
	if video.Codec != sharedconsts.VCodecMPEG2 || audio.Codec != sharedconsts.ACodecPCM {
		t.Errorf("codecs = %q, %q; want %q, %q", video.Codec, audio.Codec, sharedconsts.VCodecMPEG2, sharedconsts.ACodecPCM)
	}
}
