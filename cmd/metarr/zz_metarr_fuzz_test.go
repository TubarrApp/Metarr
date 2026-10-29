package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
	"unicode"

	"github.com/TubarrApp/gocommon/sharedconsts"
)

// metarrAsMainEnv makes the test binary run as Metarr. Metarr keeps its settings in globals, so the fuzzer runs it in a
// fresh process each time, and this way that process runs the code under test without a separate build.
const metarrAsMainEnv = "METARR_FUZZ_AS_MAIN"

// TestMain runs the test binary as Metarr when the fuzzer starts it with metarrAsMainEnv set.
func TestMain(m *testing.M) {
	if os.Getenv(metarrAsMainEnv) == "1" {
		os.Exit(run())
	}
	os.Exit(m.Run())
}

// fuzzRunTimeout is how long one Metarr run on the fuzzer's tiny clips may take before it counts as hung.
const fuzzRunTimeout = 90 * time.Second

// fuzzVideoExts are the extensions of the videos Metarr can leave.
var fuzzVideoExts = []string{".mp4", ".mkv", ".webm", ".mov", ".avi", ".flv", ".m4v", ".ts", ".wmv"}

// fuzzFlag is a Metarr flag and the values the fuzzer picks from. In a value, "%s" becomes the fuzzed text, and "%n" the
// fuzzed text made safe as one file name (so a fuzzed output folder can't land outside the test's).
type fuzzFlag struct {
	name   string
	values []string
	repeat int // How many times it can be given.
}

// fuzzFlags are the flags the fuzzer gives Metarr, as Tubarr does and beyond.
func fuzzFlags(encoders string) []fuzzFlag {
	videoCodecs := []string{"copy", "%s"}
	if strings.Contains(encoders, " libx264 ") {
		videoCodecs = append(videoCodecs, "h264", "hevc:h264")
	}
	if strings.Contains(encoders, " libx265 ") {
		videoCodecs = append(videoCodecs, "hevc", "h264:hevc")
	}
	audioCodecs := []string{"copy", "aac", "%s"}
	if strings.Contains(encoders, " libopus ") {
		audioCodecs = append(audioCodecs, "opus", "aac:opus")
	}
	if strings.Contains(encoders, " libmp3lame ") {
		audioCodecs = append(audioCodecs, "mp3")
	}
	bools := []string{"true", "false"}
	return []fuzzFlag{
		{"filename-ops", []string{"prefix:[%s] ", "suffix: %s", "replace:a:%s", "date-tag:prefix:ymd", "date-tag:suffix:dmy",
			"delete-date-tag:all:ymd", "set:%s", "keep-after:%s", "regex-replace:[aeiou]:%s", "%s"}, 2},
		{"meta-ops", []string{"title:set:%s", "title:prefix:%s ", "title:suffix: %s", "artist:set:%s", "title:replace:a:%s",
			"title:date-tag:prefix:ymd", "description:copy-to:synopsis", "synopsis:paste-from:title", "title:keep-before:%s",
			"all-titles:suffix:!", "title:regex-replace:[aeiou]:%s", "%s"}, 3},
		{"filtered-meta-ops", []string{"title:contains:%s||artist:set:Matched", "uploader:equals:Fuzz Uploader||title:prefix:[U] ", "%s||%s"}, 1},
		{"filtered-filename-ops", []string{"title:contains:%s||prefix:[F] ", "title:omits:zzz||suffix: [F]", "%s||%s"}, 1},
		{"rename-style", []string{"spaces", "underscores", "fixes-only", "skip", "%s"}, 1},
		{"output-ext", []string{"mp4", "mkv", "webm", "mov", "%s"}, 1},
		{"output-directory", []string{"out", "out/{{year}}", "out/{{video_title}}", "out/%n"}, 1},
		{"purge-metafile", []string{"all", "json", "nfo", "", "%s"}, 1},
		{"transcode-video-codecs", videoCodecs, 1},
		{"transcode-audio-codecs", audioCodecs, 1},
		{"transcode-quality", []string{"23", "0", "51", "%s"}, 1},
		{"transcode-preset", []string{"ultrafast", "%s"}, 1},
		{"transcode-video-filter", []string{"hflip", "scale=32:-2", "%s"}, 1},
		{"transcode-gpu", []string{"none", "%s"}, 1},
		{"extra-ffmpeg-args", []string{"-threads 1", "-movflags +faststart"}, 1},
		{"concurrency", []string{"1", "2", "%s"}, 1},
		{"max-cpu", []string{"50", "101"}, 1},
		{"min-free-mem", []string{"0", "1M", "%s"}, 1},
		{"meta-overwrite", bools, 1},
		{"meta-preserve", bools, 1},
		{"write-nfo", bools, 1},
		{"no-file-overwrite", bools, 1},
		{"strip-thumbnail", bools, 1},
		{"force-write-thumbnail", bools, 1},
		{"skip-videos", bools, 1},
		{"purge-original-video", bools, 1},
	}
}

// fuzzChoices hands out the fuzzed choices in order, then zeros once they run out.
type fuzzChoices []byte

// next returns the next choice.
func (c *fuzzChoices) next() int {
	if len(*c) == 0 {
		return 0
	}
	b := (*c)[0]
	*c = (*c)[1:]
	return int(b)
}

// safeName makes s usable as a single file name, so it can't name a path outside its folder. Colons become full-width,
// as yt-dlp names Tubarr's downloads (Metarr refuses paths holding colons).
func safeName(s string) string {
	s = strings.Map(func(r rune) rune {
		switch {
		case r == '/' || r == '\\':
			return '-'
		case r == ':':
			return '：'
		case unicode.IsControl(r) || r == unicode.ReplacementChar:
			return -1
		}
		return r
	}, strings.ToValidUTF8(s, ""))
	if r := []rune(strings.TrimSpace(s)); len(r) > 60 {
		s = string(r[:60])
	}
	s = strings.TrimSpace(s)
	if s == "" || s == "." || s == ".." {
		return "video"
	}
	return s
}

// metarrFuzzRun is one fuzzed Metarr run: its folders, inputs and flags.
type metarrFuzzRun struct {
	t      *testing.T
	root   string            // Holds work and home, and nothing else.
	work   string            // Where Metarr runs, holding the inputs and any output folder.
	inputs int               // How many videos Metarr is given.
	clip   []byte            // Each input video's contents.
	chosen map[string]string // The last value given for each flag.
	args   []string
}

// newMetarrFuzzRun lays out the folders and inputs, and picks the flags.
func newMetarrFuzzRun(t *testing.T, clip []byte, encoders string, choices []byte, text, title string) *metarrFuzzRun {
	t.Helper()
	text = strings.ReplaceAll(text, "\x00", "") // Arguments can't hold NUL bytes, so no caller can send one.
	r := &metarrFuzzRun{t: t, root: t.TempDir(), clip: clip, chosen: map[string]string{}}
	r.work = filepath.Join(r.root, "work")
	in := filepath.Join(r.work, "in")
	if err := os.MkdirAll(in, 0o755); err != nil {
		t.Fatal(err)
	}
	c := fuzzChoices(choices)

	r.inputs = c.next()%2 + 1
	for i := range r.inputs {
		name := safeName(title)
		if i > 0 {
			name = "second " + name
		}
		video, meta := filepath.Join(in, name+".mp4"), filepath.Join(in, name+".json")
		if err := os.WriteFile(video, clip, 0o644); err != nil {
			t.Fatal(err)
		}
		info, err := json.Marshal(map[string]any{
			"id":          "fuzz" + string(rune('a'+i)),
			"title":       title,
			"fulltitle":   title,
			"description": "About " + text,
			"uploader":    "Fuzz Uploader",
			"channel":     "Fuzz Channel",
			"upload_date": "20240102",
			"webpage_url": "https://example.com/watch?v=fuzz",
			"ext":         "mp4",
		})
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(meta, info, 0o644); err != nil {
			t.Fatal(err)
		}
		r.args = append(r.args, "--video-file="+video, "--meta-file="+meta)
	}

	for _, fl := range fuzzFlags(encoders) {
		for range c.next() % (fl.repeat + 1) {
			v := fl.values[c.next()%len(fl.values)]
			v = strings.ReplaceAll(strings.ReplaceAll(v, "%n", safeName(text)), "%s", text)
			if fl.name == "output-directory" {
				v = filepath.Join(r.work, v)
			}
			r.chosen[fl.name] = v
			r.args = append(r.args, "--"+fl.name+"="+v)
		}
	}
	return r
}

// exec runs Metarr, returning its output and whether it succeeded.
func (r *metarrFuzzRun) exec(logURL string) (output string, ok bool) {
	r.t.Helper()
	home := filepath.Join(r.root, "home")
	if err := os.Mkdir(home, 0o755); err != nil {
		r.t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(r.t.Context(), fuzzRunTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, os.Args[0], r.args...)
	cmd.Dir = r.work
	cmd.Env = []string{"PATH=" + os.Getenv("PATH"), "HOME=" + home, "TUBARR_LOG_URL=" + logURL, metarrAsMainEnv + "=1"}
	cmd.WaitDelay = 5 * time.Second
	out, err := cmd.CombinedOutput()
	output = string(out)

	var exitErr *exec.ExitError
	switch {
	case ctx.Err() != nil:
		r.t.Fatalf("Metarr hung for %v with %q:\n%s", fuzzRunTimeout, r.args, output)
	case err != nil && !errors.As(err, &exitErr):
		r.t.Fatalf("running Metarr: %v", err)
	case strings.Contains(output, "Panic recovered") || strings.Contains(output, "panic:"):
		r.t.Fatalf("Metarr panicked with %q:\n%s", r.args, output)
	case err != nil && exitErr.ExitCode() != 1:
		r.t.Fatalf("Metarr exited %d with %q:\n%s", exitErr.ExitCode(), r.args, output)
	}
	return output, err == nil
}

// check checks what Metarr left, whether it succeeded or not.
func (r *metarrFuzzRun) check(output string, ok bool) {
	r.t.Helper()
	fail := func(format string, a ...any) {
		r.t.Helper()
		r.t.Errorf(format+"\nflags: %q\noutput:\n%s", append(a, r.args, output)...)
	}

	// Nothing outside the test's folder.
	entries, err := os.ReadDir(r.root)
	if err != nil {
		r.t.Fatal(err)
	}
	for _, e := range entries {
		if e.Name() != "work" && e.Name() != "home" {
			fail("Metarr wrote %q outside its working folder", e.Name())
		}
	}

	var videos, jsons, nfos []string
	if err := filepath.WalkDir(r.work, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		name := d.Name()
		switch ext := strings.ToLower(filepath.Ext(name)); {
		case strings.HasPrefix(name, sharedconsts.TempTag):
			fail("Metarr left the temporary file %q", path)
		case slices.Contains(fuzzVideoExts, ext):
			videos = append(videos, path)
		case ext == ".json":
			jsons = append(jsons, path)
			if data, err := os.ReadFile(path); err != nil || !json.Valid(data) {
				fail("Metarr left %q, which isn't valid JSON", path)
			}
		case ext == ".nfo":
			nfos = append(nfos, path)
		}
		return nil
	}); err != nil {
		r.t.Fatal(err)
	}

	// No video lost, whatever happened.
	playable := 0
	for _, v := range videos {
		if fuzzPlayable(v) {
			playable++
		}
	}
	if playable < r.inputs {
		fail("Metarr was given %d video(s), but %d playable one(s) remain: %q", r.inputs, playable, videos)
	}

	// The clips are always valid, so once Metarr accepts its flags, the run should work, unless it refuses a codec chosen
	// for a video whose container can't hold it (which it can only tell once it has read the video).
	if !ok {
		if !strings.Contains(output, "Failed to execute cobra") && !strings.Contains(output, "can't go in a") {
			fail("Metarr accepted its flags, then failed")
		}
		return
	}

	// A successful run did what its flags asked. Unchanged originals and backups aren't among the videos it made.
	var final []string
	kept := 0
	for _, v := range videos {
		if data, err := os.ReadFile(v); err == nil && bytes.Equal(data, r.clip) {
			kept++
		} else if !strings.Contains(v, sharedconsts.BackupTag) {
			final = append(final, v)
		}
	}
	if r.chosen["purge-original-video"] != "true" && kept < r.inputs {
		fail("Metarr succeeded without --purge-original-video, but kept %d of %d original video(s) unchanged: %q", kept, r.inputs, videos)
	}
	if dir, set := r.chosen["output-directory"]; set && r.chosen["skip-videos"] != "true" { // Skipped videos stay put.
		base, _, _ := strings.Cut(
			strings.TrimPrefix(dir, r.work+string(filepath.Separator)),
			string(filepath.Separator),
		)
		for _, v := range final {
			if !strings.HasPrefix(v, filepath.Join(r.work, base)+string(filepath.Separator)) {
				fail("Metarr succeeded with --output-directory %q, but left %q outside it", dir, v)
			}
		}
	}
	if purge := r.chosen["purge-metafile"]; purge == "all" || purge == "json" {
		for _, j := range jsons {
			if !strings.Contains(j, sharedconsts.BackupTag) {
				fail("Metarr succeeded with --purge-metafile %q, but left %q", purge, j)
			}
		}
	}
	if ext := r.chosen["output-ext"]; slices.Contains([]string{"mp4", "mkv", "webm", "mov"}, ext) && r.chosen["skip-videos"] != "true" {
		for _, v := range final {
			if !strings.EqualFold(filepath.Ext(v), "."+ext) {
				fail("Metarr succeeded with --output-ext %q, but left %q", ext, v)
			}
		}
	}
	if r.chosen["write-nfo"] == "true" && len(nfos) == 0 {
		fail("Metarr succeeded with --write-nfo, but wrote no .nfo file")
	}
}

// fuzzPlayable reports whether ffprobe finds a video stream in path.
func fuzzPlayable(path string) bool {
	out, err := exec.Command("ffprobe", "-v", "error", "-select_streams", "v:0", "-show_entries", "stream=codec_type",
		"-of", "csv=p=0", path).Output()
	return err == nil && strings.TrimSpace(string(out)) == "video"
}

// seedChoices returns the choices giving inputs videos and each flag named in picks once, with the value at its index.
func seedChoices(inputs int, picks map[string]string) []byte {
	choices := []byte{byte(inputs - 1)}
	for _, fl := range fuzzFlags(" libx264 libx265 libopus libmp3lame ") {
		v, ok := picks[fl.name]
		if !ok {
			choices = append(choices, 0)
			continue
		}
		choices = append(choices, 1, byte(slices.Index(fl.values, v)))
	}
	return choices
}

// metarrFuzzSeeds are starting points: realistic flag sets as Tubarr sends them, and odd text and titles.
var metarrFuzzSeeds = []struct {
	choices     []byte
	text, title string
}{
	{seedChoices(1, nil), "", "Simple Title"},
	{seedChoices(1, map[string]string{
		"filename-ops": "prefix:[%s] ", "meta-ops": "title:set:%s", "rename-style": "spaces", "output-ext": "mkv",
		"output-directory": "out", "purge-metafile": "json", "write-nfo": "true", "meta-overwrite": "true",
	}), "Cats", "Cats & Dogs: 2024"},
	{seedChoices(2, map[string]string{
		"filename-ops": "date-tag:prefix:ymd", "meta-ops": "title:date-tag:prefix:ymd", "output-directory": "out/{{year}}",
		"purge-metafile": "all", "no-file-overwrite": "true", "concurrency": "2", "output-ext": "mp4",
	}), "dogs", "AC/DC — Live: \"Encore\" <2026> 🎸"},
	{seedChoices(1, map[string]string{
		"transcode-video-codecs": "h264", "transcode-audio-codecs": "aac", "transcode-quality": "23",
		"transcode-preset": "ultrafast", "transcode-video-filter": "hflip", "extra-ffmpeg-args": "-threads 1",
		"output-directory": "out/{{video_title}}",
	}), "x", "日本語のタイトル {{year}}"},
	{seedChoices(1, map[string]string{
		"filtered-meta-ops": "title:contains:%s||artist:set:Matched", "filtered-filename-ops": "title:contains:%s||prefix:[F] ",
		"skip-videos": "true", "rename-style": "underscores", "meta-preserve": "true",
	}), "Long", strings.Repeat("Long title ", 30)},
	{seedChoices(2, map[string]string{
		"output-ext": "mov", "strip-thumbnail": "true", "force-write-thumbnail": "true", "meta-ops": "description:copy-to:synopsis",
		"output-directory": "out/%n", "rename-style": "fixes-only",
	}), "../escape", "  spaced  out  "},
	{seedChoices(1, map[string]string{"meta-ops": "%s", "filename-ops": "%s"}), "%s|:{{video_title}}", ""},
}

// FuzzMetarr runs Metarr on tiny clips with fuzzed flags and text, as Tubarr would and beyond, and checks it never
// panics, hangs, loses a video, leaves temporary files or invalid JSON, or writes outside its folder, and that a
// successful run did what its flags asked.
func FuzzMetarr(f *testing.F) {
	for _, tool := range []string{"ffmpeg", "ffprobe"} {
		if _, err := exec.LookPath(tool); err != nil {
			f.Skipf("%s isn't installed: %v", tool, err)
		}
	}
	clipPath := filepath.Join(f.TempDir(), "clip.mp4")
	if out, err := exec.Command("ffmpeg", "-loglevel", "error", "-y",
		"-f", "lavfi", "-i", "testsrc=duration=1:size=64x64:rate=5",
		"-f", "lavfi", "-i", "sine=d=1",
		"-shortest", "-pix_fmt", "yuv420p", clipPath).CombinedOutput(); err != nil {
		f.Fatalf("making the test clip: %v: %s", err, out)
	}
	clip, err := os.ReadFile(clipPath)
	if err != nil {
		f.Fatal(err)
	}
	encoders, err := exec.Command("ffmpeg", "-hide_banner", "-encoders").Output()
	if err != nil {
		f.Fatal(err)
	}

	// Metarr posts its logs to Tubarr, so they go here instead of to a Tubarr running on this machine.
	logs := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
	}))
	f.Cleanup(logs.Close)

	for _, s := range metarrFuzzSeeds {
		f.Add(s.choices, s.text, s.title)
	}
	f.Fuzz(func(t *testing.T, choices []byte, text, title string) {
		r := newMetarrFuzzRun(t, clip, string(encoders), choices, text, title)
		output, ok := r.exec(logs.URL)
		r.check(output, ok)
	})
}
