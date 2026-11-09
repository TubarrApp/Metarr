package ffmpeg

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"maps"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"metarr/internal/abstractions"
	"metarr/internal/domain/consts"
	"metarr/internal/domain/keys"
	"metarr/internal/domain/logger"
	"metarr/internal/ffprobe"
	"metarr/internal/models"
	"metarr/internal/parsing"

	"github.com/TubarrApp/gocommon/sharedconsts"
	"github.com/TubarrApp/gocommon/sharedmodels"
	"github.com/TubarrApp/gocommon/sharedtags"
	"github.com/TubarrApp/gocommon/sharedvalidation"
)

// ******************************** Container tags ************************************************************************************

// TestMain gives the global logger a console writer, since the zero value has none.
func TestMain(m *testing.M) {
	logger.Pl.Console = io.Discard
	os.Exit(m.Run())
}

// Tags FFmpeg renames while writing each container, by lowercase name (from libavformat's conversion tables).
var (
	asfRenames = map[string]string{
		"album_artist": "wm/albumartist", "album": "wm/albumtitle", "artist": "author", "comment": "description",
		"composer": "wm/composer", "encoded_by": "wm/encodedby", "encoder": "wm/encodingsettings", "genre": "wm/genre",
		"language": "wm/language", "filename": "wm/originalfilename", "disc": "wm/partofset",
		"disc_subtitle": "wm/setsubtitle", "publisher": "wm/publisher", "track": "wm/tracknumber",
		"service_provider": "wm/mediastationcallsign", "service_name": "wm/mediastationname",
	}
	aviRenames = map[string]string{
		"artist": "iart", "comment": "icmt", "copyright": "icop", "date": "icrd", "genre": "ignr", "language": "ilng",
		"title": "inam", "album": "iprd", "track": "iprt", "encoder": "isft", "timecode": "ismp", "encoded_by": "itch",
	}
	matroskaRenames = map[string]string{"performer": "lead_performer", "track": "part_number"}
	oggRenames      = map[string]string{
		"album_artist": "albumartist", "track": "tracknumber", "disc": "discnumber", "disc_subtitle": "discsubtitle",
		"comment": "description",
	}
	ffmpegRenames = map[string]map[string]string{
		sharedconsts.ExtASF:  asfRenames,
		sharedconsts.ExtWMV:  asfRenames,
		sharedconsts.ExtAVI:  aviRenames,
		sharedconsts.ExtMKV:  matroskaRenames,
		sharedconsts.ExtWEBM: matroskaRenames,
		sharedconsts.ExtOGM:  oggRenames,
		sharedconsts.ExtOGV:  oggRenames,
	}
)

// fullFileData returns file data with every field the FFmpeg command writes filled in.
func fullFileData() *models.FileData {
	return &models.FileData{
		MTitleDesc: &models.MetadataTitlesDescs{
			Title: "title", Subtitle: "subtitle", Description: "description", LongDescription: "long",
			Synopsis: "synopsis", Summary: "summary", Comment: "comment",
		},
		MCredits: &models.MetadataCredits{
			Actor: "actor", Author: "author", Artist: "artist", AlbumArtist: "album artist", Creator: "creator",
			Studio: "studio", Publisher: "publisher", Producer: "producer", Performer: "performer",
			Composer: "composer", Director: "director", Writer: "writer",
		},
		MDates: &models.MetadataDates{
			CreationTime: "2024-01-02T03:04:05Z", Date: "2024-01-02", OriginallyAvailableAt: "2024-01-02",
			ReleaseDate: "2024-01-02", UploadDate: "2024-01-02", Year: "2024",
		},
		MShowData: &models.MetadataShowData{
			Show: "show", EpisodeID: "S01E02", EpisodeSort: "2", SeasonNumber: "1", SeasonTitle: "season",
			Network: "network",
		},
		MOther: &models.MetadataOtherData{
			Language: "en", Genre: "genre", HDVideo: "2", Album: "album", Track: "3", Disc: "1", Keywords: "a, b",
			Copyright: "copyright",
		},
		MWebData: &models.MetadataWebData{WebpageURL: "https://example.com/watch"},
		MetaOps:  models.NewMetaOps(),
	}
}

// TestContainerTagsUnique tests that no two fields are written as the same tag, including tags FFmpeg renames itself.
func TestContainerTagsUnique(t *testing.T) {
	keys := slices.Sorted(maps.Keys(MetadataTags(fullFileData())))
	for ext := range sharedconsts.AllVidExtensions {
		seen := make(map[string]string, len(keys))
		for _, key := range keys {
			tag := strings.ToLower(parsing.GetContainerKeys(key, ext))
			if tag == "" {
				continue
			}
			if renamed, ok := ffmpegRenames[ext][tag]; ok {
				tag = renamed
			}
			if other, dup := seen[tag]; dup {
				t.Errorf("%s: %q and %q are both written as %q", ext, other, key, tag)
			}
			seen[tag] = key
		}
	}
}

// TestMetadataTagsHoldsEveryField tests that every filled field reaches the FFmpeg command's metadata.
func TestMetadataTagsHoldsEveryField(t *testing.T) {
	tags := MetadataTags(fullFileData())
	for _, key := range []string{
		sharedtags.JAlbumArtist, sharedtags.JNetwork, sharedtags.JAlbum, sharedtags.JCopyright, sharedtags.JDiscNumber,
		sharedtags.JGenre, sharedtags.JHDVideo, sharedtags.JKeywords, sharedtags.JLanguage, sharedtags.JTrackNumber,
		sharedtags.JWebpageURL, sharedtags.JShow, sharedtags.JSeasonNumber, sharedtags.JEpisodeID,
	} {
		if tags[key] == "" {
			t.Errorf("MetadataTags has no %q", key)
		}
	}
}

// TestFinalCommandMetadata tests that fields a container can't hold are left out, and the muxer is named where needed.
func TestFinalCommandMetadata(t *testing.T) {
	tests := []struct {
		ext          string
		wantMuxer    string
		want, absent []string
	}{
		{ext: ".mp4", want: []string{"show=show", "album_artist=album artist", "synopsis=synopsis"}},
		{ext: ".mov", want: []string{"comment=long"}, absent: []string{"synopsis=", "author=", "show="}},
		{ext: ".ogm", wantMuxer: consts.FFmpegMuxerOGV, want: []string{"DESCRIPTION=long"}, absent: []string{"comment="}},
		{ext: ".rmvb", wantMuxer: consts.FFmpegMuxerRM, want: []string{"Comment=long"}, absent: []string{"show="}},
		{ext: ".MKV", want: []string{"ACTOR=actor", "LEAD_PERFORMER=performer", "season_title=season"}},
	}

	for _, tt := range tests {
		t.Run(tt.ext, func(t *testing.T) {
			b := &ffCommandBuilder{
				builder:     &strings.Builder{},
				inputFile:   "in.mkv",
				outputFile:  "out" + tt.ext,
				metadataMap: MetadataTags(fullFileData()),
			}
			args, err := b.buildFinalCommand(nil, false)
			if err != nil {
				t.Fatal(err)
			}

			// Check the metadata arguments.
			for _, w := range tt.want {
				if !slices.Contains(args, w) {
					t.Errorf("args lack %q: %v", w, args)
				}
			}
			for _, a := range tt.absent {
				if slices.ContainsFunc(args, func(arg string) bool { return strings.HasPrefix(arg, a) }) {
					t.Errorf("args hold %q: %v", a, args)
				}
			}

			// Check the muxer, and that the output file stays last.
			gotMuxer := ""
			if i := slices.Index(args, consts.FFmpegFormat); i >= 0 {
				gotMuxer = args[i+1]
			}
			if gotMuxer != tt.wantMuxer {
				t.Errorf("muxer = %q, want %q", gotMuxer, tt.wantMuxer)
			}
			if args[len(args)-1] != b.outputFile {
				t.Errorf("last arg = %q, want the output file", args[len(args)-1])
			}
		})
	}
}

// ******************************** Audio levels **************************************************************************************

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

// ******************************** Bad encoder args **********************************************************************************

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

// ******************************** Bitrates ******************************************************************************************

// TestBitrateFor tests a video gets the bitrate for the largest resolution at or below its shorter side.
func TestBitrateFor(t *testing.T) {
	ladder := map[int]int64{720: 1_500_000, 1080: 3_000_000, 2160: 10_000_000}
	tests := []struct {
		bitrates    map[int]int64
		shorterSide int
		want        int64
	}{
		{ladder, 1080, 3_000_000},
		{ladder, 1440, 3_000_000},
		{ladder, 2160, 10_000_000},
		{ladder, 480, 1_500_000}, // Smaller than every entry.
		{ladder, 0, 1_500_000},   // Size unknown.
		{map[int]int64{0: 2_000_000}, 4320, 2_000_000},
		{nil, 1080, 0},
	}
	for _, tt := range tests {
		if got := bitrateFor(tt.bitrates, tt.shorterSide); got != tt.want {
			t.Errorf("bitrateFor(%v, %d) = %d, want %d", tt.bitrates, tt.shorterSide, got, tt.want)
		}
	}
}

// TestVideoBitrateCapped tests the bitrate is lowered to an MKV source's own, read from the whole file as its stream has none.
func TestVideoBitrateCapped(t *testing.T) {
	if !strings.Contains(availableEncoders(t.Context()), " libx264 ") {
		t.Skip("FFmpeg with libx264 isn't installed")
	}
	in := filepath.Join(t.TempDir(), "in.mkv")
	if out, err := exec.Command(sharedconsts.ProgramFFmpeg, "-v", "error", "-f", "lavfi", "-i", "testsrc=size=160x90:rate=10", "-t", "2",
		"-c:v", "libx264", "-b:v", "100k", in).CombinedOutput(); err != nil {
		t.Fatalf("making the input: %v\n%s", err, out)
	}

	shorterSide, sourceBitrate := ffprobe.ProbeVideoSize(t.Context(), in)
	if shorterSide != 90 || sourceBitrate <= 0 {
		t.Fatalf("probeVideoSize = %d, %d; want 90 and the file's bitrate", shorterSide, sourceBitrate)
	}
	if got := videoBitrate(t.Context(), in, map[int]int64{0: 10_000_000}); got != sourceBitrate {
		t.Errorf("videoBitrate = %d, want the source's %d", got, sourceBitrate)
	}
	if got := videoBitrate(t.Context(), in, map[int]int64{0: 1_000}); got != 1_000 {
		t.Errorf("videoBitrate = %d, want 1000, which is under the source's", got)
	}
}

// TestSetVideoEncoderArgsBitrate tests the chosen bitrate is sent for the main video stream, before the other args.
func TestSetVideoEncoderArgsBitrate(t *testing.T) {
	t.Cleanup(func() { abstractions.Set(keys.TranscodeVideoArgsMap, map[string]sharedmodels.EncoderArgs{}) })
	abstractions.Set(keys.TranscodeVideoArgsMap, map[string]sharedmodels.EncoderArgs{
		sharedconsts.VCodecH264: {Bitrates: map[int]int64{720: 2_000_000, 1080: 4_000_000}, Args: []string{"-preset", "slow"}},
	})

	// No input file, so its size is unknown and the smallest resolution's bitrate is used.
	b := &ffCommandBuilder{builder: &strings.Builder{}, videoCodecSoftware: []string{consts.FFmpegCV0, "libx264"}}
	if err := b.setVideoEncoderArgs(t.Context(), "", sharedconsts.VCodecH264); err != nil {
		t.Fatal(err)
	}
	if want := []string{consts.FFmpegBitrate, strconv.Itoa(2_000_000), "-preset", "slow"}; !slices.Equal(b.encoderArgs, want) {
		t.Errorf("got %q, want %q", b.encoderArgs, want)
	}
}

// ******************************** Codec probing *************************************************************************************

// TestGetCodecsWithoutStreamBitrate tests MKV and WebM codecs are read, though neither stores a bitrate per stream.
func TestGetCodecsWithoutStreamBitrate(t *testing.T) {
	encoders := availableEncoders(t.Context())
	dir := t.TempDir()

	tests := []struct {
		file, video, audio string
		args               []string
	}{
		{"in.mp4", "h264", "aac", []string{"-c:v", "libx264", "-c:a", "aac"}},
		{"in.mkv", "h264", "aac", []string{"-c:v", "libx264", "-c:a", "aac"}},
		{"in.webm", "vp9", "opus", []string{"-c:v", "libvpx-vp9", "-deadline", "realtime", "-c:a", "libopus"}},
	}
	for _, tt := range tests {
		t.Run(tt.file, func(t *testing.T) {
			if !strings.Contains(encoders, " "+tt.args[1]+" ") || !strings.Contains(encoders, " "+tt.args[len(tt.args)-1]+" ") {
				t.Skipf("FFmpeg can't encode %s with %v", tt.file, tt.args)
			}
			in := filepath.Join(dir, tt.file)
			args := append([]string{"-v", "error", "-f", "lavfi", "-i", "testsrc=size=320x180:rate=25", "-f", "lavfi", "-i", "sine",
				"-t", "2"}, tt.args...)
			if out, err := exec.Command(sharedconsts.ProgramFFmpeg, append(args, in)...).CombinedOutput(); err != nil {
				t.Fatalf("making %s: %v\n%s", tt.file, err, out)
			}

			video, audio, err := getCodecs(t.Context(), in)
			if err != nil {
				t.Fatal(err)
			}
			if video.Codec != tt.video || audio.Codec != tt.audio {
				t.Errorf("got video %q, audio %q; want %q, %q", video.Codec, audio.Codec, tt.video, tt.audio)
			}
			if video.Compression == sharedconsts.CompressionBlank {
				t.Errorf("got no video compression level, want one from the file's bitrate")
			}
		})
	}
}

// ******************************** Container codecs **********************************************************************************

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
		if out, err := exec.Command(sharedconsts.ProgramFFmpeg, append(args, src)...).CombinedOutput(); err != nil {
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
	output, runErr := exec.Command(sharedconsts.ProgramFFmpeg, args...).CombinedOutput()

	// Read back the codec FFmpeg wrote.
	var probed string
	if runErr == nil {
		p, err := exec.Command(sharedconsts.ProgramFFprobe, "-v", "error", "-show_entries", "stream=codec_name", "-of", "csv=p=0", out).CombinedOutput()
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

// ******************************** Encoder args **************************************************************************************

// TestSetVideoEncoderArgs tests that each target codec's args are converted for the encoder that actually runs.
func TestSetVideoEncoderArgs(t *testing.T) {
	t.Cleanup(func() { abstractions.Set(keys.TranscodeVideoArgsMap, map[string]sharedmodels.EncoderArgs{}) })
	abstractions.Set(keys.TranscodeVideoArgsMap, map[string]sharedmodels.EncoderArgs{
		sharedconsts.VCodecHEVC: {Quality: "24", PixFmt: "yuv420p10le", VideoFilter: "scale=1280:-2", Args: []string{"-preset", "slow"}},
		sharedconsts.VCodecAV1:  {Quality: "60"},
		sharedconsts.VCodecVP9:  {PixFmt: "yuv444p10le"},
		sharedconsts.VCodecH264: {PixFmt: "yuv422p"},
	})

	// The pixel formats each encoder takes, as FFmpeg 9 lists them.
	encoderPixFmtsMu.Lock()
	encoderPixFmtsCache["libx265"] = []string{"yuv420p", "yuv422p", "yuv444p", "yuv420p10le", "yuv422p10le", "yuv444p10le"}
	encoderPixFmtsCache["libsvtav1"] = []string{"yuv420p", "yuv420p10le"}
	encoderPixFmtsCache["hevc_nvenc"] = []string{"yuv420p", "nv12", "p010le", "yuv444p", "nv16", "p210le", "yuv444p16le"}
	encoderPixFmtsCache["vp9_qsv"] = []string{"nv12", "p010le", "vuyx", "xv30le"}
	encoderPixFmtsCache["h264_qsv"] = []string{"nv12"}
	encoderPixFmtsMu.Unlock()
	software := func(encoder string) []string { return []string{consts.FFmpegCV0, encoder} }

	tests := []struct {
		name       string
		accelType  string
		gpuCodec   []string
		softCodec  []string
		desired    string
		wantArgs   []string
		wantFilter string
		wantErr    bool
	}{
		{"software", "", nil, software("libx265"), sharedconsts.VCodecHEVC,
			[]string{consts.FFmpegCRF, "24", consts.FFmpegPixFmt, "yuv420p10le", "-preset", "slow"}, "scale=1280:-2", false},
		{"copied video gets nothing", "", nil, software(consts.FFVCodecKeyCopy), sharedconsts.VCodecHEVC, nil, "", false},
		{"vaapi joins the upload filter", sharedconsts.AccelTypeVAAPI, software("hevc_vaapi"), nil, sharedconsts.VCodecHEVC,
			[]string{"-qp", "24", "-preset", "slow"}, "scale=1280:-2,format=p010,hwupload", false},
		{"vaapi without args still uploads", sharedconsts.AccelTypeVAAPI, software("mpeg2_vaapi"), nil, sharedconsts.VCodecMPEG2,
			nil, "format=nv12,hwupload", false},
		{"vaapi refuses formats it can't upload", sharedconsts.AccelTypeVAAPI, software("vp9_vaapi"), nil, sharedconsts.VCodecVP9,
			nil, "", true},
		{"vaapi fallen back to software has no upload filter", sharedconsts.AccelTypeVAAPI, nil, software("libx265"), sharedconsts.VCodecHEVC,
			[]string{consts.FFmpegCRF, "24", consts.FFmpegPixFmt, "yuv420p10le", "-preset", "slow"}, "scale=1280:-2", false},
		{"nvenc gets the equivalent 10-bit format", sharedconsts.AccelTypeCuda, software("hevc_nvenc"), nil, sharedconsts.VCodecHEVC,
			[]string{"-rc", "vbr", "-cq", "24", consts.FFmpegPixFmt, "p010le", "-preset", "slow"}, "scale=1280:-2", false},
		{"qsv gets the equivalent 4:4:4 format", sharedconsts.AccelTypeQSV, software("vp9_qsv"), nil, sharedconsts.VCodecVP9,
			[]string{consts.FFmpegPixFmt, "xv30le"}, "", false},
		{"encoder with no such format", sharedconsts.AccelTypeQSV, software("h264_qsv"), nil, sharedconsts.VCodecH264, nil, "", true},
		{"software encoder with no such format", "", nil, software("libsvtav1"), sharedconsts.VCodecVP9, nil, "", true},
		{"gpu quality capped at 51", sharedconsts.AccelTypeCuda, software("av1_nvenc"), nil, sharedconsts.VCodecAV1,
			[]string{"-rc", "vbr", "-cq", "51"}, "", false},
		{"software keeps AV1's higher quality", "", nil, software("libsvtav1"), sharedconsts.VCodecAV1,
			[]string{consts.FFmpegCRF, "60"}, "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := &ffCommandBuilder{builder: &strings.Builder{}, videoCodecGPU: tt.gpuCodec, videoCodecSoftware: tt.softCodec}
			err := b.setVideoEncoderArgs(t.Context(), tt.accelType, tt.desired)
			if (err != nil) != tt.wantErr {
				t.Fatalf("error = %v, want error %v", err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			if got := slices.Concat(b.qualityParameter, b.encoderArgs); !slices.Equal(got, tt.wantArgs) || b.videoFilter != tt.wantFilter {
				t.Errorf("got args %q, filter %q; want %q, %q", got, b.videoFilter, tt.wantArgs, tt.wantFilter)
			}
		})
	}
}

// TestSetGPUAccelerationCodecCopies tests that a GPU encoder is only chosen when the video changes codec.
func TestSetGPUAccelerationCodecCopies(t *testing.T) {
	const encoders = " hevc_vaapi h264_vaapi "
	for _, desired := range []string{sharedconsts.VCodecHEVC, sharedconsts.VCodecCopy, ""} {
		b := &ffCommandBuilder{builder: &strings.Builder{}}
		b.setGPUAccelerationCodec(sharedconsts.AccelTypeVAAPI, sharedconsts.VCodecHEVC, desired, encoders)
		if b.videoCodecGPU != nil {
			t.Errorf("HEVC video wanting %q: got GPU codec %q, want a copy", desired, b.videoCodecGPU)
		}
	}

	b := &ffCommandBuilder{builder: &strings.Builder{}}
	b.setGPUAccelerationCodec(sharedconsts.AccelTypeVAAPI, sharedconsts.VCodecH264, sharedconsts.VCodecHEVC, encoders)
	if want := []string{consts.FFmpegCV0, "hevc_vaapi"}; !slices.Equal(b.videoCodecGPU, want) {
		t.Errorf("H.264 video wanting HEVC: got GPU codec %q, want %q", b.videoCodecGPU, want)
	}
}

// TestBuildFinalCommandGPUNode tests that the GPU device and video filter are only added when the GPU encodes.
func TestBuildFinalCommandGPUNode(t *testing.T) {
	b := &ffCommandBuilder{
		builder:    &strings.Builder{},
		inputFile:  "in.mkv",
		outputFile: "out.mkv",
		gpuNode:    []string{consts.FFmpegDeviceVAAPI, "/dev/dri/renderD128"},
	}
	args, err := b.buildFinalCommand(nil, false)
	if err != nil {
		t.Fatal(err)
	}
	if slices.Contains(args, consts.FFmpegDeviceVAAPI) || slices.Contains(args, consts.FFmpegFilter) {
		t.Errorf("copy run: got %q, want no GPU device or filter", args)
	}

	const upload = "format=nv12,hwupload"
	b.videoCodecGPU = []string{consts.FFmpegCV0, "hevc_vaapi"}
	b.videoFilter = upload
	if args, err = b.buildFinalCommand(nil, false); err != nil {
		t.Fatal(err)
	}
	filter := slices.Index(args, consts.FFmpegFilter)
	if !slices.Contains(args, consts.FFmpegDeviceVAAPI) || filter < 0 || filter+1 >= len(args) || args[filter+1] != upload {
		t.Errorf("GPU run: got %q, want the GPU device and %s %s", args, consts.FFmpegFilter, upload)
	}
}

// ******************************** Fitting codecs ************************************************************************************

// TestFitVideoCodecFromReadOnlyContainer tests codecs Metarr doesn't know are transcoded out of a .rm file rather than
// copied, while known ones that fit, and unknown ones between writable containers, are still copied.
func TestFitVideoCodecFromReadOnlyContainer(t *testing.T) {
	for _, c := range []struct {
		name           string
		inExt, current string
		desired, want  string
		wantErr        bool
	}{
		{name: "RealVideo", inExt: ".rm", current: "rv40", want: sharedconsts.VCodecH264},
		{name: "copy refused", inExt: ".rm", current: "rv40", desired: sharedconsts.VCodecCopy, wantErr: true},
		{name: "unknown codec in MKV", inExt: ".mkv", current: "prores"},
	} {
		got, err := fitVideoCodecToContainer(t.Context(), "in"+c.inExt, sharedconsts.ExtMKV, c.current, c.desired)
		switch {
		case c.wantErr && err == nil:
			t.Errorf("%s: got %q, want an error", c.name, got)
		case !c.wantErr && (err != nil || got != c.want):
			t.Errorf("%s: got %q, %v; want %q", c.name, got, err, c.want)
		}
	}
}

// TestFitAudioCodecFromReadOnlyContainer tests codecs Metarr doesn't know are transcoded out of a .rm file rather than
// copied, while known ones that fit, and unknown ones between writable containers, are still copied.
func TestFitAudioCodecFromReadOnlyContainer(t *testing.T) {
	for _, c := range []struct {
		name           string
		inExt, current string
		desired, want  string
		wantErr        bool
	}{
		{name: "Cook", inExt: ".rm", current: "cook", want: sharedconsts.ACodecAAC},
		{name: "AAC is copied", inExt: ".rm", current: "aac", want: ""},
		{name: "unknown codec in MKV", inExt: ".mkv", current: "aac"},
	} {
		got, err := fitAudioCodecToContainer(t.Context(), "in"+c.inExt, sharedconsts.ExtMKV, c.current, c.desired)
		switch {
		case c.wantErr && err == nil:
			t.Errorf("%s: got %q, want an error", c.name, got)
		case !c.wantErr && (err != nil || got != c.want):
			t.Errorf("%s: got %q, %v; want %q", c.name, got, err, c.want)
		}
	}
}

// ******************************** Keeping the original ******************************************************************************

// TestPurgeOriginalVideo tests the original video is kept unless purged, as a backup if the new video takes its path.
func TestPurgeOriginalVideo(t *testing.T) {
	if _, err := exec.LookPath(sharedconsts.ProgramFFmpeg); err != nil {
		t.Skip("FFmpeg isn't installed")
	}
	t.Cleanup(func() {
		abstractions.Set(keys.OutputFiletype, "")
		abstractions.Set(keys.PurgeOriginalVideo, false)
	})

	for _, tc := range []struct {
		name   string
		outExt string
		purge  bool
	}{
		{"new extension", sharedconsts.ExtMKV, false},
		{"new extension, purged", sharedconsts.ExtMKV, true},
		{"same extension", sharedconsts.ExtMP4, false},
		{"same extension, purged", sharedconsts.ExtMP4, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			abstractions.Set(keys.OutputFiletype, tc.outExt)
			abstractions.Set(keys.PurgeOriginalVideo, tc.purge)

			// Make a short source video.
			dir := t.TempDir()
			orig := filepath.Join(dir, "video"+sharedconsts.ExtMP4)
			if out, err := exec.Command(sharedconsts.ProgramFFmpeg, "-v", "error", "-f", "lavfi", "-i", "testsrc=s=64x36:r=5:d=0.4", "-c:v", "mpeg4", orig).CombinedOutput(); err != nil {
				t.Fatalf("making source video: %v\n%s", err, out)
			}

			fd := models.NewFileData()
			fd.VideoDirectory = dir
			fd.OriginalVideoPath = orig
			if err := ExecuteVideo(t.Context(), fd); err != nil {
				t.Fatal(err)
			}

			// The original keeps its own name, unless the new video took it.
			keptPath := orig
			if tc.outExt == sharedconsts.ExtMP4 {
				keptPath = filepath.Join(dir, "video"+sharedconsts.BackupTag+sharedconsts.ExtMP4)
			}
			_, newErr := os.Stat(fd.PostFFmpegVideoPath)
			_, keptErr := os.Stat(keptPath)
			if newErr != nil || (keptErr == nil) == tc.purge {
				t.Errorf("new video: %v, original kept: %v; want the new video, original kept: %v", newErr, keptErr == nil, !tc.purge)
			}
		})
	}
}

// ******************************** Metadata ******************************************************************************************

// newTestBuilder returns a builder with just what the metadata functions write into.
func newTestBuilder() *ffCommandBuilder {
	return &ffCommandBuilder{builder: &strings.Builder{}, metadataMap: make(map[string]string)}
}

// TestModelNotMutated is what stops the container's preference leaking elsewhere: the
// model's title feeds the video-title directory template, so it must keep holding the
// file's actual title rather than the resolved container value.
func TestModelNotMutated(t *testing.T) {
	td := &models.MetadataTitlesDescs{
		Title:           "This is a video about ...",
		Fulltitle:       "This is a video about cats",
		Description:     "short",
		LongDescription: "long",
	}
	before := *td

	b := newTestBuilder()
	b.addTitlesDescs(&models.FileData{MTitleDesc: td, MetaOps: models.NewMetaOps()})

	if *td != before {
		t.Errorf("addTitlesDescs mutated the shared model:\n before %+v\n after  %+v", before, *td)
	}
}

// TestTitlePrecedence tests every combination of title fields: fulltitle wins, as a metafile title is often cut short.
func TestTitlePrecedence(t *testing.T) {
	const (
		truncated = "This is a video about ..."
		full      = "This is a video about cats"
	)

	var table strings.Builder
	for _, c := range []struct {
		// Described behaviour.
		name string

		// Fields in the model.
		title,
		fulltitle string

		// Desired title tag.
		want string
	}{
		{"fulltitle wins over title",
			truncated, full,
			full},

		{"title used without fulltitle",
			truncated, "",
			truncated},

		{"fulltitle used without title",
			"", full,
			full},

		{"neither",
			"", "",
			""},
	} {
		b := newTestBuilder()
		b.addTitlesDescs(&models.FileData{
			MTitleDesc: &models.MetadataTitlesDescs{Title: c.title, Fulltitle: c.fulltitle},
			MetaOps:    models.NewMetaOps(),
		})

		got := b.metadataMap[sharedtags.JTitle]
		fmt.Fprintf(&table, "\n%s %-30s \ngot: %-28q title: %-28q fulltitle: %q\n", mark(got == c.want), c.name, got, c.title, c.fulltitle)
		if got != c.want {
			t.Errorf("%s: title tag = %q, want %q", c.name, got, c.want)
		}
	}
	t.Log("\nTitle tag: fulltitle > title\n" + table.String())
}

// TestDescriptionPrecedence tests every combination of description fields.
//
// The first of longdescription, long_description and description goes into both the description and
// longdescription tags. long_description is another spelling of longdescription, so it's never a tag of its own.
func TestDescriptionPrecedence(t *testing.T) {
	values := [3]string{"long", "underscored", "short"} // longdescription, long_description, description.

	var table strings.Builder
	for combo := range 1 << len(values) {
		// Fill the fields in this combination.
		var present [3]string
		for i, v := range values {
			if combo&(1<<i) != 0 {
				present[i] = v
			}
		}

		// The first field present wins.
		want := ""
		for _, v := range present {
			if v != "" {
				want = v
				break
			}
		}

		b := newTestBuilder()
		b.addTitlesDescs(&models.FileData{
			MTitleDesc: &models.MetadataTitlesDescs{
				LongDescription:           present[0],
				LongUnderscoreDescription: present[1],
				Description:               present[2],
			},
			MetaOps: models.NewMetaOps(),
		})

		gotDesc := b.metadataMap[sharedtags.JDescription]
		gotLong := b.metadataMap[sharedtags.JLongDesc]
		_, gotUnderscore := b.metadataMap[sharedtags.JLongUnderscoreDesc]
		ok := gotDesc == want && gotLong == want && !gotUnderscore
		fmt.Fprintf(&table, "%s got: %-14q longdescription: %-7q long_description: %-13q description: %q\n",
			mark(ok), gotDesc, present[0], present[1], present[2])
		if !ok {
			t.Errorf("%v: description %q, longdescription %q, long_description written %v; want %q, %q, false",
				present, gotDesc, gotLong, gotUnderscore, want, want)
		}
	}
	t.Log("\nDescription and longdescription tags: longdescription > long_description > description\n" + table.String())
}

// TestDateFollowsYear tests that the date tag is the first full date in the year, or the year alone if none is.
func TestDateFollowsYear(t *testing.T) {
	var table strings.Builder
	for _, c := range []struct {
		// Described behaviour.
		name string

		// Dates in the model.
		releaseDate,
		originallyAvailable,
		date,
		uploadDate,
		year string

		// Desired date tag.
		want string
	}{
		{"release date first",
			"2021-01-02", "2021-02-03", "2021-03-04", "2021-04-05", "2021",
			"2021-01-02"},

		{"originally available next",
			"", "2021-02-03", "2021-03-04", "2021-04-05", "2021",
			"2021-02-03"},

		{"date next",
			"", "", "2021-03-04", "2021-04-05", "2021",
			"2021-03-04"},

		{"upload date last",
			"", "", "", "2021-04-05", "2021",
			"2021-04-05"},

		{"dates from other years skipped",
			"2019-01-02", "", "2021-03-04", "", "2021",
			"2021-03-04"},

		{"no full date in the year, as with a release year",
			"", "", "2021-03-04", "2021-04-05", "2019",
			"2019"},

		{"year with no date",
			"", "", "", "", "2019",
			"2019"},

		{"date with no year",
			"", "", "2021-03-04", "", "",
			"2021-03-04"},
	} {
		b := newTestBuilder()
		b.addDates(&models.MetadataDates{
			ReleaseDate:           c.releaseDate,
			OriginallyAvailableAt: c.originallyAvailable,
			Date:                  c.date,
			UploadDate:            c.uploadDate,
			Year:                  c.year,
		})

		got := b.metadataMap[sharedtags.JDate]
		fmt.Fprintf(&table,
			"\n%s %-50s \ngot: %-13q release_date: %-13q originally_available_at: %-13q date: %-13q upload_date: %-13q year: %q\n\n",
			mark(got == c.want), c.name, got, c.releaseDate, c.originallyAvailable, c.date, c.uploadDate, c.year)
		if got != c.want {
			t.Errorf("%s: date tag = %q, want %q", c.name, got, c.want)
		}
	}
	t.Log("\nDate tag: first full date in the year (release date > originally available > date > upload date), else the year\n" +
		table.String())
}

// TestCreditArrays tests that a single credit wins over a list of them, and a list is joined when there's no single one.
func TestCreditArrays(t *testing.T) {
	var table strings.Builder
	for _, c := range []struct {
		// Described behaviour.
		name string

		// Credits in the model.
		single string
		list   []string

		// Desired director tag.
		want string
	}{
		{"single value only",
			"Ann", nil,
			"Ann"},

		{"list only",
			"", []string{"Ann", "Bob"},
			"Ann; Bob"},

		{"single value wins over list",
			"Ann", []string{"Bob", "Cat"},
			"Ann"},

		{"neither",
			"", nil,
			""},
	} {
		b := newTestBuilder()
		b.addCredits(&models.MetadataCredits{Director: c.single, Directors: c.list})

		got := b.metadataMap[sharedtags.JDirector]
		fmt.Fprintf(&table, "  %s %-28s got: %-11q director: %-6q directors: %q\n",
			mark(got == c.want), c.name, got, c.single, strings.Join(c.list, ", "))
		if got != c.want {
			t.Errorf("%s: director tag = %q, want %q", c.name, got, c.want)
		}
	}
	t.Log("\nDirector tag: the single value, else the list joined\n" + table.String())
}

// mark returns a tick for a passing case and a cross for a failing one.
func mark(ok bool) string {
	if ok {
		return "✓"
	}
	return "✗"
}

// ******************************** Passes ********************************************************************************************

// TestFFmpegPass tests each pass gets FFmpeg's pass number: the first writes stats, middle ones refine them, the last makes the output.
func TestFFmpegPass(t *testing.T) {
	for passes, want := range map[int][]int{2: {1, 2}, 3: {1, 3, 2}} {
		var got []int
		for i := 1; i <= passes; i++ {
			got = append(got, ffmpegPass(i, passes))
		}
		if !slices.Equal(got, want) {
			t.Errorf("%d passes: got FFmpeg passes %v, want %v", passes, got, want)
		}
	}
}

// TestSetVideoEncoderArgsPasses tests software encoders keep their passes, while GPU encoders and copies make one.
func TestSetVideoEncoderArgsPasses(t *testing.T) {
	t.Cleanup(func() { abstractions.Set(keys.TranscodeVideoArgsMap, map[string]sharedmodels.EncoderArgs{}) })
	abstractions.Set(keys.TranscodeVideoArgsMap, map[string]sharedmodels.EncoderArgs{
		sharedconsts.VCodecH264: {Passes: 3, Args: []string{"-b:v", "3M"}},
	})

	tests := []struct {
		name      string
		accelType string
		gpuCodec  []string
		softCodec []string
		want      int
	}{
		{"software", "", nil, []string{consts.FFmpegCV0, "libx264"}, 3},
		{"gpu", sharedconsts.AccelTypeVAAPI, []string{consts.FFmpegCV0, "h264_vaapi"}, nil, 1},
		{"copy", "", nil, []string{consts.FFmpegCV0, consts.FFVCodecKeyCopy}, 1},
	}
	for _, tt := range tests {
		b := &ffCommandBuilder{builder: &strings.Builder{}, videoCodecGPU: tt.gpuCodec, videoCodecSoftware: tt.softCodec}
		if err := b.setVideoEncoderArgs(t.Context(), tt.accelType, sharedconsts.VCodecH264); err != nil {
			t.Fatal(err)
		}
		if b.passes != tt.want {
			t.Errorf("%s: got %d passes, want %d", tt.name, b.passes, tt.want)
		}
	}
}

// TestRunPasses tests one to three passes write the output and leave no pass stats behind.
func TestRunPasses(t *testing.T) {
	if !strings.Contains(availableEncoders(t.Context()), " libx264 ") {
		t.Skip("FFmpeg with libx264 isn't installed")
	}
	dir := t.TempDir()
	t.Setenv("TMPDIR", dir) // Pass stats folders go here, so leftovers can be found.
	in := filepath.Join(dir, "in.mp4")
	if out, err := exec.Command(sharedconsts.ProgramFFmpeg, "-v", "error", "-f", "lavfi", "-i", "testsrc=size=160x90:rate=10", "-t", "1",
		"-c:v", "libx264", in).CombinedOutput(); err != nil {
		t.Fatalf("making the input: %v\n%s", err, out)
	}

	for _, passes := range []int{1, 2, 3} {
		out := filepath.Join(dir, fmt.Sprintf("out%d.mkv", passes))
		args := []string{"-y", "-i", in, consts.FFmpegCV0, "libx264", "-b:v", "100k", out}
		if stderr, err := runPasses(t.Context(), args, passes, 1_000_000, in); err != nil {
			t.Fatalf("%d passes: %v\n%s", passes, err, stderr)
		}
		if info, err := os.Stat(out); err != nil || info.Size() == 0 {
			t.Errorf("%d passes: no output written (%v)", passes, err)
		}
	}
	if leftovers, _ := filepath.Glob(filepath.Join(dir, "metarr-passes-*")); len(leftovers) > 0 {
		t.Errorf("pass stats left behind: %v", leftovers)
	}
}

// ******************************** Progress ******************************************************************************************

// TestProgressWriter tests FFmpeg's -progress blocks become progress lines, split across writes, at most once a second
// and always at the end.
func TestProgressWriter(t *testing.T) {
	var out bytes.Buffer
	p := newProgressWriter(&out, 10_000_000, 2, 3) // 10 seconds, second of three passes.

	block := "frame=120\nfps=29.97\nout_time_us=2500000\nspeed=1.52x\nprogress=continue\n"
	half := len(block) / 2
	for _, chunk := range []string{block[:half], block[half:]} {
		if _, err := p.Write([]byte(chunk)); err != nil {
			t.Fatal(err)
		}
	}
	// A second block straight after is skipped, as under a second has passed.
	if _, err := p.Write([]byte("frame=130\nout_time_us=2700000\nprogress=continue\n")); err != nil {
		t.Fatal(err)
	}
	p.last = time.Now().Add(-2 * time.Second)
	if _, err := p.Write([]byte("frame=200\nfps=30\nout_time_us=N/A\nspeed=N/A\nprogress=continue\n")); err != nil {
		t.Fatal(err)
	}
	if _, err := p.Write([]byte("frame=300\nout_time_us=9990000\nspeed=1.6x\nprogress=end\n")); err != nil {
		t.Fatal(err)
	}

	want := []string{
		"metarr progress: pct=25.0 frame=120 fps=30.0 speed=1.52x eta=4 pass=2 passes=3", // 7.5s of video left at 1.52x.
		"metarr progress: pct=-1.0 frame=200 fps=30.0 speed= eta=-1 pass=2 passes=3",
		"metarr progress: pct=100.0 frame=300 fps=30.0 speed=1.6x eta=0 pass=2 passes=3",
	}
	if got := strings.Split(strings.TrimSpace(out.String()), "\n"); strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("got lines:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

// TestProgressWriterUnknownDuration tests progress has no percentage when the duration is unknown, until FFmpeg ends.
func TestProgressWriterUnknownDuration(t *testing.T) {
	var out bytes.Buffer
	p := newProgressWriter(&out, 0, 1, 1)
	if _, err := p.Write([]byte("frame=10\nout_time_us=500000\nprogress=continue\n")); err != nil {
		t.Fatal(err)
	}
	if got := out.String(); !strings.Contains(got, "pct=-1.0 frame=10") || !strings.Contains(got, "eta=-1") {
		t.Errorf("got %q, want no percentage", got)
	}
}

// ******************************** Re-encoding ***************************************************************************************

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
	video, audio, err := getCodecs(t.Context(), clip)
	if err != nil {
		t.Fatal(err)
	}

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
		args, err := b.buildCommand(t.Context(), fd, video, audio, sharedconsts.VCodecH264, "", sharedconsts.ExtMKV, c.reencode)
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

// ******************************** Stream mapping ************************************************************************************

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
			video, audio, err := getCodecs(t.Context(), tc.path)
			if err != nil {
				t.Fatal(err)
			}

			for attempt := 1; attempt <= 2; attempt++ {
				args, err := b.buildCommand(t.Context(), fd, video, audio, "", "", tc.outExt, false)
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

	video, audio, err := getCodecs(t.Context(), path)
	if err != nil {
		t.Fatal(err)
	}
	if video.Codec != sharedconsts.VCodecMPEG2 || audio.Codec != sharedconsts.ACodecPCM {
		t.Errorf("codecs = %q, %q; want %q, %q", video.Codec, audio.Codec, sharedconsts.VCodecMPEG2, sharedconsts.ACodecPCM)
	}
}

// ******************************** Transcode fixes ***********************************************************************************

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
		video, audio, err := getCodecs(t.Context(), path)
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
