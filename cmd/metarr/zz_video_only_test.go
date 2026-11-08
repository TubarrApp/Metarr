//go:build unix

package main

import (
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"metarr/internal/domain/keys"

	"github.com/TubarrApp/gocommon/sharedconsts"
)

// TestVideoWithoutMetafile runs Metarr on a video given with no metafile, which keeps its own tags and still goes through
// the video settings.
func TestVideoWithoutMetafile(t *testing.T) {
	if _, err := exec.LookPath(sharedconsts.ProgramFFmpeg); err != nil {
		t.Skip("FFmpeg isn't installed")
	}
	const title = "Kept Title"

	for _, c := range []struct {
		name        string
		ext         string
		thumbnail   bool
		flags       []string
		wantFFmpeg  bool
		wantName    string // The video's name afterwards, in the output directory if one is set.
		wantOutDir  bool
		wantNoThumb bool
	}{
		{name: "nothing to change", ext: sharedconsts.ExtMKV, flags: []string{"--" + keys.WriteNFO}, wantName: "video.mkv"},
		{name: "new container keeps its tags", ext: sharedconsts.ExtMKV, flags: []string{"--" + keys.OutputFiletype + "=mp4"}, wantFFmpeg: true, wantName: "video.mp4"},
		{name: "thumbnail stripped", ext: sharedconsts.ExtMP4, thumbnail: true, flags: []string{"--" + keys.StripThumbnails}, wantFFmpeg: true, wantName: "video.mp4", wantNoThumb: true},
		{name: "thumbnail kept", ext: sharedconsts.ExtMP4, thumbnail: true, wantName: "video.mp4"},
		{
			name: "renamed, moved and purge passed over", ext: sharedconsts.ExtMKV,
			flags:    []string{"--" + keys.FilenameOpsInput + "=prefix:new ", "--" + keys.MetaPurge + "=all"},
			wantName: "new video.mkv", wantOutDir: true,
		},
		{name: "template keeps the name", ext: sharedconsts.ExtMKV, flags: []string{"--" + keys.FilenameOpsInput + "=prefix:{{title}} "}, wantName: "video.mkv", wantOutDir: true},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			video := makeTaggedVideo(t, dir, c.ext, title, c.thumbnail)

			flags := append([]string{"--" + keys.VideoFiles + "=" + video}, c.flags...)
			wantPath := filepath.Join(dir, c.wantName)
			if c.wantOutDir {
				out := filepath.Join(dir, "out")
				flags = append(flags, "--"+keys.OutputDirectory+"="+out)
				wantPath = filepath.Join(out, c.wantName)
			}
			run := runReencodeMetarr(t, flags)

			// FFmpeg run or not.
			if got := run.command != ""; got != c.wantFFmpeg {
				t.Errorf("FFmpeg ran = %t, want %t\ncommand: %s", got, c.wantFFmpeg, run.command)
			}

			// Where the video is, as reported, with no metafile reported.
			if !strings.Contains(run.output, "final video path: "+wantPath+"\n") {
				t.Errorf("final video path not %q in:\n%s", wantPath, run.output)
			}
			if !strings.Contains(run.output, "final json path: \n") {
				t.Errorf("a final metafile path was reported:\n%s", run.output)
			}

			// Its own tags and thumbnail.
			if got := probeTitle(t, wantPath); got != title {
				t.Errorf("title = %q, want %q", got, title)
			}
			if c.thumbnail {
				if got := hasThumbnail(t, wantPath); got == c.wantNoThumb {
					t.Errorf("has thumbnail = %t, want %t", got, !c.wantNoThumb)
				}
			}

			// No NFO is made with no metadata to fill it.
			if nfos, _ := filepath.Glob(filepath.Join(filepath.Dir(wantPath), "*"+sharedconsts.MExtNFO)); len(nfos) > 0 {
				t.Errorf("NFO written: %v", nfos)
			}
		})
	}
}

// makeTaggedVideo writes a short video with a title tag, and a thumbnail if asked.
func makeTaggedVideo(t *testing.T, dir, ext, title string, thumbnail bool) string {
	t.Helper()
	video := filepath.Join(dir, "video"+ext)
	args := []string{"-v", "error", "-y", "-f", "lavfi", "-i", "testsrc2=s=64x64:d=0.2:r=10"}
	if thumbnail {
		thumb := filepath.Join(dir, "thumb.jpg")
		if out, err := exec.Command(sharedconsts.ProgramFFmpeg, "-v", "error", "-y", "-f", "lavfi", "-i", "color=red:s=32x32", "-frames:v", "1", thumb).CombinedOutput(); err != nil {
			t.Fatalf("making thumbnail: %v\n%s", err, out)
		}
		args = append(args, "-i", thumb, "-map", "0", "-map", "1", "-c:v:1", "copy", "-disposition:v:1", "attached_pic")
	}
	args = append(args, "-c:v:0", "mpeg4", "-metadata", "title="+title, video)
	if out, err := exec.Command(sharedconsts.ProgramFFmpeg, args...).CombinedOutput(); err != nil {
		t.Fatalf("making video: %v\n%s", err, out)
	}
	return video
}

// probeTitle returns a video's title tag.
func probeTitle(t *testing.T, path string) string {
	t.Helper()
	out, err := exec.Command(sharedconsts.ProgramFFprobe, "-v", "error", "-show_entries", "format_tags=title", "-of", "default=nw=1:nk=1", path).Output()
	if err != nil {
		t.Fatalf("probing %s: %v", path, err)
	}
	return strings.TrimSpace(string(out))
}

// hasThumbnail reports whether a video has an attached picture.
func hasThumbnail(t *testing.T, path string) bool {
	t.Helper()
	out, err := exec.Command(sharedconsts.ProgramFFprobe, "-v", "error", "-show_entries", "stream_disposition=attached_pic", "-of", "csv=p=0", path).Output()
	if err != nil {
		t.Fatalf("probing %s: %v", path, err)
	}
	for _, line := range strings.Fields(string(out)) {
		if line == "1" {
			return true
		}
	}
	return false
}
