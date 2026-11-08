package ffprobe

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"math"
	"metarr/internal/abstractions"
	"metarr/internal/domain/consts"
	"metarr/internal/domain/keys"
	"metarr/internal/domain/logger"
	"metarr/internal/models"
	"metarr/internal/parsing"
	"os/exec"
	"strconv"
	"strings"

	"github.com/TubarrApp/gocommon/logging"
	"github.com/TubarrApp/gocommon/sharedconsts"
)

// tagDiff holds the existing and new values of a metadata tag, for comparison.
type tagDiff struct {
	existing string
	new      string
}

// tagDiffMap maps metadata tag names to their existing and new values, for comparison.
type tagDiffMap map[string]tagDiff

// CheckMetaMatches checks FFprobe captured metadata from the video against the tags FFmpeg would write.
func CheckMetaMatches(ctx context.Context, extension string, fd *models.FileData, tags map[string]string) (allMetaMatches bool) {
	command := exec.CommandContext(
		ctx,
		sharedconsts.ProgramFFprobe,
		"-v", "quiet",
		"-print_format", "json",
		"-show_format", "-show_streams", "-show_programs",
		fd.OriginalVideoPath,
	)

	logger.Pl.I("Made command for FFprobe:\n\n%v", command.String())
	output, err := command.Output()
	if err != nil {
		logger.Pl.E("Error running FFprobe command: %v. Will not process video.", err)
		return false
	}

	// Parse JSON output.
	var ffData ffprobeOutput
	if err := json.Unmarshal(output, &ffData); err != nil {
		logger.Pl.E("Error parsing FFprobe output: %v. Will not process video.", err)
		return false
	}

	// Output FFprobe data for debugging.
	logger.Pl.D(4, "FFprobe output for %q:\n%+v", fd.OriginalVideoPath, ffData)

	// Check if thumbnail is already present in file.
	for _, s := range ffData.Streams {
		if s.Disposition.AttachedPic == 1 && s.CodecType == "video" {
			logger.Pl.I("Video %q has an embedded thumbnail", fd.OriginalVideoPath)
			fd.HasEmbeddedThumbnail = true
			break
		}
	}

	// Get strip thumbnail bool.
	stripThumbnail := false
	if abstractions.IsSet(keys.StripThumbnails) {
		stripThumbnail = abstractions.GetBool(keys.StripThumbnails)
	}

	// Thumbnail is in file but user wants to strip.
	if fd.HasEmbeddedThumbnail && stripThumbnail {
		logger.Pl.I("Thumbnail exists in video %q, set to be stripped", fd.OriginalVideoPath)
		return false
	}

	// No thumbnail in file but thumbnail exists in metadata, in a file that can hold one.
	if !fd.HasEmbeddedThumbnail && fd.EmbedThumbnail() != "" && parsing.HoldsThumbnail(extension) {
		logger.Pl.I("No thumbnail in video %q, found thumbnail %q", fd.OriginalVideoPath, fd.EmbedThumbnail())
		return false
	}

	// Map of metadata to check.
	metaCheckMap := getDiffMapForFiletype(extension, tags, ffData)

	// Collect all metadata for logging.
	ffContent := make([]string, 0, len(metaCheckMap))
	matches := true

	// Check each field.
	for key, values := range metaCheckMap {
		printVals := fmt.Sprintf("Currently in video: Key=%s, Value=%s, New Value=%s", key, values.existing, values.new)
		ffContent = append(ffContent, printVals)

		// FFmpeg only ever adds or overwrites tags, never clears them (empty
		// values are skipped when building the command), so a field with no new
		// value can never be made to match and must not force a reprocess.
		if values.new == "" {
			logger.Pl.D(2, "No new value for key %q, skipping comparison against video value %q.", key, values.existing)
			continue
		}

		if values.new != values.existing { // Maintain case sensitivity (avoid strings.EqualFold).
			logger.Pl.D(2, "======== Mismatched meta in file: %q ========\nMismatch in key %q:\nNew value: %q\nIn video as: %q. Will process video.",
				fd.MetaFilePath, key, values.new, values.existing)
			matches = false
		} else {
			logger.Pl.D(2, "Detected key %q as being the same.\nFFprobe: %q\nMetafile: %q", key, values.existing, values.new)
		}
	}

	// Print all captured metadata.
	if logging.Level() > 0 {
		printArray(ffContent)
	}
	return matches
}

// ffprobeBPPPFOutput holds info used to grab codecs and calculate BPPPF (compression level).
type ffprobeCodecOutput struct {
	Streams []ffprobeCodecOutputStream `json:"streams"`
	Format  struct {
		BitRate string `json:"bit_rate"`
	} `json:"format"`
}

// ffprobeCodecOutputStream holds the info grabbed for the stream.
type ffprobeCodecOutputStream struct {
	CodecType    string `json:"codec_type"`
	CodecName    string `json:"codec_name"`
	Width        int    `json:"width"`
	Height       int    `json:"height"`
	AvgFrameRate string `json:"avg_frame_rate"`
	BitRate      string `json:"bit_rate"`
	SampleRate   string `json:"sample_rate"`
	Channels     int    `json:"channels"`
	Disposition  struct {
		AttachedPic int `json:"attached_pic"`
	} `json:"disposition"`
}

// ProbeCodecs determines the video and audio codecs and compression levels, measuring the audio bitrate from its packets if asked and the file has none.
func ProbeCodecs(ctx context.Context, inputFile string, measureAudio bool) (audioCodec, videoCodec string, audioCompression, videoCompression sharedconsts.CompressionLevel, err error) {
	if inputFile == "" {
		return "", "", sharedconsts.CompressionBlank, sharedconsts.CompressionBlank, fmt.Errorf("input file sent in empty")
	}

	// FFprobe command to grab relevant values.
	cmd := exec.CommandContext(ctx, sharedconsts.ProgramFFprobe,
		"-v", "error",
		"-show_entries",
		"stream=codec_type,codec_name,width,height,avg_frame_rate,bit_rate,sample_rate,channels:stream_disposition=attached_pic:format=bit_rate",
		"-of", "json",
		inputFile,
	)

	// Run.
	raw, err := cmd.Output()
	if err != nil {
		return "", "", sharedconsts.CompressionBlank, sharedconsts.CompressionBlank, err
	}

	var vInfo ffprobeCodecOutput
	if err := json.Unmarshal(raw, &vInfo); err != nil {
		return "", "", sharedconsts.CompressionBlank, sharedconsts.CompressionBlank, err
	}

	// Find the first video and audio streams. A thumbnail isn't the video.
	var video *ffprobeCodecOutputStream
	var audio *ffprobeCodecOutputStream
	for i := range vInfo.Streams {
		stream := &vInfo.Streams[i]

		switch stream.CodecType {
		case "video":
			if video == nil && stream.Disposition.AttachedPic == 0 {
				video = stream
				videoCodec = strings.TrimSpace(video.CodecName)
				logger.Pl.I("Got video codec from '%s' as '%s'", inputFile, videoCodec)
			}
		case "audio":
			if audio == nil {
				audio = stream
				audioCodec = strings.TrimSpace(audio.CodecName)
				logger.Pl.I("Got audio codec from '%s' as '%s'", inputFile, audioCodec)
			}
		}
	}

	// MKV and WebM store no audio bitrate, so measure it when a codec line needs the level.
	var audioBitRate string
	if audio != nil {
		if audio.BitRate == "" && measureAudio {
			audio.BitRate = probeAudioBitrate(ctx, inputFile)
		}
		audioBitRate = audio.BitRate
	}

	// Determine video compression level, left unknown if it can't be worked out so the codecs still count.
	if video == nil {
		logger.Pl.I("No video stream in '%s', so only its audio can be transcoded", inputFile)
	} else if videoCompression, err = getVideoCompressionLevel(video, vInfo.Format.BitRate, audioBitRate, inputFile); err != nil {
		logger.Pl.W("Can't tell how compressed the video in %q is, so only codec lines without lc_, mc_ or hc_ match it: %v", inputFile, err)
		videoCompression = sharedconsts.CompressionBlank
	}

	// Determine audio compression level, likewise.
	if audio != nil {
		if audioCompression, err = getAudioCompressionLevel(audio, inputFile); err != nil {
			logger.Pl.W("Can't tell how compressed the audio in %q is, so only codec lines without lc_, mc_ or hc_ match it: %v", inputFile, err)
			audioCompression = sharedconsts.CompressionBlank
		}
	}

	return audioCodec, videoCodec, audioCompression, videoCompression, nil
}

// probeAudioBitrate works out the first audio stream's bitrate from its packets, or returns "" if it can't.
func probeAudioBitrate(ctx context.Context, inputFile string) string {
	logger.Pl.I("Measuring the audio bitrate of %q from its packets. This reads the whole file, so it can take a while for long videos.", inputFile)

	// List each audio packet's start, length and size, which FFprobe gives in that order.
	cmd := exec.CommandContext(ctx, sharedconsts.ProgramFFprobe,
		"-v", "error",
		"-select_streams", "a:0",
		"-show_entries", "packet=pts_time,duration_time,size",
		"-of", "csv=p=0",
		inputFile,
	)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		logger.Pl.W("Can't read the audio packets in %q: %v", inputFile, err)
		return ""
	}
	if err := cmd.Start(); err != nil {
		logger.Pl.W("Can't read the audio packets in %q: %v", inputFile, err)
		return ""
	}

	// Add up the sizes line by line, as a long video lists millions of packets.
	// MKV rounds each packet's length to the millisecond, so time them from the first start to the last end.
	var totalBytes float64
	start := math.Inf(1)
	end := math.Inf(-1)
	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		fields := strings.Split(scanner.Text(), ",")
		if len(fields) != 3 {
			continue
		}
		packetStart, startErr := strconv.ParseFloat(fields[0], 64)
		packetSeconds, durationErr := strconv.ParseFloat(fields[1], 64)
		packetBytes, sizeErr := strconv.ParseFloat(fields[2], 64)
		if startErr != nil || durationErr != nil || sizeErr != nil {
			continue
		}
		start = min(start, packetStart)
		end = max(end, packetStart+packetSeconds)
		totalBytes += packetBytes
	}
	if err := cmd.Wait(); err != nil {
		logger.Pl.W("Can't read the audio packets in %q: %v", inputFile, err)
		return ""
	}
	if end <= start {
		logger.Pl.W("The audio packets in %q have no length, so its bitrate is unknown", inputFile)
		return ""
	}

	bitRate := strconv.FormatFloat(totalBytes*8/(end-start), 'f', 0, 64)
	logger.Pl.I("Measured the audio bitrate of %q as %s bits per second", inputFile, bitRate)
	return bitRate
}

// ProbeFirstFrames returns the codec of a file's first stream of a type ("V", "a"), and how many of its frames read back in a trial copy's length.
func ProbeFirstFrames(ctx context.Context, path, stream string) (codec string, frames int) {
	out, err := exec.CommandContext(ctx, sharedconsts.ProgramFFprobe,
		"-v", "error",
		"-select_streams", stream+":0",
		"-read_intervals", "%+"+consts.CopyCheckSeconds,
		"-count_frames",
		"-show_entries", "stream=codec_name,nb_read_frames",
		"-of", "json",
		path,
	).Output()
	if err != nil {
		return "", 0
	}

	var probed struct {
		Streams []struct {
			CodecName  string `json:"codec_name"`
			ReadFrames string `json:"nb_read_frames"`
		} `json:"streams"`
	}
	if err := json.Unmarshal(out, &probed); err != nil || len(probed.Streams) == 0 {
		return "", 0
	}

	// A count FFprobe couldn't make isn't a number, and counts as none.
	frames, _ = strconv.Atoi(probed.Streams[0].ReadFrames)
	return probed.Streams[0].CodecName, frames
}

// ProbeVideoFrame returns a video's width, height and pixel format, blank if they can't be read.
func ProbeVideoFrame(ctx context.Context, inputFile string) (width, height int, pixFmt string) {
	out, err := exec.CommandContext(ctx, sharedconsts.ProgramFFprobe,
		"-v", "error",
		"-select_streams", "V:0",
		"-show_entries", "stream=width,height,pix_fmt",
		"-of", "json",
		inputFile,
	).Output()
	if err != nil {
		logger.Pl.W("Could not read the size of %q: %v", inputFile, err)
		return 0, 0, ""
	}

	var probe struct {
		Streams []struct {
			Width  int    `json:"width"`
			Height int    `json:"height"`
			PixFmt string `json:"pix_fmt"`
		} `json:"streams"`
	}
	if err := json.Unmarshal(out, &probe); err != nil || len(probe.Streams) == 0 {
		logger.Pl.W("Could not read the size of %q: %v", inputFile, err)
		return 0, 0, ""
	}
	return probe.Streams[0].Width, probe.Streams[0].Height, probe.Streams[0].PixFmt
}

// ProbeVideoSize returns a video's shorter side in pixels and its bitrate, the whole file's when the stream has none (as in MKV and WebM).
func ProbeVideoSize(ctx context.Context, inputFile string) (shorterSide int, bitrate int64) {
	out, err := exec.CommandContext(ctx, sharedconsts.ProgramFFprobe,
		"-v", "error",
		"-select_streams", "v:0",
		"-show_entries", "stream=width,height,bit_rate:format=bit_rate",
		"-of", "json",
		inputFile,
	).Output()
	if err != nil {
		logger.Pl.W("Could not read the size of %q, so the smallest -b:v resolution is used: %v", inputFile, err)
		return 0, 0
	}

	var probe struct {
		Streams []struct {
			Width   int    `json:"width"`
			Height  int    `json:"height"`
			BitRate string `json:"bit_rate"`
		} `json:"streams"`
		Format struct {
			BitRate string `json:"bit_rate"`
		} `json:"format"`
	}
	if err := json.Unmarshal(out, &probe); err != nil {
		logger.Pl.W("Could not read the size of %q, so the smallest -b:v resolution is used: %v", inputFile, err)
		return 0, 0
	}

	// The first video stream's size and bitrate, else the file's bitrate.
	if len(probe.Streams) > 0 {
		shorterSide = min(probe.Streams[0].Width, probe.Streams[0].Height)
		bitrate, _ = strconv.ParseInt(probe.Streams[0].BitRate, 10, 64)
	}
	if bitrate <= 0 {
		bitrate, _ = strconv.ParseInt(probe.Format.BitRate, 10, 64)
	}
	return shorterSide, bitrate
}

// ProbeStreamCodecs returns the codec names of a file's streams of a type ("s", "t"), in order, blank if unknown.
func ProbeStreamCodecs(ctx context.Context, inputFile, streamType string) []string {
	cmd := exec.CommandContext(ctx, sharedconsts.ProgramFFprobe,
		"-v", "error",
		"-select_streams", streamType,
		"-show_entries", "stream=codec_name",
		"-of", "json",
		inputFile,
	)
	raw, err := cmd.Output()
	if err != nil {
		logger.Pl.E("Failed to check the %q streams of %q: %v", streamType, inputFile, err)
		return nil
	}

	var probed ffprobeCodecOutput
	if err := json.Unmarshal(raw, &probed); err != nil {
		logger.Pl.E("Failed to read the %q streams of %q: %v", streamType, inputFile, err)
		return nil
	}
	codecs := make([]string, 0, len(probed.Streams))
	for _, stream := range probed.Streams {
		codecs = append(codecs, stream.CodecName)
	}
	return codecs
}

// thumbnailStream is a thumbnail embedded in a video.
type thumbnailStream struct {
	Index       int    `json:"index"`
	CodecName   string `json:"codec_name"`
	Disposition struct {
		AttachedPic int `json:"attached_pic"`
	} `json:"disposition"`
	Tags struct {
		Filename string `json:"filename"`
		Mimetype string `json:"mimetype"`
	} `json:"tags"`
}

// ProbeThumbnailStreams returns a file's embedded thumbnails.
func ProbeThumbnailStreams(ctx context.Context, inputFile string) []thumbnailStream {
	cmd := exec.CommandContext(ctx, sharedconsts.ProgramFFprobe,
		"-v", "error",
		"-select_streams", "v",
		"-show_entries", "stream=index,codec_name:stream_disposition=attached_pic:stream_tags=filename,mimetype",
		"-of", "json",
		inputFile,
	)
	raw, err := cmd.Output()
	if err != nil {
		logger.Pl.E("Failed to check %q for thumbnails: %v", inputFile, err)
		return nil
	}

	var probed struct {
		Streams []thumbnailStream `json:"streams"`
	}
	if err := json.Unmarshal(raw, &probed); err != nil {
		logger.Pl.E("Failed to read the thumbnail check of %q: %v", inputFile, err)
		return nil
	}
	var thumbnails []thumbnailStream
	for _, stream := range probed.Streams {
		if stream.Disposition.AttachedPic == 1 {
			thumbnails = append(thumbnails, stream)
		}
	}
	return thumbnails
}

// ProbeDurationUS returns a file's duration in microseconds, or 0 if FFprobe can't read it.
func ProbeDurationUS(inputFile string) int64 {
	out, err := exec.Command(sharedconsts.ProgramFFprobe,
		"-v", "error",
		"-show_entries", "format=duration",
		"-of", "default=noprint_wrappers=1:nokey=1",
		inputFile,
	).Output()
	if err != nil {
		logger.Pl.D(1, "Could not read the duration of %q, progress will have no percentage: %v", inputFile, err)
		return 0
	}
	secs, err := strconv.ParseFloat(strings.TrimSpace(string(out)), 64)
	if err != nil || secs <= 0 {
		return 0
	}
	return int64(secs * 1e6)
}
