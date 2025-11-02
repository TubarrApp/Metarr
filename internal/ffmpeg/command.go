package ffmpeg

import (
	"context"
	"fmt"
	"metarr/internal/abstractions"
	"metarr/internal/domain/consts"
	"metarr/internal/domain/keys"
	"metarr/internal/models"
	"metarr/internal/utils/logging"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
)

// ffCommandBuilder handles FFmpeg command construction
type ffCommandBuilder struct {
	inputFile          string
	outputFile         string
	formatFlags        map[string]string
	gpuAccel           []string
	gpuAccelCodec      []string
	gpuDir             []string
	gpuCompatability   []string
	audioCodec         []string
	videoCodecSoftware []string
	qualityParameter   []string
	metadataMap        map[string]string
	builder            *strings.Builder
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
func (b *ffCommandBuilder) buildCommand(ctx context.Context, fd *models.FileData, outExt string) ([]string, error) {
	if b.inputFile == "" || b.outputFile == "" {
		return nil, fmt.Errorf("input file or output file is empty.\n\nInput file: %v\nOutput file: %v", b.inputFile, b.outputFile)
	}
	// Grab current codecs
	currentVCodec, currentACodec, err := checkCodecs(b.inputFile)
	if err != nil {
		logging.E("Failed to check codecs in file %q: %v", b.inputFile, err)
	}
	availableCodecs := b.ffmpegCodecOutput(ctx)

	// Get GPU flags/codecs
	accelType, transcodeCodec, useHW := b.getHWAccelFlags()
	if useHW {
		b.setGPUAcceleration(accelType)
		b.setGPUAccelerationCodec(accelType, transcodeCodec, availableCodecs)
	}

	// Get software codecs
	if b.gpuAccelCodec == nil {
		b.setVideoSoftwareCodec(currentVCodec, availableCodecs)
	}
	b.setAudioCodec(currentACodec, availableCodecs)
	b.setTranscodeQuality(accelType)

	b.setDefaultFormatFlagMap(outExt)
	args := b.setFormatFlags()
	b.addAllMetadata(fd)

	// Return the fully appended argument string
	return b.buildFinalCommand(args, useHW)
}

// setAudioCodec gets the audio codec for transcode operations.
func (b *ffCommandBuilder) setAudioCodec(currentACodec, availableCodecs string) {
	if !abstractions.IsSet(keys.TranscodeAudioCodec) {
		return
	}
	codec := abstractions.GetString(keys.TranscodeAudioCodec)
	codec = strings.ToLower(codec)
	codec = strings.ReplaceAll(codec, " ", "")
	codec = strings.ReplaceAll(codec, ".", "")

	switch codec {
	case consts.ACodecAAC:
		b.audioCodec = []string{consts.FFmpegCA, consts.AudioToAAC}
	case consts.ACodecAC3:
		b.audioCodec = []string{consts.FFmpegCA, consts.AudioToAC3}
	case consts.ACodecALAC:
		b.audioCodec = []string{consts.FFmpegCA, consts.AudioToALAC}
	case consts.ACodecDTS:
		b.audioCodec = []string{consts.FFmpegCA, consts.AudioToDTS}
	case consts.ACodecEAC3:
		b.audioCodec = []string{consts.FFmpegCA, consts.AudioToEAC3}
	case consts.ACodecFLAC:
		b.audioCodec = []string{consts.FFmpegCA, consts.AudioToFLAC}
	case consts.ACodecMP2:
		b.audioCodec = []string{consts.FFmpegCA, consts.AudioToMP2}
	case consts.ACodecMP3:
		b.audioCodec = []string{consts.FFmpegCA, consts.AudioToMP3}
	case consts.ACodecOpus:
		b.audioCodec = []string{consts.FFmpegCA, consts.AudioToOpus}
	case consts.ACodecPCM:
		b.audioCodec = []string{consts.FFmpegCA, consts.AudioToPCM}
	case consts.ACodecVorbis:
		b.audioCodec = []string{consts.FFmpegCA, consts.AudioToVorbis}
	case consts.ACodecWAV:
		b.audioCodec = []string{consts.FFmpegCA, consts.AudioToWAV}
	case consts.ACodecTrueHD:
		b.audioCodec = []string{consts.FFmpegCA, consts.AudioToTrueHD}
	case currentACodec, "":
		b.audioCodec = []string{consts.FFmpegCA, consts.AudioCodecCopy} // Codecs match or user codec empty, use copy
	default:
		b.audioCodec = nil
	}

	if len(b.audioCodec) == 2 {
		if !strings.Contains(availableCodecs, b.audioCodec[1]) {
			logging.W("Audio codec %q not available in FFmpeg build, falling back to software.", b.audioCodec[1])
			b.audioCodec = []string{consts.FFmpegCA, consts.AudioCodecCopy}
		}
	} else if b.audioCodec != nil {
		logging.E("%s Strings expected to be 2 parts, got %v", consts.LogTagDevError, b.audioCodec)
		b.audioCodec = nil
	}
}

// setVideoSoftwareCodec gets the audio codec for transcode operations.
func (b *ffCommandBuilder) setVideoSoftwareCodec(currentVCodec, availableCodecs string) {
	if !abstractions.IsSet(keys.TranscodeVideoCodec) {
		return
	}
	codec := abstractions.GetString(keys.TranscodeVideoCodec)
	codec = strings.ToLower(codec)
	codec = strings.ReplaceAll(codec, " ", "")
	codec = strings.ReplaceAll(codec, ".", "")

	switch codec {
	case consts.VCodecAV1:
		b.videoCodecSoftware = []string{consts.FFmpegCV, consts.VideoToAV1}
	case consts.VCodecH264:
		b.videoCodecSoftware = []string{consts.FFmpegCV, consts.VideoToH264}
	case consts.VCodecHEVC:
		b.videoCodecSoftware = []string{consts.FFmpegCV, consts.VideoToH265}
	case consts.VCodecMPEG2:
		b.videoCodecSoftware = []string{consts.FFmpegCV, consts.VideoToMPEG2}
	case consts.VCodecVP8:
		b.videoCodecSoftware = []string{consts.FFmpegCV, consts.VideoToVP8}
	case consts.VCodecVP9:
		b.videoCodecSoftware = []string{consts.FFmpegCV, consts.VideoToVP9}
	case currentVCodec, "":
		b.videoCodecSoftware = []string{consts.FFmpegCV, consts.VideoCodecCopy}
	default:
		b.videoCodecSoftware = nil
	}

	if len(b.videoCodecSoftware) == 2 {
		if !strings.Contains(availableCodecs, b.videoCodecSoftware[1]) {
			logging.W("Video codec %q not available in FFmpeg build, falling back to software.", b.videoCodecSoftware[1])
			b.videoCodecSoftware = []string{consts.FFmpegCV, consts.VideoCodecCopy}
		}
	} else if b.videoCodecSoftware != nil {
		logging.E("%s Strings expected to be 2 parts, got %v", consts.LogTagDevError, b.videoCodecSoftware)
		b.videoCodecSoftware = nil
	}
}

// ffmpegCodecOutput ensures the desired codec is available.
func (b *ffCommandBuilder) ffmpegCodecOutput(ctx context.Context) (output string) {
	cmd := exec.CommandContext(ctx, "ffmpeg", "-encoders")
	outBytes, err := cmd.Output()
	result := strings.TrimSpace(string(outBytes))
	if err != nil {
		logging.E("Codec Grab Failed: %v", err)
		return ""
	}
	return result
}

// setGPUAcceleration sets appropriate GPU acceleration flags.
func (b *ffCommandBuilder) setGPUAcceleration(accelType string) {
	var transcodeDir string
	if abstractions.IsSet(keys.TranscodeDeviceDir) {
		transcodeDir = abstractions.GetString(keys.TranscodeDeviceDir)
	}

	logging.I("Got GPU flag: %q", accelType)
	switch accelType {
	case consts.AccelTypeAuto:
		b.gpuAccel = []string{consts.FFmpegHWAccel, consts.AccelTypeAuto}

	case consts.AccelTypeNvidia:
		if transcodeDir != "" {
			b.gpuAccel = []string{
				consts.FFmpegHWAccel, consts.AccelTypeNvidia,
				consts.FFmpegHWAccelOutputFormat, consts.AccelTypeNvidia,
			}
			devNumber := strings.TrimPrefix(transcodeDir, "/dev/nvidia")
			if _, err := strconv.ParseInt(devNumber, 10, 64); err == nil { // if err IS nil
				b.gpuDir = []string{consts.FFmpegDeviceHW, devNumber}
			} else {
				logging.E("Nvidia device directory %q not valid, should end in a digit e.g. '/dev/nvidia0")
			}
			b.gpuCompatability = append(b.gpuCompatability, consts.FFmpegVF)
			b.gpuCompatability = append(b.gpuCompatability, consts.CudaCompatability...)
		}

	case consts.AccelTypeQSV:
		if transcodeDir != "" {
			b.gpuAccel = []string{
				consts.FFmpegHWAccel, consts.AccelTypeQSV,
				consts.FFmpegHWAccelOutputFormat, consts.AccelTypeQSV,
			}
			b.gpuDir = []string{consts.FFmpegDeviceQSV, transcodeDir}
		}

	case consts.AccelTypeVAAPI:
		if transcodeDir != "" {
			b.gpuAccel = []string{
				consts.FFmpegHWAccel, consts.AccelTypeVAAPI,
				consts.FFmpegHWAccelOutputFormat, consts.AccelTypeVAAPI,
			}
			b.gpuDir = []string{consts.FFmpegDeviceVAAPI, transcodeDir}
			b.gpuCompatability = append(b.gpuCompatability, consts.FFmpegVF)
			b.gpuCompatability = append(b.gpuCompatability, consts.VAAPICompatability...)
		}

	default:
		logging.E("Invalid hardware transcode flag %q, using software transcode...", accelType)
		return
	}
}

// setGPUAccelerationCodec sets the codec to use for the GPU acceleration (separated from setGPUAcceleration for ordering reasons).
func (b *ffCommandBuilder) setGPUAccelerationCodec(accelType, transcodeCodec, availableCodecs string) {
	if accelType == "" || accelType == consts.AccelTypeAuto {
		logging.D(2, "Using 'auto' HW acceleration, will use a standard software codec (e.g. 'libx264')")
		return
	}

	sb := strings.Builder{}
	sb.Grow(len(transcodeCodec) + 1 + len(accelType))
	sb.WriteString(transcodeCodec)
	sb.WriteByte('_')
	if accelType == consts.AccelTypeNvidia {
		sb.WriteString(consts.AccelFlagNvenc)
	} else {
		sb.WriteString(accelType)
	}

	gpuCodecString := sb.String()
	b.gpuAccelCodec = []string{consts.FFmpegCV, gpuCodecString}

	if !strings.Contains(availableCodecs, gpuCodecString) {
		logging.W("GPU-bound video codec %q not available in FFmpeg build, falling back to software.", gpuCodecString)
		b.gpuAccelCodec = nil
		b.gpuAccel = nil
	}
	if b.gpuAccel != nil && b.gpuAccelCodec != nil {
		command := append(b.gpuAccel, b.gpuAccelCodec...)
		logging.I("Using hardware acceleration:\n\nType: %s\nCodec: %s\nArguments: %v\n", accelType, transcodeCodec, command)
	}
}

// getHWAccelFlags checks and returns the flags for HW acceleration.
func (b *ffCommandBuilder) getHWAccelFlags() (accelType, vCodec string, useHW bool) {
	if !abstractions.IsSet(keys.UseGPU) {
		return "", "", false
	}

	// Check GPU flag
	accelType = abstractions.GetString(keys.UseGPU)
	accelType = strings.ToLower(accelType)
	if accelType == "" {
		logging.I("HW acceleration flags disabled, using software encode/decode")
		return "", "", false
	}

	// Fetch transcode codec
	var transcodeVideoCodec string
	if abstractions.IsSet(keys.TranscodeVideoCodec) {
		transcodeVideoCodec = abstractions.GetString(keys.TranscodeVideoCodec)
	}

	// Do not use HW on copy
	if transcodeVideoCodec == "copy" {
		logging.I("Video codec is 'copy', hardware acceleration not needed")
		return "", "", false
	}

	// GPU flag but no codec
	if accelType != consts.AccelTypeAuto && transcodeVideoCodec == "" {
		logging.E("Non-auto hardware acceleration (HW accel type entered: %q) requires a codec specified (e.g. h264), falling back to software transcode...", accelType)
		return "", "", false
	}

	if gpuMap, exists := unsafeHardwareEncode[accelType]; exists {
		if unsafe, ok := gpuMap[transcodeVideoCodec]; ok && unsafe {
			logging.I("Codec in input file %q is %q, which is not reliably safe for hardware transcoding of type %q. Falling back to software transcode.", b.inputFile, transcodeVideoCodec, accelType)
			return "", "", false
		}
	}

	switch {
	case transcodeVideoCodec != "" && accelType != "":
		return accelType, transcodeVideoCodec, true
	case accelType == consts.AccelTypeAuto:
		return accelType, "", true
	default:
		return "", "", false
	}
}

// setTranscodeQuality sets the transcode quality flags for the transcode type.
func (b *ffCommandBuilder) setTranscodeQuality(accelType string) {
	if !abstractions.IsSet(keys.TranscodeQuality) {
		return
	}

	qNum := abstractions.GetString(keys.TranscodeQuality)
	switch accelType {
	case "", consts.AccelTypeAuto:
		// CRF for software encoders or 'auto'
		b.qualityParameter = append(b.qualityParameter, consts.FFmpegCRF, qNum)

	case consts.AccelTypeAMF:
		b.qualityParameter = append(b.qualityParameter, "-qp_p", qNum)

	case consts.AccelTypeNvidia:
		// NVIDIA NVENC uses CQ
		b.qualityParameter = append(b.qualityParameter,
			"-rc", "vbr",
			"-cq", qNum,
		)

	case consts.AccelTypeQSV:
		// Intel QSV uses global_quality
		b.qualityParameter = append(b.qualityParameter, "-global_quality", qNum)

	case consts.AccelTypeVAAPI:
		// VAAPI uses QP
		b.qualityParameter = append(b.qualityParameter, "-qp", qNum)
	}
}

// setDefaultFormatFlagMap adds commands specific for the extension input and output.
func (b *ffCommandBuilder) setDefaultFormatFlagMap(outExt string) {
	inExt := strings.ToLower(filepath.Ext(b.inputFile))
	outExt = strings.ToLower(outExt)

	if outExt == "" || strings.EqualFold(inExt, outExt) {
		b.formatFlags = copyPreset.flags
		return
	}

	logging.D(2, "Making default format map for input extension: %q, output extension: %q. (File: %q)",
		inExt, outExt, b.inputFile)

	// Get format preset from map
	if presets, exists := formatMap[outExt]; exists {
		// Try exact input format match
		if preset, exists := presets[inExt]; exists {
			b.formatFlags = preset.flags
			return
		}
		// Fall back to default preset for this output format
		if preset, exists := presets["*"]; exists {
			b.formatFlags = preset.flags
			return
		}
	}
	// Fall back to copy preset if no mapping found
	b.formatFlags = copyPreset.flags
	logging.D(1, "No format mapping found for %s to %s conversion, using copy preset",
		inExt, outExt)
}

// setFormatFlags sets flags for the transcoding format, e.g. codec, etc.
func (b *ffCommandBuilder) setFormatFlags() (args []string) {
	// Add compatability filters
	if b.gpuCompatability != nil {
		args = append(args, b.gpuCompatability...)
	}

	// Add flags with possible compatability clash
	if abstractions.IsSet(keys.TranscodeVideoFilter) && !slices.Contains(args, consts.FFmpegVF) {
		args = append(args, consts.FFmpegVF, abstractions.GetString(keys.TranscodeVideoFilter))
	}

	// Add video codec
	if len(b.gpuAccelCodec) != 0 {
		args = append(args, b.gpuAccelCodec...)
	} else if len(b.videoCodecSoftware) != 0 {
		args = append(args, b.videoCodecSoftware...)
	} else if vCodec, exists := b.formatFlags[consts.FFmpegCV]; exists {
		args = append(args, consts.FFmpegCV, vCodec)
	}

	// Add audio codec
	if len(b.audioCodec) != 0 {
		args = append(args, b.audioCodec...)
	} else if aCodec, exists := b.formatFlags[consts.FFmpegCA]; exists {
		args = append(args, consts.FFmpegCA, aCodec)
	}

	// Add subtitle
	if subtitle, exists := b.formatFlags[consts.FFmpegCS]; exists {
		args = append(args, consts.FFmpegCS, subtitle)
	}

	// Add data stream
	if subtitle, exists := b.formatFlags[consts.FFmpegCD]; exists {
		args = append(args, consts.FFmpegCD, subtitle)
	}

	// Add attachment
	if attachment, exists := b.formatFlags[consts.FFmpegCT]; exists {
		args = append(args, consts.FFmpegCT, attachment)
	}

	// Add quality
	if len(b.qualityParameter) != 0 {
		args = append(args, b.qualityParameter...)
	}
	return args
}

// buildFinalCommand assembles the final FFmpeg command.
func (b *ffCommandBuilder) buildFinalCommand(formatArgs []string, useHW bool) ([]string, error) {
	args := make([]string, 0, b.calculateCommandCapacity())

	// Add HW acceleration flags
	if useHW {
		if len(b.gpuAccel) != 0 {
			args = append(args, b.gpuAccel...)
		}
		if len(b.gpuDir) != 0 {
			args = append(args, b.gpuDir...)
		}
	}

	// Add input file
	args = append(args, "-y", "-i", b.inputFile)
	args = append(args, formatArgs...)

	// Add all -metadata commands
	for key, value := range b.metadataMap {
		b.builder.Reset()
		b.builder.WriteString(key)
		b.builder.WriteByte('=')
		b.builder.WriteString(strings.TrimSpace(value))

		// Write argument
		logging.I("Adding metadata argument: '-metadata %s", b.builder.String())
		args = append(args, "-metadata", b.builder.String())
	}

	// Extra FFmpeg arguments
	if abstractions.IsSet(keys.ExtraFFmpegArgs) {
		args = append(args, strings.Fields(abstractions.GetString(keys.ExtraFFmpegArgs))...)
	}

	// Add output file
	args = append(args, b.outputFile)

	return args, nil
}

// calculateCommandCapacity determines the total length needed for the command.
func (b *ffCommandBuilder) calculateCommandCapacity() int {
	const (
		base = 2 + // "-y", "-i"
			1 + // <input file>
			1 + // "--codec"
			1 // <output file>

		mapArgMultiply = 1 + // "-metadata" for each metadata entry
			1 // "key=value" for each metadata entry
	)

	totalCapacity := base
	totalCapacity += 2 // "-hwaccel" and type
	totalCapacity += (len(b.metadataMap) * mapArgMultiply)
	totalCapacity += len(b.gpuAccel)
	totalCapacity += len(b.videoCodecSoftware)
	totalCapacity += len(b.gpuAccelCodec)
	totalCapacity += len(b.formatFlags)

	if abstractions.IsSet(keys.TranscodeVideoFilter) {
		totalCapacity += 2 // consts.FFmpegVF and flag
	}

	logging.D(3, "Total command capacity calculated as: %d", totalCapacity)
	return totalCapacity
}
