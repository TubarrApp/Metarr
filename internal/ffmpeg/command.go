package ffmpeg

import (
	"cmp"
	"context"
	"fmt"
	"maps"
	"metarr/internal/abstractions"
	"metarr/internal/domain/consts"
	"metarr/internal/domain/keys"
	"metarr/internal/domain/logger"
	"metarr/internal/domain/vars"
	"metarr/internal/ffprobe"
	"metarr/internal/file"
	"metarr/internal/models"
	"metarr/internal/parsing"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"github.com/TubarrApp/gocommon/sharedconsts"
	"github.com/TubarrApp/gocommon/sharedmodels"
	"github.com/TubarrApp/gocommon/sharedparsing"
	"github.com/TubarrApp/gocommon/sharedvalidation"
)

// availableCodecsCache caches the codecs in FFmpeg to avoid repeated calls.
var (
	availableCodecsCache     string
	availableCodecsCacheOnce sync.Once
)

// availableEncoders returns FFmpeg's encoder list, read once.
func availableEncoders(ctx context.Context) string {
	availableCodecsCacheOnce.Do(func() {
		availableCodecsCache = ffmpegAvailableCodecs(ctx)
	})
	return availableCodecsCache
}

// ffCommandBuilder handles FFmpeg command construction.
type ffCommandBuilder struct {
	// Files
	inputFile  string
	outputFile string

	// Maps
	formatFlagsMap map[string]string
	metadataMap    map[string]string

	// HW accel
	gpuAccelFlags []string
	gpuNode       []string

	// Video codecs
	videoCodecGPU      []string
	videoCodecSoftware []string

	// Video encoder
	encoderArgs []string
	videoFilter string
	passes      int

	// Audio codec
	audioCodec []string
	audioRate  []string

	// Streams to keep
	streamMapping []string

	// Thumbnail
	thumbnailInput []string
	thumbnail      []string
	thumbnailCount int
	tempThumbnails []string // Thumbnail files made for this video, removed once it's done.

	// Other parameters
	qualityParameter []string

	// Builder
	builder *strings.Builder
}

// newFfCommandBuilder creates a new FFmpeg command builder.
func newFfCommandBuilder(fd *models.FileData, outputFile string) (*ffCommandBuilder, error) {
	absOutputFile, err := filepath.Abs(outputFile)
	if err != nil {
		return nil, err
	}

	return &ffCommandBuilder{
		builder:     &strings.Builder{},
		inputFile:   fd.OriginalVideoPath,
		outputFile:  absOutputFile,
		metadataMap: make(map[string]string),
	}, nil
}

// buildCommand constructs the complete FFmpeg command.
func (b *ffCommandBuilder) buildCommand(ctx context.Context, fd *models.FileData, currentVCodec, currentACodec sharedmodels.FromCodec, desiredVCodec, desiredACodec, outExt string, reencodeV, reencodeA bool) ([]string, error) {
	if b.inputFile == "" || b.outputFile == "" {
		return nil, fmt.Errorf("input file or output file is empty.\n\nInput file: %v\nOutput file: %v", b.inputFile, b.outputFile)
	}
	availableCodecs := availableEncoders(ctx)

	// A video encoded again has no codec to copy, not even its own.
	copyVCodec := currentVCodec.Codec
	if reencodeV {
		copyVCodec = ""
	}

	// Audio encoded again likewise.
	copyACodec := currentACodec.Codec
	if reencodeA {
		copyACodec = ""
	}

	// Get GPU flags/codecs.
	accelType, useHWDecode := b.setHWAccelFlags()
	b.setGPUAccelerationCodec(accelType, copyVCodec, desiredVCodec, availableCodecs)

	logger.Pl.D(1, "Transcoding to codec %q from current codec %q", desiredVCodec, currentVCodec)

	// Get software codecs.
	if b.videoCodecGPU == nil {
		if err := b.setVideoSoftwareCodec(copyVCodec, desiredVCodec, availableCodecs); err != nil {
			return nil, err
		}
	}
	if err := b.setAudioCodec(copyACodec, desiredACodec, availableCodecs); err != nil {
		return nil, err
	}
	b.setAudioEncoderArgs(desiredACodec)
	if err := b.setVideoEncoderArgs(ctx, accelType, desiredVCodec); err != nil {
		return nil, err
	}

	b.setDefaultFormatFlagMap(outExt)
	args := b.setFormatFlags()
	b.addAllMetadata(fd)

	stripThumbnails := false
	if abstractions.IsSet(keys.StripThumbnails) {
		stripThumbnails = abstractions.GetBool(keys.StripThumbnails)
	}
	// Start afresh, as the builder is reused when FFmpeg is retried.
	b.thumbnailInput = nil
	b.thumbnail = nil
	b.thumbnailCount = 0
	if !stripThumbnails {
		b.setThumbnail(ctx, fd.EmbedThumbnail(), parsing.GetBaseNameWithoutExt(fd.OriginalVideoPath), outExt, fd.HasEmbeddedThumbnail)
	}

	// The video's own thumbnails stay unless stripped or replaced. Copying only keeps them as thumbnails within a container other than Matroska.
	sameContainer := strings.EqualFold(filepath.Ext(b.inputFile), outExt)
	if !stripThumbnails && len(b.thumbnail) == 0 && (!sameContainer || strings.EqualFold(outExt, sharedconsts.ExtMKV)) {
		if err := b.keepThumbnails(ctx, outExt); err != nil {
			return nil, err
		}
	}

	// Set the streams to keep.
	b.setStreamMapping(ctx, outExt, !stripThumbnails && len(b.thumbnail) == 0)

	// Return the fully appended argument string.
	return b.buildFinalCommand(args, useHWDecode)
}

// setStreamMapping keeps the streams the output container can hold, as FFmpeg otherwise keeps only one of each type.
func (b *ffCommandBuilder) setStreamMapping(ctx context.Context, outExt string, keepThumbnails bool) {
	inExt := strings.ToLower(filepath.Ext(b.inputFile))
	outExt = strings.ToLower(outExt)

	// Video ("v" includes thumbnails, "V" doesn't) and audio. Thumbnails are only copied within a container.
	video := "0:V?"
	if keepThumbnails && inExt == outExt {
		video = "0:v?"
	}
	b.streamMapping = []string{"-map", video, "-map", "0:a?"}

	// Subtitles.
	b.streamMapping = append(b.streamMapping, b.subtitleMaps(ctx, inExt, outExt)...)

	// Attachments (e.g. subtitle fonts), which only Matroska holds.
	if outExt == sharedconsts.ExtMKV {
		b.streamMapping = append(b.streamMapping, "-map", "0:t?")
	}

	// Data streams aren't mapped, as copying some fails (e.g. timecodes, which MP4 makes again itself).

	// Copy anything no later option sets a codec for (e.g. a second video stream).
	b.streamMapping = append(b.streamMapping, "-c", "copy")

	// Convert copied video the output container can't take as it is.
	b.streamMapping = append(b.streamMapping, b.bitstreamFilters(ctx, outExt)...)
}

// bitstreamFilters returns the filters copied H.264 and HEVC streams need to go in a file of extension outExt.
func (b *ffCommandBuilder) bitstreamFilters(ctx context.Context, outExt string) []string {
	switch outExt {
	case sharedconsts.ExtAVI, sharedconsts.ExtMPEG, sharedconsts.ExtMPG, sharedconsts.ExtVOB:
	default:
		return nil
	}

	// Only the first video stream can be encoded rather than copied.
	encoding := b.videoCodecGPU != nil || (len(b.videoCodecSoftware) >= 2 && b.videoCodecSoftware[1] != consts.FFVCodecKeyCopy)

	var filters []string
	for i, codec := range ffprobe.ProbeStreamCodecs(ctx, b.inputFile, "V") {
		if i == 0 && encoding {
			continue
		}
		switch codec {
		case sharedconsts.VCodecH264:
			filters = append(filters, "-bsf:v:"+strconv.Itoa(i), h264AnnexBFilter(b.inputFile))
		case sharedconsts.VCodecHEVC:
			filters = append(filters, "-bsf:v:"+strconv.Itoa(i), "hevc_mp4toannexb")
		}
	}
	return filters
}

// h264AnnexBFilter returns the filter for H.264 copied from a file. ASF keeps the stream's headers apart, and without them
// repeated in the stream an MPEG file loses its video. Elsewhere that can leave a file nothing can open, so it's only for ASF.
func h264AnnexBFilter(inputFile string) string {
	switch strings.ToLower(filepath.Ext(inputFile)) {
	case sharedconsts.ExtASF, sharedconsts.ExtWMV:
		return "h264_mp4toannexb,dump_extra"
	}
	return "h264_mp4toannexb"
}

// subtitleMaps returns the maps for the subtitle streams a file of extension outExt can hold, from one of inExt.
func (b *ffCommandBuilder) subtitleMaps(ctx context.Context, inExt, outExt string) []string {
	// A container holds its own, and Matroska holds any.
	if inExt == outExt || outExt == sharedconsts.ExtMKV {
		return []string{"-map", "0:s?"}
	}

	subtitles := ffprobe.ProbeStreamCodecs(ctx, b.inputFile, "s")
	if subtitleEncoderFor(inExt, outExt) == "" {
		if len(subtitles) > 0 {
			logger.Pl.W("Leaving out the subtitles of %q, as %s files can't hold them", b.inputFile, outExt)
		}
		return nil
	}

	// Text converts to what the container holds, images can't.
	var subtitleMaps []string
	for i, codec := range subtitles {
		if _, bitmap := bitmapSubtitleCodecs[codec]; bitmap {
			logger.Pl.W("Leaving out subtitle stream %d of %q, as %s subtitles can't be converted for a %s file", i, b.inputFile, codec, outExt)
			continue
		}
		subtitleMaps = append(subtitleMaps, "-map", "0:s:"+strconv.Itoa(i))
	}
	return subtitleMaps
}

// setThumbnail sets the thumbnail image in the video metadata.
func (b *ffCommandBuilder) setThumbnail(ctx context.Context, thumbnailURL, videoBaseName, outExt string, hasEmbeddedThumbnail bool) {
	if thumbnailURL == "" {
		if hasEmbeddedThumbnail {
			logger.Pl.I("Video %q has an embedded thumbnail. Will copy existing attached_pic.", b.inputFile)
		}
		return
	}

	// Thumbnail URL not "" beyond here...

	// Cut to leave room in the thumbnail's file name.
	videoBaseName = sharedparsing.SanitizePathComponent(videoBaseName, 150)

	// A file given with --thumbnail is used where it is (its JPG copy goes in the temp dir), anything else is downloaded.
	thumbnail, jpgPath := thumbnailURL, filepath.Join(os.TempDir(), "thumb_"+videoBaseName+".jpg")
	if thumbnailURL != abstractions.GetString(keys.Thumbnail) || parsing.IsHTTPURL(thumbnailURL) {
		var err error
		if thumbnail, err = file.DownloadThumbnail(ctx, thumbnailURL, videoBaseName); err != nil {
			logger.Pl.E("Could not download thumbnail %q: %v", thumbnailURL, err)
			return
		}
		b.tempThumbnails = append(b.tempThumbnails, thumbnail)
		jpgPath = parsing.GetFilepathWithoutExt(thumbnail) + "_converted.jpg"
	}

	// Ensure JPG, going by what the file holds, as its name may not say.
	var err error
	if codecs := ffprobe.ProbeStreamCodecs(ctx, thumbnail, "v"); len(codecs) != 1 || codecs[0] != jpgCodec {
		if thumbnail, err = convertToJPG(thumbnail, jpgPath); err != nil {
			logger.Pl.E("Could not convert thumbnail %q to JPG: %v", thumbnail, err)
			return
		}
		b.tempThumbnails = append(b.tempThumbnails, jpgPath)
	}

	b.embedThumbnail(ctx, thumbnail, outExt, thumbnailFilename, thumbnailMimetype)
}

// keepThumbnails embeds the video's own thumbnails in the new video, for where copying them wouldn't keep them as thumbnails.
func (b *ffCommandBuilder) keepThumbnails(ctx context.Context, outExt string) error {
	// Find the thumbnails, if there are any.
	thumbnails := ffprobe.ProbeThumbnailStreams(ctx, b.inputFile)
	if len(thumbnails) == 0 {
		return nil
	}
	if !parsing.HoldsThumbnail(outExt) {
		logger.Pl.W("Leaving out the thumbnail of %q, as %s files can't hold one", b.inputFile, outExt)
		return nil
	}

	for _, stream := range thumbnails {
		tmp, err := os.CreateTemp("", "thumb_*.jpg")
		if err != nil {
			return fmt.Errorf("failed to make a temporary file for the thumbnail of %q: %w", b.inputFile, err)
		}
		b.tempThumbnails = append(b.tempThumbnails, tmp.Name())
		if err := tmp.Close(); err != nil {
			return fmt.Errorf("failed to close temporary thumbnail %q: %w", tmp.Name(), err)
		}

		// Take it out unchanged if it's a JPG, or Matroska can attach it as it is, else as a JPG.
		args := []string{"-y", "-v", "error", "-i", b.inputFile, "-map", "0:" + strconv.Itoa(stream.Index), "-frames:v", "1", "-update", "1", "-f", "image2"}
		if stream.CodecName == jpgCodec || (strings.EqualFold(outExt, sharedconsts.ExtMKV) && stream.Tags.Mimetype != "") {
			args = append(args, "-c", "copy")
		} else {
			args = append(args, jpgQualityArgs...)
		}
		if out, err := exec.CommandContext(ctx, sharedconsts.ProgramFFmpeg, append(args, tmp.Name())...).CombinedOutput(); err != nil {
			return fmt.Errorf("failed to take out the thumbnail of %q: %w\n%s", b.inputFile, err, out)
		}

		b.embedThumbnail(ctx, tmp.Name(), outExt, cmp.Or(stream.Tags.Filename, thumbnailFilename), cmp.Or(stream.Tags.Mimetype, thumbnailMimetype))
	}
	return nil
}

// embedThumbnail adds the arguments embedding an image file as a thumbnail of the output, if its container can hold one.
func (b *ffCommandBuilder) embedThumbnail(ctx context.Context, thumbnail, outExt, filename, mimetype string) {
	ext := strings.ToLower(outExt)
	switch ext {
	case sharedconsts.ExtMP4,
		sharedconsts.ExtM4V:

		// The thumbnail comes after the video's own video streams, taken to be one if they can't be read, and any thumbnails before it.
		videoStreams := ffprobe.ProbeStreamCodecs(ctx, b.inputFile, "V")
		videoCount := len(videoStreams)
		if videoStreams == nil {
			videoCount = 1
		}
		index := strconv.Itoa(videoCount + b.thumbnailCount)

		b.thumbnailInput = append(b.thumbnailInput, "-i", thumbnail) // add the thumbnail as another input.
		b.thumbnail = append(b.thumbnail,
			"-map", strconv.Itoa(b.thumbnailCount+1), // map new thumbnail, copied as the stream mapping sets.
			"-disposition:v:"+index, "attached_pic", // mark as a thumbnail.
		)
		b.thumbnailCount++

	case sharedconsts.ExtMKV:
		// The thumbnail is attached after any attachments the video has, which keep their own mimetypes.
		index := strconv.Itoa(len(ffprobe.ProbeStreamCodecs(ctx, b.inputFile, "t")) + b.thumbnailCount)
		b.thumbnail = append(b.thumbnail,
			"-attach", thumbnail,
			"-metadata:s:t:"+index, "mimetype="+mimetype,
			"-metadata:s:t:"+index, "filename="+filename,
		)
		b.thumbnailCount++

	default:
		logger.Pl.D(1, "Thumbnail embedding not supported for extension: %s", ext)
	}
}

// removeTempThumbnails removes the thumbnail files made for this video.
func (b *ffCommandBuilder) removeTempThumbnails() {
	for _, path := range b.tempThumbnails {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			logger.Pl.W("Failed to remove temporary thumbnail %q: %v", path, err)
		}
	}
}

// convertToJPG converts an image to a JPG at outputPath for embedding.
func convertToJPG(inputPath, outputPath string) (string, error) {
	args := append([]string{"-y", "-i", inputPath}, jpgQualityArgs...)
	cmd := exec.Command(sharedconsts.ProgramFFmpeg, append(args, outputPath)...)
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("failed to convert %q to jpg: %w", filepath.Ext(inputPath), err)
	}
	return outputPath, nil
}

// setAudioCodec gets the audio codec for transcode operations.
func (b *ffCommandBuilder) setAudioCodec(currentACodec, desiredACodec, availableCodecs string) error {
	// AAC staying AAC would normally be copied, but on this route copying leaves it with the wrong timing, so it's encoded again.
	stayingAAC := currentACodec == sharedconsts.ACodecAAC && (desiredACodec == "" || desiredACodec == sharedconsts.ACodecAAC)
	if stayingAAC && aacCopyBreaksInAVI(b.inputFile, filepath.Ext(b.outputFile)) {
		b.audioCodec = []string{consts.FFmpegCA, consts.FFACodecKeyAAC}
		return nil
	}

	switch desiredACodec {
	case sharedconsts.ACodecCopy, currentACodec, "": // -- Set and return early. --
		b.audioCodec = []string{consts.FFmpegCA, sharedconsts.ACodecCopy} // Hardcoded copy (do not use 'desiredACodec').

		// FFmpeg counts TrueHD in MP4 as experimental, even copied.
		if currentACodec == sharedconsts.ACodecTrueHD {
			b.audioCodec = append(b.audioCodec, consts.FFmpegStrictExperimental...)
		}
		return nil

	case sharedconsts.ACodecAAC, // -- No audio rate needed. --
		sharedconsts.ACodecALAC,
		sharedconsts.ACodecFLAC,
		sharedconsts.ACodecMP2,
		sharedconsts.ACodecMP3:

		b.audioCodec = []string{consts.FFmpegCA, desiredACodec}

	case sharedconsts.ACodecOpus:
		b.audioCodec = []string{consts.FFmpegCA, consts.FFACodecKeyOpus}

	case sharedconsts.ACodecVorbis:
		b.audioCodec = []string{consts.FFmpegCA, consts.FFACodecKeyVorbis}

	// TrueHD's only encoder is experimental, which FFmpeg refuses without -strict experimental.
	case sharedconsts.ACodecTrueHD:
		logger.Pl.W("FFmpeg's TrueHD encoder is experimental, its output may not play everywhere.")
		b.audioCodec = []string{consts.FFmpegCA, desiredACodec}
		b.audioCodec = append(b.audioCodec, consts.FFmpegStrictExperimental...)

	case sharedconsts.ACodecAC3, // -- 48KHz audio rate required. --
		sharedconsts.ACodecEAC3:
		b.audioCodec = []string{consts.FFmpegCA, desiredACodec}
		b.audioRate = []string{consts.FFmpegAR, consts.AudioRate48khz}

	// DTS's only encoder is experimental, which FFmpeg refuses without -strict experimental.
	case sharedconsts.ACodecDTS:
		logger.Pl.W("FFmpeg's DTS encoder is experimental, its output may not play everywhere.")
		b.audioCodec = []string{consts.FFmpegCA, desiredACodec}
		b.audioCodec = append(b.audioCodec, consts.FFmpegStrictExperimental...)
		b.audioRate = []string{consts.FFmpegAR, consts.AudioRate48khz}

	case sharedconsts.ACodecPCM, sharedconsts.ACodecWAV: // "pcm" isn't an encoder name.
		b.audioCodec = []string{consts.FFmpegCA, consts.FFACodecKeyPCM}

	default:
		b.audioCodec = nil // -- Invalid or un-set codec. --
	}

	// Check codec availability. A codec chosen but not installed fails, rather than leaving the audio as it was.
	if len(b.audioCodec) >= 2 {
		if availableCodecs != "" && !strings.Contains(availableCodecs, b.audioCodec[1]) {
			return fmt.Errorf("can't transcode audio to %q: FFmpeg's %q encoder isn't installed", desiredACodec, b.audioCodec[1])
		}
	} else if b.audioCodec != nil {
		logger.Pl.E("%s Strings expected to be at least parts, got %v", sharedconsts.ErrorfDevError, b.audioCodec)
		b.audioCodec = nil
	}
	return nil
}

// setAudioEncoderArgs adds the target codec's encoder args to audio being encoded.
func (b *ffCommandBuilder) setAudioEncoderArgs(desiredACodec string) {
	// Copied audio has no encoder to set.
	if len(b.audioCodec) < 2 || b.audioCodec[1] == sharedconsts.ACodecCopy {
		return
	}
	args := audioEncoderArgs(desiredACodec)

	// Run the codec's filter, then the layout filter Opus needs.
	filter := args.Filter
	if b.audioCodec[1] == consts.FFACodecKeyOpus {
		if filter != "" {
			filter += ","
		}
		filter += consts.OpusLayoutFilter
	}
	if filter != "" {
		b.audioCodec = append(b.audioCodec, consts.FFmpegAF, filter)
	}

	// A sample rate asked replaces the one AC-3, E-AC-3 and DTS get.
	if args.SampleRate != "" {
		b.audioRate = []string{consts.FFmpegAR, args.SampleRate}
	}

	// Then the channels and the other args.
	if args.Channels != "" {
		b.audioCodec = append(b.audioCodec, consts.FFmpegAC, args.Channels)
	}
	b.audioCodec = append(b.audioCodec, args.Args...)
}

// setVideoSoftwareCodec gets the audio codec for transcode operations.
func (b *ffCommandBuilder) setVideoSoftwareCodec(currentVCodec, desiredVCodec, availableCodecs string) error {
	switch desiredVCodec {
	case sharedconsts.VCodecCopy, currentVCodec, "": // -- Set and return early. --
		b.videoCodecSoftware = []string{consts.FFmpegCV0, consts.FFVCodecKeyCopy} // Hardcoded copy (do not use 'desiredVCodec').
		return nil

	// Software codecs.
	case sharedconsts.VCodecAV1, // -- Software codecs. --
		sharedconsts.VCodecH264,
		sharedconsts.VCodecHEVC,
		sharedconsts.VCodecMPEG2,
		sharedconsts.VCodecVP8,
		sharedconsts.VCodecVP9:
		b.videoCodecSoftware = []string{consts.FFmpegCV0, consts.VCodecToFFVCodec[desiredVCodec]}

	default:
		b.videoCodecSoftware = nil // -- Invalid or un-set codec. --
	}

	// Check codec availability. A codec chosen but not installed fails, rather than leaving the video as it was.
	if len(b.videoCodecSoftware) >= 2 {
		if availableCodecs != "" && !strings.Contains(availableCodecs, b.videoCodecSoftware[1]) {
			return fmt.Errorf("can't transcode video to %q: FFmpeg's %q encoder isn't installed", desiredVCodec, b.videoCodecSoftware[1])
		}
	} else if b.videoCodecSoftware != nil {
		logger.Pl.E("%s Strings expected to be at least 2 parts, got %v", sharedconsts.ErrorfDevError, b.videoCodecSoftware)
		b.videoCodecSoftware = nil
	}
	return nil
}

// ffmpegAvailableCodecs lists codecs available in FFmpeg.
func ffmpegAvailableCodecs(ctx context.Context) (output string) {
	cmd := exec.CommandContext(ctx, sharedconsts.ProgramFFmpeg, "-encoders")
	outBytes, err := cmd.Output()
	result := strings.TrimSpace(string(outBytes))
	if err != nil {
		logger.Pl.E("Codec Grab Failed: %v", err)
		return ""
	}
	return result
}

// setGPUAccelerationCodec sets the codec to use for the GPU acceleration (separated from setGPUAcceleration for ordering reasons).
func (b *ffCommandBuilder) setGPUAccelerationCodec(accelType, currentVCodec, useTranscodeCodec, availableCodecs string) {
	if accelType == "" || accelType == sharedconsts.AccelTypeAuto {
		logger.Pl.D(2, "Using 'auto' HW acceleration, will use a standard software codec (e.g. 'libx264')")
		return
	}

	// Copy a video already in the desired codec, as the software path does.
	switch useTranscodeCodec {
	case sharedconsts.VCodecCopy, currentVCodec, "":
		return
	}

	// Build codec string '<codec>_<accelerator>'
	gpuCodecString := useTranscodeCodec + "_"
	if accelType == sharedconsts.AccelTypeCuda {
		gpuCodecString += consts.AccelFlagNvenc
	} else {
		gpuCodecString += accelType
	}
	b.videoCodecGPU = []string{consts.FFmpegCV0, gpuCodecString}

	// Check codec availability and set nil if not available.
	if !strings.Contains(availableCodecs, gpuCodecString) {
		logger.Pl.W("GPU-bound video codec %q not available in FFmpeg build, falling back to software.", gpuCodecString)
		b.videoCodecGPU = nil
	}

	// Log GPU args.
	if b.gpuAccelFlags != nil && b.videoCodecGPU != nil {
		logCommand := append(b.gpuAccelFlags, b.videoCodecGPU...)
		logger.Pl.I("Using hardware acceleration:\n\nType: %s\nCodec: %s\nArguments: %v\n", accelType, useTranscodeCodec, logCommand)
	}
}

// setHWAccelFlags checks and returns the flags for HW acceleration.
func (b *ffCommandBuilder) setHWAccelFlags() (accelType string, useHWDecode bool) {
	if !abstractions.IsSet(keys.TranscodeGPU) {
		return "", false
	}

	accelType = strings.ToLower(abstractions.GetString(keys.TranscodeGPU))
	if accelType == "" {
		logger.Pl.I("HW acceleration flags disabled, using software encode/decode")
		return "", false
	}

	// Get GPU device node if set.
	var gpuNode string
	if abstractions.IsSet(keys.TranscodeGPUNode) {
		gpuNode = abstractions.GetString(keys.TranscodeGPUNode)
	}

	// Add device nodes for encode-only modes.
	switch accelType {
	case sharedconsts.AccelTypeVAAPI:
		// VAAPI requires device node on Linux for encode-only.
		if vars.OS != sharedconsts.OSLinux {
			logger.Pl.W("VAAPI acceleration is only available on Linux.")
			return "", false
		}
		if gpuNode == "" {
			logger.Pl.W("VAAPI requires a device directory on Linux. Falling back to software...")
			return "", false
		}
		b.gpuNode = []string{consts.FFmpegDeviceVAAPI, gpuNode}

	case sharedconsts.AccelTypeAMF:
		// AMF is Windows-only.
		if vars.OS != sharedconsts.OSWindows {
			logger.Pl.W("AMF acceleration is only available on Windows.")
			return "", false
		}
	}

	// Only auto gets hardware decode (-hwaccel flags).
	if accelType == sharedconsts.AccelTypeAuto {
		b.gpuAccelFlags = []string{consts.FFmpegHWAccel, accelType}
		return sharedconsts.AccelTypeAuto, true
	}

	return accelType, false
}

// setVideoEncoderArgs sets the target codec's encoder args, converting -crf, -pix_fmt and the video filter for the encoder that runs.
func (b *ffCommandBuilder) setVideoEncoderArgs(ctx context.Context, accelType, desiredVCodec string) error {
	// Start afresh, as the builder is reused when FFmpeg is retried.
	b.qualityParameter = nil
	b.encoderArgs = nil
	b.videoFilter = ""
	b.passes = 1
	onGPU := len(b.videoCodecGPU) >= 2

	// Copied video has no encoder to set.
	if !onGPU && (len(b.videoCodecSoftware) < 2 || b.videoCodecSoftware[1] == consts.FFVCodecKeyCopy) {
		return nil
	}

	// Get the target codec's args.
	encoderArgs := videoEncoderArgs(desiredVCodec)
	b.videoFilter = encoderArgs.VideoFilter

	// The bitrate for the video's resolution, never above its own.
	var bitrateArgs []string
	if len(encoderArgs.Bitrates) > 0 {
		bitrateArgs = []string{consts.FFmpegBitrate, strconv.FormatInt(videoBitrate(ctx, b.inputFile, encoderArgs.Bitrates), 10)}
	}

	// Software encoders take the args as written.
	if !onGPU {
		if err := sharedvalidation.ValidateSoftwareEncoderArgs(desiredVCodec, encoderArgs); err != nil {
			return err
		}
		if encoderArgs.Quality != "" {
			b.qualityParameter = []string{consts.FFmpegCRF, encoderArgs.Quality}
		}
		if encoderArgs.PixFmt != "" {
			pixFmt, err := encoderPixFmt(ctx, b.videoCodecSoftware[1], encoderArgs.PixFmt)
			if err != nil {
				return err
			}
			b.encoderArgs = append(b.encoderArgs, consts.FFmpegPixFmt, pixFmt)
		}
		b.encoderArgs = append(b.encoderArgs, bitrateArgs...)
		b.encoderArgs = append(b.encoderArgs, encoderArgs.Args...)
		b.passes = max(encoderArgs.Passes, 1)
		return b.setEvenSize(ctx, encoderArgs.PixFmt)
	}
	encoder := b.videoCodecGPU[1]

	// GPU encoders ignore -pass, so more passes would only take longer.
	if encoderArgs.Passes > 1 {
		logger.Pl.W("Encoder %q makes one pass, so -pass %d is left out.", encoder, encoderArgs.Passes)
	}

	// GPU encoders take quality up to 51.
	quality := encoderArgs.Quality
	if q, err := strconv.Atoi(quality); err == nil && q > 51 {
		logger.Pl.W("Encoder %q takes quality up to 51, using 51 instead of %d.", encoder, q)
		quality = "51"
	}

	// Convert -crf to the GPU encoder's quality option.
	if quality != "" {
		switch accelType {
		case sharedconsts.AccelTypeAMF:
			b.qualityParameter = []string{"-qp_p", quality}
		case sharedconsts.AccelTypeCuda:
			b.qualityParameter = []string{"-rc", "vbr", "-cq", quality}
		case sharedconsts.AccelTypeQSV:
			b.qualityParameter = []string{"-global_quality", quality}
		case sharedconsts.AccelTypeVAAPI:
			b.qualityParameter = []string{"-qp", quality}
		default:
			logger.Pl.W("Metarr can't convert -crf for encoder %q, so it's left out.", encoder)
		}
		if b.qualityParameter != nil {
			logger.Pl.I("Sending -crf %s to encoder %q as %v.", encoderArgs.Quality, encoder, b.qualityParameter)
		}
	}

	// VAAPI encodes frames uploaded to the GPU, other GPU encoders take a pixel format.
	switch {
	case accelType == sharedconsts.AccelTypeVAAPI:
		uploadPixFmt := consts.VAAPIUploadPixFmt
		if encoderArgs.PixFmt != "" {
			var ok bool
			if uploadPixFmt, ok = consts.VAAPIUploadFormats[encoderArgs.PixFmt]; !ok {
				return fmt.Errorf("VAAPI encoder %q can't encode pixel format %q, use yuv420p or yuv420p10le", encoder, encoderArgs.PixFmt)
			}
		}
		b.videoFilter = fmt.Sprintf(consts.VAAPIUploadFilter, uploadPixFmt)
		if encoderArgs.VideoFilter != "" {
			b.videoFilter = encoderArgs.VideoFilter + "," + b.videoFilter
		}
	case encoderArgs.PixFmt != "":
		pixFmt, err := encoderPixFmt(ctx, encoder, encoderArgs.PixFmt)
		if err != nil {
			return err
		}
		b.encoderArgs = append(b.encoderArgs, consts.FFmpegPixFmt, pixFmt)
	}
	b.encoderArgs = append(b.encoderArgs, bitrateArgs...)
	b.encoderArgs = append(b.encoderArgs, encoderArgs.Args...)
	return nil
}

// setEvenSize cuts the last row or column off an odd-sized video for the software encoders that refuse one, or returns an
// error if the video is so small that would spoil it.
func (b *ffCommandBuilder) setEvenSize(ctx context.Context, askedPixFmt string) error {
	encoder := b.videoCodecSoftware[1]
	if encoder != consts.FFVCodecKeyH264 && encoder != consts.FFVCodecKeyH265 {
		return nil
	}

	// Only odd sizes matter, in formats with less colour detail than picture detail (4:4:4 and RGB can be any size).
	width, height, sourcePixFmt := ffprobe.ProbeVideoFrame(ctx, b.inputFile)
	if width%2 == 0 && height%2 == 0 {
		return nil
	}
	pixFmt := askedPixFmt
	if pixFmt == "" {
		pixFmt = sourcePixFmt
	}
	if strings.Contains(pixFmt, "444") || strings.HasPrefix(pixFmt, "gbr") {
		return nil
	}

	// Cut the row or column, unless that's too much of the picture.
	if min(width, height) < consts.MinSideToCut {
		return fmt.Errorf("encoder %q can't take the odd size %dx%d, and cutting a row or column from a video this small would spoil it", encoder, width, height)
	}
	logger.Pl.I("Encoder %q can't take the odd size %dx%d, so cutting its last row or column.", encoder, width, height)
	if b.videoFilter != "" {
		b.videoFilter += ","
	}
	b.videoFilter += consts.EvenSizeFilter
	return nil
}

// setDefaultFormatFlagMap adds commands specific for the extension input and output.
func (b *ffCommandBuilder) setDefaultFormatFlagMap(outExt string) {
	inExt := strings.ToLower(filepath.Ext(b.inputFile))
	outExt = strings.ToLower(outExt)

	b.formatFlagsMap = copyPreset
	if outExt == "" || inExt == outExt {
		return
	}

	logger.Pl.D(2, "Making default format map for input extension: %q, output extension: %q. (File: %q)",
		inExt, outExt, b.inputFile)

	// Convert subtitles for a container that can't take them as they are.
	if encoder := subtitleEncoderFor(inExt, outExt); encoder != "" {
		b.formatFlagsMap = make(map[string]string, len(copyPreset)+1)
		maps.Copy(b.formatFlagsMap, copyPreset)
		b.formatFlagsMap[consts.FFmpegCS] = encoder
	}
}

// setFormatFlags sets flags for the transcoding format, e.g. codec, etc.
func (b *ffCommandBuilder) setFormatFlags() (args []string) {
	// Add video codec.
	if len(b.videoCodecGPU) != 0 { // Priority #1: GPU codec.
		args = append(args, b.videoCodecGPU...)
	} else if len(b.videoCodecSoftware) != 0 { // Priority #2: Software codec.
		args = append(args, b.videoCodecSoftware...)
	} else if vCodec, exists := b.formatFlagsMap[consts.FFmpegCV0]; exists { // Priority #3: Format preset codec.
		args = append(args, consts.FFmpegCV0, vCodec)
	}

	// Add audio codec.
	if len(b.audioCodec) != 0 { // Priority #1: Set audio codec.
		args = append(args, b.audioCodec...)
	} else if aCodec, exists := b.formatFlagsMap[consts.FFmpegCA]; exists { // Priority #2: Format preset codec.
		args = append(args, consts.FFmpegCA, aCodec)
	}

	// Add audio rate.
	if len(b.audioRate) != 0 {
		args = append(args, b.audioRate...)
	}

	// Add subtitle.
	if subtitle, exists := b.formatFlagsMap[consts.FFmpegCS]; exists {
		args = append(args, consts.FFmpegCS, subtitle)
	}

	// Add data stream.
	if subtitle, exists := b.formatFlagsMap[consts.FFmpegCD]; exists {
		args = append(args, consts.FFmpegCD, subtitle)
	}

	// Add attachment.
	if attachment, exists := b.formatFlagsMap[consts.FFmpegCT]; exists {
		args = append(args, consts.FFmpegCT, attachment)
	}

	// Add quality.
	if len(b.qualityParameter) != 0 {
		args = append(args, b.qualityParameter...)
	}

	// Add the target codec's other encoder args.
	if len(b.encoderArgs) != 0 {
		args = append(args, b.encoderArgs...)
	}
	return args
}

// buildFinalCommand assembles the final FFmpeg command.
func (b *ffCommandBuilder) buildFinalCommand(formatArgs []string, useHWDecode bool) ([]string, error) {
	args := make([]string, 0, b.calculateCommandCapacity())

	// Add HW acceleration flags (only for decode mode).
	if useHWDecode {
		if len(b.gpuAccelFlags) != 0 {
			args = append(args, b.gpuAccelFlags...)
		}
	}

	// Add GPU device nodes (for encode-only modes like VAAPI), only when the GPU encodes.
	if len(b.gpuNode) != 0 && b.videoCodecGPU != nil {
		args = append(args, b.gpuNode...)
	}

	// Add input file (main video).
	args = append(args, "-y")
	args = append(args, consts.FFmpegGeneratePTS...)
	args = append(args, "-i", b.inputFile)

	// If thumbnail present, add it as an input (must appear before metadata).
	args = append(args, b.thumbnailInput...)

	// Add the streams to keep, then the thumbnail's.
	args = append(args, b.streamMapping...)
	args = append(args, b.thumbnail...)

	// Add format and codec flags.
	args = append(args, formatArgs...)

	// Apply the video filter only to the main video stream (stream 0).
	if b.videoFilter != "" {
		args = append(args, consts.FFmpegFilter, b.videoFilter)
	}

	// Grab output extension.
	outputExt := filepath.Ext(b.outputFile)

	// Insert metatags.
	for key, value := range b.metadataMap {
		if value == "" {
			continue
		}

		// Look up correct metadata keys by container type.
		containerKey := parsing.GetContainerKeys(key, outputExt)
		if containerKey == "" {
			logger.Pl.D(2, "Not inserting key %q, %s files can't hold it.", key, outputExt)
			continue
		}

		// Add metadata argument for key.
		b.builder.Reset()
		b.builder.WriteString(containerKey)
		b.builder.WriteByte('=')
		b.builder.WriteString(strings.ReplaceAll(strings.TrimSpace(value), "\x00", "")) // A NUL stops ffmpeg starting.

		logger.Pl.I("Adding metadata argument: '%s %s'", consts.FFmpegMetadata, b.builder.String())
		args = append(args, consts.FFmpegMetadata, b.builder.String())
	}

	// Extra FFmpeg arguments.
	if abstractions.IsSet(keys.ExtraFFmpegArgs) {
		extraArgs, err := sharedparsing.ShellSplit(abstractions.GetString(keys.ExtraFFmpegArgs))
		if err != nil {
			return nil, err
		}

		// Refuse extra FFmpeg arguments containing disallowed options.
		if !sharedconsts.InsecureArgumentsAllowed() {
			if blocked, blockedKey := sharedconsts.HasBlockedOptions(extraArgs, sharedconsts.BlockedFFmpegOptions); blocked {
				logger.Pl.W("Refusing extra FFmpeg arguments for file %q: Option %q is not allowed for FFmpeg.", b.inputFile, blockedKey)
			} else {
				args = append(args, extraArgs...)
			}
		} else {
			args = append(args, extraArgs...)
		}
	}

	// Name the muxer for extensions FFmpeg can't pick one for.
	args = append(args, outputMuxerArgs(outputExt)...)

	// Add output file last.
	args = append(args, b.outputFile)

	return args, nil
}

// outputMuxerArgs returns the arguments naming the muxer for extensions FFmpeg can't pick one for.
func outputMuxerArgs(outExt string) []string {
	switch strings.ToLower(outExt) {
	case sharedconsts.ExtOGM:
		return []string{consts.FFmpegFormat, consts.FFmpegMuxerOGV}
	case sharedconsts.ExtRMVB:
		return []string{consts.FFmpegFormat, consts.FFmpegMuxerRM}
	}
	return nil
}

// calculateCommandCapacity determines the total length needed for the command.
func (b *ffCommandBuilder) calculateCommandCapacity() int {
	const (
		base = 2 + // "-y", "-i"
			2 + // consts.FFmpegGeneratePTS
			1 + // <input file>
			1 // <output file>

		mapArgMultiply = 2 // consts.FFmpegMetadata + "key=value"
	)

	totalCapacity := base
	totalCapacity += (len(b.metadataMap) * mapArgMultiply)
	totalCapacity += len(b.gpuAccelFlags)
	totalCapacity += len(b.gpuNode)
	totalCapacity += len(b.videoCodecGPU)
	totalCapacity += len(b.videoCodecSoftware)
	totalCapacity += len(b.audioCodec)
	totalCapacity += len(b.qualityParameter)
	totalCapacity += len(b.encoderArgs)
	if b.videoFilter != "" {
		totalCapacity += 2 // consts.FFmpegFilter + filter chain
	}
	totalCapacity += len(b.formatFlagsMap)
	totalCapacity += len(b.thumbnailInput)
	totalCapacity += len(b.streamMapping)
	totalCapacity += len(b.thumbnail)

	if abstractions.IsSet(keys.ExtraFFmpegArgs) {
		totalCapacity += len(strings.Fields(abstractions.GetString(keys.ExtraFFmpegArgs)))
	}

	logger.Pl.D(3, "Total command capacity calculated as: %d", totalCapacity)
	return totalCapacity
}
