package ffmpeg

import (
	"context"
	"fmt"
	"io"
	"metarr/internal/abstractions"
	"metarr/internal/domain/consts"
	"metarr/internal/domain/keys"
	"metarr/internal/domain/logger"
	"metarr/internal/domain/vars"
	"metarr/internal/models"
	"metarr/internal/parsing"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"github.com/TubarrApp/gocommon/sharedconsts"
)

// availableCodecsCache caches the codecs in FFmpeg to avoid repeated calls.
var (
	availableCodecsCache     string
	availableCodecsCacheOnce sync.Once
)

// ffCommandBuilder handles FFmpeg command construction.
type ffCommandBuilder struct {
	// Files
	inputFile  string
	outputFile string

	// Maps
	formatFlagsMap map[string]string
	metadataMap    map[string]string

	// HW accel
	gpuAccelFlags      []string
	gpuNode            []string
	accelCompatibility []string

	// Video codecs
	videoCodecGPU      []string
	videoCodecSoftware []string

	// Audio codec
	audioCodec []string
	audioRate  []string

	// Thumbnail
	thumbnail []string

	// Other parameters
	qualityParameter []string

	// Builder
	builder *strings.Builder
}

// newFfCommandBuilder creates a new FFmpeg command builder.
func newFfCommandBuilder(fd *models.FileData, outputFile string) *ffCommandBuilder {
	return &ffCommandBuilder{
		builder:     &strings.Builder{},
		inputFile:   fd.OriginalVideoPath,
		outputFile:  outputFile,
		metadataMap: make(map[string]string),
	}
}

// buildCommand constructs the complete FFmpeg command.
func (b *ffCommandBuilder) buildCommand(ctx context.Context, fd *models.FileData, desiredVCodec, desiredACodec, outExt string) ([]string, error) {
	if b.inputFile == "" || b.outputFile == "" {
		return nil, fmt.Errorf("input file or output file is empty.\n\nInput file: %v\nOutput file: %v", b.inputFile, b.outputFile)
	}

	// Grab current codecs.
	currentVCodec, currentACodec, err := checkCodecs(b.inputFile)
	if err != nil {
		logger.Pl.E("Failed to check codecs in file %q: %v", b.inputFile, err)
	}
	availableCodecsCacheOnce.Do(func() {
		availableCodecsCache = b.ffmpegAvailableCodecs(ctx)
	})
	availableCodecs := availableCodecsCache

	// Get GPU flags/codecs.
	accelType, useHWDecode := b.setHWAccelFlags()
	b.setGPUAccelerationCodec(accelType, desiredVCodec, availableCodecs)

	logger.Pl.D(1, "Transcoding to codec %q from current codec %q", desiredVCodec, currentVCodec)

	// Get software codecs.
	if b.videoCodecGPU == nil {
		b.setVideoSoftwareCodec(currentVCodec, desiredVCodec, availableCodecs)
	}
	b.setAudioCodec(currentACodec, desiredACodec, availableCodecs)
	b.setTranscodeQuality(accelType)

	b.setDefaultFormatFlagMap(outExt)
	args := b.setFormatFlags()
	b.addAllMetadata(fd)

	stripThumbnails := false
	if abstractions.IsSet(keys.StripThumbnails) {
		stripThumbnails = abstractions.GetBool(keys.StripThumbnails)
	}
	if !stripThumbnails {
		b.setThumbnail(fd.MWebData.Thumbnail, parsing.GetBaseNameWithoutExt(fd.OriginalVideoPath), outExt, fd.HasEmbeddedThumbnail)
	}

	// Return the fully appended argument string.
	return b.buildFinalCommand(args, useHWDecode)
}

// setThumbnail sets the thumbnail image in the video metadata.
//
// NOTE: Uses -map to avoid accumulating thumbnails, if numerous functions need -map additions at some point,
// it will be necessary to create something like a b.streamMapping field and setStreamMapping() function.
func (b *ffCommandBuilder) setThumbnail(thumbnailURL, videoBaseName, outExt string, hasEmbeddedThumbnail bool) {
	if thumbnailURL == "" {
		if hasEmbeddedThumbnail {
			logger.Pl.I("Video %q has an embedded thumbnail. Will copy existing attached_pic.", b.inputFile)

			switch strings.ToLower(outExt) {
			case sharedconsts.ExtMP4,
				sharedconsts.ExtM4V,
				sharedconsts.ExtMOV:

				b.thumbnail = []string{
					"-map", "0:V", // Map only regular video streams (excludes existing attached_pic).
					"-map", "0:a?", // Map audio streams if present.
					"-map", "0:s?", // Map subtitle streams if present.
					"-map", "0:d?", // Map data streams if present.
					"-map", "0:t?", // Map attachment streams if present.
					"-map", "0:v", // Now map attached_pic (will be only the first one found).
					"-c", "copy",
					"-disposition:v:1", "attached_pic",
				}

			case sharedconsts.ExtMKV:
				b.thumbnail = []string{
					"-map", "0",
					"-c", "copy",
				}

			default:
				logger.Pl.D(1, "Copying attached thumbnails not supported for extension: %s", outExt)
			}
			return
		}
		return
	}

	// Thumbnail URL not "" beyond here...

	// Download local thumbnail.
	thumbnail, err := downloadThumbnail(thumbnailURL, videoBaseName)
	if err != nil {
		logger.Pl.E("Could not download thumbnail %q: %v", thumbnailURL, err)
		return
	}

	// Ensure JPG.
	thumbExt := strings.ToLower(filepath.Ext(thumbnail))
	if thumbExt != ".jpg" && thumbExt != ".jpeg" {
		if thumbnail, err = convertToJPG(thumbnail); err != nil {
			logger.Pl.E("Could not convert thumbnail %q to JPG: %v", thumbnail, err)
			return
		}
	}

	ext := strings.ToLower(outExt)
	switch ext {
	case sharedconsts.ExtMP4,
		sharedconsts.ExtM4V,
		sharedconsts.ExtMOV:

		b.thumbnail = []string{
			"-i", thumbnail, // add the thumbnail as a second input.
			"-map", "0:V", // map only regular video streams (excludes any existing attached_pic).
			"-map", "0:a?", // map audio streams if present.
			"-map", "0:s?", // map subtitle streams if present.
			"-map", "0:d?", // map data streams if present.
			"-map", "0:t?", // map attachment streams if present.
			"-map", "1", // map new thumbnail.
			"-c:v:1", "mjpeg", // always use mjpeg codec for thumbnail.
			"-disposition:v:1", "attached_pic", // mark as cover art.
		}

	case sharedconsts.ExtMKV:
		b.thumbnail = []string{
			"-attach", thumbnail,
			"-metadata:s:t", "mimetype=image/jpeg",
		}

	default:
		logger.Pl.D(1, "Thumbnail embedding not supported for extension: %s", ext)
	}
}

// convertToJPG converts an inputted file format to JPG for embedding.
func convertToJPG(inputPath string) (string, error) {
	outputPath := parsing.GetFilepathWithoutExt(inputPath) + ".jpg"
	cmd := exec.Command("ffmpeg", "-y", "-i", inputPath, outputPath)
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("failed to convert %q to jpg: %w", filepath.Ext(inputPath), err)
	}
	return outputPath, nil
}

// downloadThumbnail downloads a thumbnail from a URL to a temporary file.
//
// Returns the local file path to use with FFmpeg -attach.
func downloadThumbnail(urlStr, videoBaseName string) (string, error) {
	if urlStr == "" {
		return "", nil
	}

	resp, err := http.Get(urlStr)
	if err != nil {
		return "", err
	}
	if resp == nil {
		return "", fmt.Errorf("got nil resp in downloadThumbnail()")
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			logger.Pl.E("Failed to close response body due to error: %v", closeErr)
		}
	}()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to download thumbnail: %s", resp.Status)
	}

	// Remove query parameters.
	base, _, _ := strings.Cut(urlStr, "?")
	base, _, _ = strings.Cut(base, "#")

	// Remove illegal characters
	cleanBase := strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') ||
			(r >= 'A' && r <= 'Z') ||
			(r >= '0' && r <= '9') ||
			r == '_' || r == '-' || r == '.' {
			return r
		}
		return '_'
	}, filepath.Base(base))

	// Limit length to stay below filesystem limits.
	if len(cleanBase) > 50 {
		cleanBase = cleanBase[:50]
	}

	tmpPath := filepath.Join(os.TempDir(), "thumb_"+videoBaseName+"_"+cleanBase)

	file, err := os.Create(tmpPath)
	if err != nil {
		return "", err
	}
	defer func() {
		if closeErr := file.Close(); closeErr != nil {
			logger.Pl.E("Failed to close file %q due to error: %v", file.Name(), closeErr)
		}
	}()

	if _, err := io.Copy(file, resp.Body); err != nil {
		return "", err
	}

	return tmpPath, nil
}

// setAudioCodec gets the audio codec for transcode operations.
func (b *ffCommandBuilder) setAudioCodec(currentACodec, desiredACodec, availableCodecs string) {
	switch desiredACodec {
	case sharedconsts.ACodecCopy, currentACodec, "": // -- Set and return early. --
		b.audioCodec = []string{consts.FFmpegCA, sharedconsts.ACodecCopy} // Hardcoded copy (do not use 'desiredACodec').
		return

	case sharedconsts.ACodecAAC, // -- No audio rate needed. --
		sharedconsts.ACodecALAC,
		sharedconsts.ACodecFLAC,
		sharedconsts.ACodecMP2,
		sharedconsts.ACodecMP3,
		sharedconsts.ACodecOpus,
		sharedconsts.ACodecPCM,
		sharedconsts.ACodecVorbis,
		sharedconsts.ACodecTrueHD:

		b.audioCodec = []string{consts.FFmpegCA, desiredACodec}

	case sharedconsts.ACodecAC3, // -- 48KHz audio rate required. --
		sharedconsts.ACodecDTS,
		sharedconsts.ACodecEAC3:
		b.audioCodec = []string{consts.FFmpegCA, desiredACodec}
		b.audioRate = []string{consts.FFmpegAR, consts.AudioRate48khz}

	case sharedconsts.ACodecWAV:
		b.audioCodec = []string{consts.FFmpegCA, "pcm_s16le"}

	default:
		b.audioCodec = nil // -- Invalid or un-set codec. --
	}

	// Check codec availability.
	if len(b.audioCodec) >= 2 {
		if !strings.Contains(availableCodecs, b.audioCodec[1]) {
			logger.Pl.W("Audio codec %q not available in FFmpeg build, falling back to software.", b.audioCodec[1])
			b.audioCodec = []string{consts.FFmpegCA, sharedconsts.ACodecCopy}
		}
	} else if b.audioCodec != nil {
		logger.Pl.E("%s Strings expected to be at least parts, got %v", consts.LogTagDevError, b.audioCodec)
		b.audioCodec = nil
	}
}

// setVideoSoftwareCodec gets the audio codec for transcode operations.
func (b *ffCommandBuilder) setVideoSoftwareCodec(currentVCodec, desiredVCodec, availableCodecs string) {
	switch desiredVCodec {
	case sharedconsts.VCodecCopy, currentVCodec, "": // -- Set and return early. --
		b.videoCodecSoftware = []string{consts.FFmpegCV0, consts.FFVCodecKeyCopy} // Hardcoded copy (do not use 'desiredVCodec').
		return

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

	// Check codec availability.
	if len(b.videoCodecSoftware) >= 2 {
		if !strings.Contains(availableCodecs, b.videoCodecSoftware[1]) {
			logger.Pl.W("Video codec %q not available in FFmpeg build, falling back to software.", b.videoCodecSoftware[1])
			b.videoCodecSoftware = []string{consts.FFmpegCV0, sharedconsts.VCodecCopy}
		}
	} else if b.videoCodecSoftware != nil {
		logger.Pl.E("%s Strings expected to be at least 2 parts, got %v", consts.LogTagDevError, b.videoCodecSoftware)
		b.videoCodecSoftware = nil
	}
}

// ffmpegAvailableCodecs lists codecs available in FFmpeg.
func (b *ffCommandBuilder) ffmpegAvailableCodecs(ctx context.Context) (output string) {
	cmd := exec.CommandContext(ctx, "ffmpeg", "-encoders")
	outBytes, err := cmd.Output()
	result := strings.TrimSpace(string(outBytes))
	if err != nil {
		logger.Pl.E("Codec Grab Failed: %v", err)
		return ""
	}
	return result
}

// setGPUAccelerationCodec sets the codec to use for the GPU acceleration (separated from setGPUAcceleration for ordering reasons).
func (b *ffCommandBuilder) setGPUAccelerationCodec(accelType, useTranscodeCodec, availableCodecs string) {
	if accelType == "" || accelType == sharedconsts.AccelTypeAuto {
		logger.Pl.D(2, "Using 'auto' HW acceleration, will use a standard software codec (e.g. 'libx264')")
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

	// Add compatibility and device nodes for encode-only modes.
	switch accelType {
	case sharedconsts.AccelTypeVAAPI:
		b.accelCompatibility = append(b.accelCompatibility, consts.VAAPICompatibility...)
		// VAAPI requires device node on Linux for encode-only.
		if vars.OS != "linux" {
			logger.Pl.W("VAAPI acceleration is only available on Linux.")
			return "", false
		}
		if gpuNode == "" {
			logger.Pl.W("VAAPI requires a device directory on Linux; falling back to software.")
			return "", false
		}
		b.gpuNode = []string{consts.FFmpegDeviceVAAPI, gpuNode}

	case sharedconsts.AccelTypeAMF:
		// AMF is Windows-only.
		if vars.OS != "windows" {
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

// setTranscodeQuality sets the transcode quality flags for the transcode type.
func (b *ffCommandBuilder) setTranscodeQuality(accelType string) {
	if !abstractions.IsSet(keys.TranscodeQuality) {
		return
	}

	qNum := abstractions.GetString(keys.TranscodeQuality)
	switch accelType {
	case "", sharedconsts.AccelTypeAuto:
		// CRF for software encoders or 'auto'.
		b.qualityParameter = append(b.qualityParameter, consts.FFmpegCRF, qNum)

	case sharedconsts.AccelTypeAMF:
		b.qualityParameter = append(b.qualityParameter, "-qp_p", qNum)

	case sharedconsts.AccelTypeCuda:
		// Nvidia uses CQ.
		b.qualityParameter = append(b.qualityParameter,
			"-rc", "vbr",
			"-cq", qNum,
		)

	case sharedconsts.AccelTypeQSV:
		// Intel uses QSV.
		b.qualityParameter = append(b.qualityParameter, "-global_quality", qNum)

	case sharedconsts.AccelTypeVAAPI:
		// VAAPI uses QP.
		b.qualityParameter = append(b.qualityParameter, "-qp", qNum)
	}
}

// setDefaultFormatFlagMap adds commands specific for the extension input and output.
func (b *ffCommandBuilder) setDefaultFormatFlagMap(outExt string) {
	inExt := strings.ToLower(filepath.Ext(b.inputFile))
	outExt = strings.ToLower(outExt)

	if outExt == "" || strings.EqualFold(inExt, outExt) {
		b.formatFlagsMap = copyPreset
		return
	}

	logger.Pl.D(2, "Making default format map for input extension: %q, output extension: %q. (File: %q)",
		inExt, outExt, b.inputFile)

	// Get format preset from map.
	b.formatFlagsMap = copyPreset
	// Fall back to copy preset if no mapping found.
	b.formatFlagsMap = copyPreset
	logger.Pl.D(1, "No format mapping found for %s to %s conversion, using copy preset",
		inExt, outExt)
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

	// Add GPU device nodes (for encode-only modes like VAAPI, Cuda).
	if len(b.gpuNode) != 0 {
		args = append(args, b.gpuNode...)
	}

	// Add input file (main video).
	args = append(args, "-y", "-i", b.inputFile)

	// If thumbnail present, add it as an input (must appear before metadata).
	if len(b.thumbnail) > 0 {
		args = append(args, b.thumbnail...)
	}

	// Add format and codec flags.
	args = append(args, formatArgs...)

	// Apply GPU compatibility filters only to the main video stream (stream 0).
	if len(b.accelCompatibility) > 0 {
		args = append(args, consts.FFmpegFilter)
		args = append(args, b.accelCompatibility...)
	}

	outputExt := filepath.Ext(b.outputFile)

	for key, value := range b.metadataMap {
		if value == "" {
			continue
		}

		containerKey := parsing.GetContainerKeys(key, outputExt)
		if containerKey == "" {
			logger.Pl.W("Not inserting key: %q, could not find match for container %q.", key, outputExt)
			continue
		}

		// Write metadata argument.
		b.builder.Reset()
		b.builder.WriteString(containerKey)
		b.builder.WriteByte('=')
		b.builder.WriteString(strings.TrimSpace(value))

		logger.Pl.I("Adding metadata argument: '-metadata %s", b.builder.String())
		args = append(args, "-metadata", b.builder.String())
	}

	// Extra FFmpeg arguments.
	if abstractions.IsSet(keys.ExtraFFmpegArgs) {
		args = append(args, strings.Fields(abstractions.GetString(keys.ExtraFFmpegArgs))...)
	}

	// Add output file last.
	args = append(args, b.outputFile)

	return args, nil
}

// calculateCommandCapacity determines the total length needed for the command.
func (b *ffCommandBuilder) calculateCommandCapacity() int {
	const (
		base = 2 + // "-y", "-i"
			1 + // <input file>
			1 // <output file>

		mapArgMultiply = 2 // "-metadata" + "key=value"
	)

	totalCapacity := base
	totalCapacity += (len(b.metadataMap) * mapArgMultiply)
	totalCapacity += len(b.gpuAccelFlags)
	totalCapacity += len(b.gpuNode)
	totalCapacity += len(b.accelCompatibility)
	totalCapacity += len(b.videoCodecGPU)
	totalCapacity += len(b.videoCodecSoftware)
	totalCapacity += len(b.audioCodec)
	totalCapacity += len(b.qualityParameter)
	totalCapacity += len(b.formatFlagsMap)
	totalCapacity += len(b.thumbnail)

	if abstractions.IsSet(keys.TranscodeVideoFilter) {
		totalCapacity += 2 // -vf and flag.
	}

	if abstractions.IsSet(keys.ExtraFFmpegArgs) {
		totalCapacity += len(strings.Fields(abstractions.GetString(keys.ExtraFFmpegArgs)))
	}

	logger.Pl.D(3, "Total command capacity calculated as: %d", totalCapacity)
	return totalCapacity
}
