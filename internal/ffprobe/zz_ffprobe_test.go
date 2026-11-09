package ffprobe

import (
	"encoding/json"
	"fmt"
	"io"
	"maps"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"

	"metarr/internal/abstractions"
	"metarr/internal/domain/keys"
	"metarr/internal/domain/logger"
	"metarr/internal/models"
	"metarr/internal/parsing"

	"github.com/TubarrApp/gocommon/sharedconsts"
	"github.com/TubarrApp/gocommon/sharedtags"
)

// ******************************** Compression ***************************************************************************************

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

// ******************************** Helpers *******************************************************************************************

// TestDiffMapReadsProbeNames tests that each written tag is compared with the name FFprobe reports it under.
func TestDiffMapReadsProbeNames(t *testing.T) {
	tests := []struct {
		name  string
		ext   string
		tags  map[string]string
		probe ffprobeOutput
	}{
		{
			name: "avi tags come back under FFmpeg's names",
			ext:  ".avi",
			tags: map[string]string{
				sharedtags.JArtist: "artist", sharedtags.JDescription: "desc", sharedtags.JReleaseDate: "2024-01-02",
				sharedtags.JSynopsis: "syn", sharedtags.JTrackNumber: "3", sharedtags.JAlbum: "album",
			},
			probe: probeTags(ffprobeTags{
				"artist": "artist", "comment": "desc", "date": "2024-01-02", "subject": "syn", "track": "3", "product": "album",
			}),
		},
		{
			name: "asf tags come back converted, or under their Windows Media names",
			ext:  ".wmv",
			tags: map[string]string{
				sharedtags.JArtist: "artist", sharedtags.JComment: "comment", sharedtags.JComposer: "composer",
				sharedtags.JYear: "2024", sharedtags.JShow: "show",
			},
			probe: probeTags(ffprobeTags{
				"artist": "artist", "comment": "comment", "composer": "composer", "WM/Year": "2024", "show": "show",
			}),
		},
		{
			name: "matroska tags come back in capitals, the performer and part converted",
			ext:  ".mkv",
			tags: map[string]string{
				sharedtags.JActor: "actor", sharedtags.JPerformer: "performer", sharedtags.JTrackNumber: "3",
				sharedtags.JTitle: "title", sharedtags.JSeasonTitle: "season",
			},
			probe: probeTags(ffprobeTags{
				"ACTOR": "actor", "performer": "performer", "track": "3", "title": "title", "SEASON_TITLE": "season",
			}),
		},
		{
			name: "ogg tags come back on the stream, the description as the comment",
			ext:  ".ogv",
			tags: map[string]string{
				sharedtags.JDescription: "desc", sharedtags.JAlbumArtist: "album artist", sharedtags.JDiscNumber: "1",
			},
			probe: ffprobeOutput{Streams: []ffprobeStream{{Tags: ffprobeTags{"comment": "desc", "album_artist": "album artist", "disc": "1"}}}},
		},
		{
			name:  "mpeg-ts service tags come back on the program",
			ext:   ".ts",
			tags:  map[string]string{sharedtags.JTitle: "title", sharedtags.JArtist: "artist"},
			probe: ffprobeOutput{Programs: []ffprobeProgram{{Tags: ffprobeTags{"service_name": "title", "service_provider": "artist"}}}},
		},
		{
			name:  "dates compare only their date part",
			ext:   ".mp4",
			tags:  map[string]string{sharedtags.JCreationTime: "2024-01-02T03:04:05Z"},
			probe: probeTags(ffprobeTags{"creation_time": "2024-01-02T00:00:00.000000Z"}),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			diffs := getDiffMapForFiletype(tt.ext, tt.tags, tt.probe)
			if len(diffs) != len(tt.tags) {
				t.Errorf("compared %d tags, want %d: %v", len(diffs), len(tt.tags), diffs)
			}
			for tag, d := range diffs {
				if d.existing != d.new {
					t.Errorf("%q read back as %q, want %q", tag, d.existing, d.new)
				}
			}
		})
	}
}

// TestDiffMapSkipsUnheldFields tests that fields a container can't hold aren't compared.
func TestDiffMapSkipsUnheldFields(t *testing.T) {
	diffs := getDiffMapForFiletype(".mov", map[string]string{sharedtags.JSynopsis: "syn", sharedtags.JTitle: "title"}, ffprobeOutput{})
	if _, ok := diffs[sharedtags.MOVTitle]; !ok || len(diffs) != 1 {
		t.Errorf("diffs = %v, want only the title", diffs)
	}
}

// probeTags returns FFprobe output holding the given format tags.
func probeTags(tags ffprobeTags) ffprobeOutput {
	return ffprobeOutput{Format: ffprobeFormat{Tags: tags}}
}

// ******************************** Round trip script *********************************************************************************

// TestWriteRoundTripScript writes a shell script repeating TestRoundTripWithFFmpeg, for machines without Go.
func TestWriteRoundTripScript(t *testing.T) {
	path := os.Getenv("METARR_ROUNDTRIP_SCRIPT")
	if path == "" {
		t.Skip("set METARR_ROUNDTRIP_SCRIPT to the script path to write")
	}

	flatKey := regexp.MustCompile(`[^A-Za-z0-9]`)
	var b strings.Builder
	b.WriteString(`#!/bin/sh
# Writes every container with each tag Metarr maps, then checks ffprobe reads each one back.
set -u
dir=$(mktemp -d) && cd "$dir" || exit 1
ffmpeg -version | head -n 1
ffmpeg -v error -f lavfi -i color=s=176x144:d=0.2 src.mkv || exit 1
fails=0

# expect <ext> <ffprobe flat output> <section.key> <value prefix>
expect() {
	if printf '%s\n' "$2" | grep -iqF "$3=\"$4"; then :; else
		echo "FAIL $1: $3 is not \"$4\" (got: $(printf '%s\n' "$2" | grep -iF "$3=" | head -n 1))"
		fails=$((fails+1))
	fi
}
`)

	exts := slices.Sorted(maps.Keys(sharedconsts.AllVidExtensions))
	keys := slices.Sorted(maps.Keys(roundTripTags))
	for _, ext := range exts {
		fmt.Fprintf(&b, "\n# %s\n", ext)

		// Write the container.
		b.WriteString("ffmpeg -v error -y -i src.mkv")
		for _, key := range keys {
			if tag := parsing.GetContainerKeys(key, ext); tag != "" {
				fmt.Fprintf(&b, " -metadata '%s=%s'", tag, roundTripTags[key])
			}
		}
		switch ext {
		case sharedconsts.ExtOGM:
			b.WriteString(" -f ogv")
		case sharedconsts.ExtRMVB:
			b.WriteString(" -f rm")
		}
		fmt.Fprintf(&b, " out%s || { echo \"FAIL %s: ffmpeg could not write it\"; fails=$((fails+1)); }\n", ext, ext)

		// 3GP tags can't be read back.
		if ext == sharedconsts.Ext3GP || ext == sharedconsts.Ext3G2 {
			continue
		}

		// Read it back from the section FFprobe reports the container's tags in.
		section := "format.tags."
		switch ext {
		case sharedconsts.ExtMTS, sharedconsts.ExtTS:
			section = "programs.program.0.tags."
		case sharedconsts.ExtOGM, sharedconsts.ExtOGV:
			section = "streams.stream.0.tags."
		}
		fmt.Fprintf(&b, "p=$(ffprobe -v quiet -show_entries format_tags:stream_tags:program_tags -of flat out%s)\n", ext)
		for _, key := range keys {
			tag := parsing.GetContainerKeys(key, ext)
			if tag == "" {
				continue
			}
			value := roundTripTags[key]
			if slices.Contains(sharedtags.DateKeys, key) {
				value = getDatePart(value)
			}
			fmt.Fprintf(&b, "expect %s \"$p\" '%s%s' '%s'\n", ext, section, flatKey.ReplaceAllString(probeKey(tag, ext), "_"), value)
		}
	}

	// Check the .ogm and .rmvb muxers take Metarr's codecs.
	b.WriteString(`
# Codecs through the .ogm and .rmvb muxers.
ffmpeg -v error -y -f lavfi -i color=s=176x144:d=0.2 -f lavfi -i sine=d=0.2 -c:v libvpx -c:a libvorbis -f ogv codecs.ogm || { echo "FAIL .ogm: vp8 + vorbis"; fails=$((fails+1)); }
ffmpeg -v error -y -f lavfi -i color=s=176x144:d=0.2 -f lavfi -i sine=d=0.2 -c:v libx264 -c:a aac -f rm codecs.rmvb || { echo "FAIL .rmvb: h264 + aac"; fails=$((fails+1)); }

echo "Done: $fails failures"
rm -rf "$dir"
`)

	if err := os.WriteFile(path, []byte(b.String()), 0o755); err != nil {
		t.Fatal(err)
	}
}

// ******************************** Round trip ****************************************************************************************

// roundTripTags holds a value for every field the FFmpeg command writes.
var roundTripTags = map[string]string{
	sharedtags.JTitle: "Title", sharedtags.JSubtitle: "Subtitle", sharedtags.JComment: "Comment",
	sharedtags.JDescription: "Description", sharedtags.JLongDesc: "Description", sharedtags.JSummary: "Summary",
	sharedtags.JSynopsis: "Synopsis", sharedtags.JActor: "Actor", sharedtags.JAuthor: "Author",
	sharedtags.JArtist: "Artist", sharedtags.JAlbumArtist: "Album Artist", sharedtags.JCreator: "Creator",
	sharedtags.JStudio: "Studio", sharedtags.JPublisher: "Publisher", sharedtags.JProducer: "Producer",
	sharedtags.JPerformer: "Performer", sharedtags.JComposer: "Composer", sharedtags.JDirector: "Director",
	sharedtags.JWriter: "Writer", sharedtags.JCreationTime: "2024-01-02T03:04:05Z", sharedtags.JDate: "2024-01-02",
	sharedtags.JOriginallyAvailable: "2024-01-02", sharedtags.JReleaseDate: "2024-01-02",
	sharedtags.JUploadDate: "2024-01-02", sharedtags.JYear: "2024", sharedtags.JEpisodeID: "S01E02",
	sharedtags.JEpisodeSort: "2", sharedtags.JNetwork: "Network", sharedtags.JSeasonNumber: "1",
	sharedtags.JSeasonTitle: "Season", sharedtags.JShow: "Show", sharedtags.JAlbum: "Album",
	sharedtags.JCopyright: "Copyright", sharedtags.JDiscNumber: "1", sharedtags.JGenre: "Genre",
	sharedtags.JHDVideo: "2", sharedtags.JKeywords: "a, b", sharedtags.JLanguage: "eng",
	sharedtags.JTrackNumber: "3", sharedtags.JWebpageURL: "https://example.com/watch",
}

// TestRoundTripWithFFmpeg writes every container with the installed FFmpeg and checks FFprobe reads each tag back.
func TestRoundTripWithFFmpeg(t *testing.T) {
	if os.Getenv("METARR_FFMPEG_ROUNDTRIP") == "" {
		t.Skip("set METARR_FFMPEG_ROUNDTRIP=1 to run against the installed FFmpeg")
	}

	// Make a short source clip.
	dir := t.TempDir()
	src := filepath.Join(dir, "src.mkv")
	if out, err := exec.Command(sharedconsts.ProgramFFmpeg, "-v", "error", "-f", "lavfi", "-i", "color=s=176x144:d=0.2", src).CombinedOutput(); err != nil {
		t.Fatalf("making source clip: %v\n%s", err, out)
	}
	tags := roundTripTags

	for ext := range sharedconsts.AllVidExtensions {
		t.Run(ext, func(t *testing.T) {
			if ext == sharedconsts.Ext3GP || ext == sharedconsts.Ext3G2 {
				t.Skip("FFprobe can't read 3GP tags back")
			}

			// Write the container with every tag it can hold, as the FFmpeg command does.
			out := filepath.Join(dir, "out"+ext)
			args := []string{"-v", "error", "-y", "-i", src}
			for key, value := range tags {
				if tag := parsing.GetContainerKeys(key, ext); tag != "" {
					args = append(args, "-metadata", tag+"="+value)
				}
			}
			switch ext {
			case sharedconsts.ExtOGM:
				args = append(args, "-f", "ogv")
			case sharedconsts.ExtRMVB:
				args = append(args, "-f", "rm")
			}
			if output, err := exec.Command(sharedconsts.ProgramFFmpeg, append(args, out)...).CombinedOutput(); err != nil {
				t.Skipf("FFmpeg can't write %s here: %v\n%s", ext, err, output)
			}

			// Read it back and compare.
			probed, err := exec.Command(sharedconsts.ProgramFFprobe, "-v", "quiet", "-print_format", "json", "-show_format", "-show_streams", "-show_programs", out).Output()
			if err != nil {
				t.Fatalf("ffprobe: %v", err)
			}
			var ffData ffprobeOutput
			if err := json.Unmarshal(probed, &ffData); err != nil {
				t.Fatal(err)
			}
			for tag, d := range getDiffMapForFiletype(ext, tags, ffData) {
				if d.existing != d.new {
					t.Errorf("%q read back as %q, want %q", tag, d.existing, d.new)
				}
			}
		})
	}
}

// ******************************** Thumbnail matching ********************************************************************************

// TestEmbeddedThumbnailNotInMetafile tests a video isn't reprocessed for having a thumbnail its metafile doesn't name, unless it's being stripped.
func TestEmbeddedThumbnailNotInMetafile(t *testing.T) {
	if _, err := exec.LookPath(sharedconsts.ProgramFFmpeg); err != nil {
		t.Skip("FFmpeg isn't installed")
	}
	logger.Pl.Console = io.Discard
	t.Cleanup(func() { abstractions.Set(keys.StripThumbnails, false) })

	dir := t.TempDir()
	thumbnail := filepath.Join(dir, "thumbnail.jpg")
	if out, err := exec.Command(sharedconsts.ProgramFFmpeg, "-nostdin", "-v", "error", "-f", "lavfi", "-i", "testsrc=d=0.1:s=32x32", "-frames:v", "1", thumbnail).CombinedOutput(); err != nil {
		t.Fatalf("making thumbnail: %v\n%s", err, out)
	}
	path := filepath.Join(dir, "v.mp4")
	if out, err := exec.Command(sharedconsts.ProgramFFmpeg, "-nostdin", "-v", "error", "-f", "lavfi", "-i", "testsrc=s=64x36:r=5:d=0.4", "-i", thumbnail,
		"-map", "0", "-map", "1", "-c:v:0", "mpeg4", "-c:v:1", "copy", "-disposition:v:1", "attached_pic", path).CombinedOutput(); err != nil {
		t.Fatalf("making source video: %v\n%s", err, out)
	}

	for _, strip := range []bool{false, true} {
		abstractions.Set(keys.StripThumbnails, strip)
		fd := models.NewFileData()
		fd.OriginalVideoPath = path
		if got := CheckMetaMatches(t.Context(), sharedconsts.ExtMP4, fd, map[string]string{}); got == strip {
			t.Errorf("strip %v: metadata matches = %v", strip, got)
		}
		if !fd.HasEmbeddedThumbnail {
			t.Errorf("strip %v: thumbnail not noticed", strip)
		}
	}
}

// TestMissingThumbnailOnlyMattersWhereItFits tests a video without the metadata's thumbnail is only reprocessed in a container that can hold one.
func TestMissingThumbnailOnlyMattersWhereItFits(t *testing.T) {
	if _, err := exec.LookPath(sharedconsts.ProgramFFmpeg); err != nil {
		t.Skip("FFmpeg isn't installed")
	}
	logger.Pl.Console = io.Discard

	path := filepath.Join(t.TempDir(), "v.mp4")
	if out, err := exec.Command(sharedconsts.ProgramFFmpeg, "-nostdin", "-v", "error", "-f", "lavfi", "-i", "testsrc=s=64x36:r=5:d=0.4", "-c:v", "mpeg4", path).CombinedOutput(); err != nil {
		t.Fatalf("making source video: %v\n%s", err, out)
	}
	fd := models.NewFileData()
	fd.OriginalVideoPath = path
	fd.MWebData.Thumbnail = "https://example.com/thumb.jpg"

	for ext, want := range map[string]bool{
		sharedconsts.ExtWEBM: true,
		sharedconsts.ExtMOV:  true,
		sharedconsts.ExtMP4:  false,
		sharedconsts.ExtMKV:  false,
	} {
		if got := CheckMetaMatches(t.Context(), ext, fd, map[string]string{}); got != want {
			t.Errorf("CheckMetaMatches for %s = %v, want %v", ext, got, want)
		}
	}
}
