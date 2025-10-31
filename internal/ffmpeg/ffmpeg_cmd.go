package ffmpeg

import (
	"fmt"
	"metarr/internal/abstractions"
	"metarr/internal/domain/consts"
	"metarr/internal/domain/keys"
	"metarr/internal/models"
	"metarr/internal/utils/logging"
	"path/filepath"
	"strings"
)

// ffCommandBuilder handles FFmpeg command construction
type ffCommandBuilder struct {
	inputFile          string
	outputFile         string
	formatFlags        []string
	gpuAccel           []string
	gpuAccelCodec      []string
	audioCodec         []string
	videoCodecSoftware []string
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
func (b *ffCommandBuilder) buildCommand(fd *models.FileData, outExt string) ([]string, error) {
	if b.inputFile == "" || b.outputFile == "" {
		return nil, fmt.Errorf("input file or output file is empty.\n\nInput file: %v\nOutput file: %v", b.inputFile, b.outputFile)
	}

	// Grab current codecs
	vCodec, aCodec, err := checkCodecs(b.inputFile)
	if err != nil {
		logging.E("Failed to check codecs in file %q: %v", b.inputFile, err)
	}

	gpuFlag, transcodeCodec, useAccel := b.getHWAccelFlags(vCodec)

	if useAccel {
		b.setGPUAcceleration(gpuFlag)
		b.setGPUAccelerationCodec(gpuFlag, transcodeCodec)
	}

	// Get codecs
	if b.gpuAccelCodec == nil {
		b.setVideoSoftwareCodec()
	}
	b.setAudioCodec(aCodec)

	b.setDefaultFormatFlags(outExt)
	b.setUserFormatFlags()
	b.addAllMetadata(fd)

	// Return the fully appended argument string
	return b.buildFinalCommand(gpuFlag, useAccel)
}

// setAudioCodec gets the audio codec for transcode operations.
func (b *ffCommandBuilder) setAudioCodec(currentACodec string) {
	if !abstractions.IsSet(keys.TranscodeAudioCodec) {
		return
	}
	codec := abstractions.GetString(keys.TranscodeAudioCodec)
	codec = strings.ToLower(codec)
	codec = strings.ReplaceAll(codec, " ", "")
	codec = strings.ReplaceAll(codec, ".", "")

	switch codec {
	case currentACodec, "":
		b.audioCodec = consts.AudioCodecCopy[:] // Codecs match or user codec empty, use copy
	case consts.ACodecAAC:
		b.audioCodec = consts.AudioToAAC[:]
	case consts.ACodecAC3:
		b.audioCodec = consts.AudioToAC3[:]
	case consts.ACodecALAC:
		b.audioCodec = consts.AudioToALAC[:]
	case consts.ACodecDTS:
		b.audioCodec = consts.AudioToDTS[:]
	case consts.ACodecEAC3:
		b.audioCodec = consts.AudioToEAC3[:]
	case consts.ACodecFLAC:
		b.audioCodec = consts.AudioToFLAC[:]
	case consts.ACodecMP2:
		b.audioCodec = consts.AudioToMP2[:]
	case consts.ACodecMP3:
		b.audioCodec = consts.AudioToMP3[:]
	case consts.ACodecOpus:
		b.audioCodec = consts.AudioToOpus[:]
	case consts.ACodecPCM:
		b.audioCodec = consts.AudioToPCM[:]
	case consts.ACodecVorbis:
		b.audioCodec = consts.AudioToVorbis[:]
	case consts.ACodecWAV:
		b.audioCodec = consts.AudioToWAV[:]
	case consts.ACodecTrueHD:
		b.audioCodec = consts.AudioToTrueHD[:]
	default:
		b.audioCodec = []string{"-c:a", codec}
	}
}

// setVideoSoftwareCodec gets the audio codec for transcode operations.
func (b *ffCommandBuilder) setVideoSoftwareCodec() {
	if !abstractions.IsSet(keys.TranscodeVideoCodec) {
		return
	}
	codec := abstractions.GetString(keys.TranscodeVideoCodec)
	codec = strings.ToLower(codec)
	codec = strings.ReplaceAll(codec, " ", "")
	codec = strings.ReplaceAll(codec, ".", "")

	switch codec {
	case consts.VCodecAV1:
		b.videoCodecSoftware = consts.VideoToAV1[:]
	case consts.VCodecH264:
		b.videoCodecSoftware = consts.VideoToH264[:]
	case consts.VCodecHEVC:
		b.videoCodecSoftware = consts.VideoToH265[:]
	case consts.VCodecMPEG2:
		b.videoCodecSoftware = consts.VideoToMPEG2[:]
	case consts.VCodecVP8:
		b.videoCodecSoftware = consts.VideoToVP8[:]
	case consts.VCodecVP9:
		b.videoCodecSoftware = consts.VideoToVP9[:]
	default:
		b.videoCodecSoftware = nil
	}
}

// setGPUAcceleration sets appropriate GPU acceleration flags.
func (b *ffCommandBuilder) setGPUAcceleration(gpuFlag string) {
	switch gpuFlag {
	case consts.AccelTypeAuto:
		b.gpuAccel = consts.AccelFlagAuto[:]

	case consts.AccelTypeNVENC:
		b.gpuAccel = consts.AccelFlagNvidia[:]

	case consts.AccelTypeQSV:
		b.gpuAccel = consts.AccelFlagIntel[:]

	case consts.AccelTypeVAAPI:
		b.gpuAccel = consts.AccelFlagVAAPI[:]
	default:
		logging.E("Invalid hardware transcode flag %q, using software transcode...", gpuFlag)
		return
	}
}

// setGPUAccelerationCodec sets the codec to use for the GPU acceleration (separated from setGPUAcceleration for ordering reasons).
func (b *ffCommandBuilder) setGPUAccelerationCodec(gpuFlag, transcodeCodec string) {
	if gpuFlag == "auto" {
		logging.D(2, "Using 'auto' HW acceleration, will use a standard codec (e.g. libx264 rather than guessing h264_vaapi)")
		return
	}
	sb := strings.Builder{}
	sb.Grow(len(transcodeCodec) + 1 + len(gpuFlag))
	sb.WriteString(transcodeCodec)
	sb.WriteByte('_')
	sb.WriteString(gpuFlag)

	b.gpuAccelCodec = []string{"-c:v", sb.String()}

	command := append(b.gpuAccel, b.gpuAccelCodec...)
	logging.I("Using hardware acceleration:\n\nType: %s\nCodec: %s\nCommand: %v\n", gpuFlag, transcodeCodec, command)
}

// getHWAccelFlags checks and returns the flags for HW acceleration.
func (b *ffCommandBuilder) getHWAccelFlags(vCodec string) (gpuFlag, transcodeVideoCodec string, useHWAccel bool) {
	// Should use GPU?
	if !abstractions.IsSet(keys.UseGPU) {
		return "", "", false
	}

	// Check GPU flag
	gpuFlag = abstractions.GetString(keys.UseGPU)
	gpuFlag = strings.ToLower(gpuFlag)
	if gpuFlag == "" {
		logging.I("HW acceleration flags disabled, using software encode/decode")
		return "", "", false
	}

	// Fetch transcode codec
	if abstractions.IsSet(keys.TranscodeVideoCodec) {
		transcodeVideoCodec = abstractions.GetString(keys.TranscodeVideoCodec)
	}

	// GPU flag but no codec
	if gpuFlag != consts.AccelTypeAuto && transcodeVideoCodec == "" {
		logging.E("Non-auto hardware acceleration (HW accel type entered: %q) requires a codec specified (e.g. h264), falling back to software transcode...", gpuFlag, transcodeVideoCodec)
		return "", "", false
	}

	if gpuMap, exists := unsafeHardwareEncode[gpuFlag]; exists {
		if unsafe, ok := gpuMap[vCodec]; ok && unsafe {
			logging.I("Codec in input file %v is %v, which is not reliably safe for hardware transcoding of type %v. Falling back to software transcode.", b.inputFile, vCodec, gpuFlag)
			return "", "", false
		}
	}

	return gpuFlag, transcodeVideoCodec, true
}

// setDefaultFormatFlags adds commands specific for the extension input and output.
func (b *ffCommandBuilder) setDefaultFormatFlags(outExt string) {
	inExt := strings.ToLower(filepath.Ext(b.inputFile))
	outExt = strings.ToLower(outExt)

	if outExt == "" || strings.EqualFold(inExt, outExt) {
		b.formatFlags = copyPreset.flags
		return
	}

	logging.I("Input extension: %q, output extension: %q, File: %s",
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

// setUserFormatFlags replaces the preset format flags with those inputted by the user.
func (b *ffCommandBuilder) setUserFormatFlags() {
	for i, entry := range b.formatFlags {
		switch entry {
		case "-c:v":
			// HW Accel Case
			if len(b.gpuAccelCodec) == 2 {
				if len(b.formatFlags) >= i {
					logging.I("Replacing preset %q with user selected %q", b.formatFlags[i+1], b.gpuAccelCodec[1])
					b.formatFlags[i+1] = b.gpuAccelCodec[1]

					// VAAPI
					if strings.Contains(b.gpuAccelCodec[1], "vaapi") {
						devDir := []string{"-vaapi_device", abstractions.GetString(keys.TranscodeDeviceDir)}
						b.gpuAccel = append(b.gpuAccel, devDir...)
					}

					// QSV
					if strings.Contains(b.gpuAccelCodec[1], "qsv") {
						devDir := []string{"-qsv_device", abstractions.GetString(keys.TranscodeDeviceDir)}
						b.gpuAccel = append(b.gpuAccel, devDir...)
					}
				} else {
					logging.E("Unexpected end of format flags, %s needs an argument", entry)
				}
			}
			// Software codec case
			if len(b.videoCodecSoftware) == 2 {
				if len(b.formatFlags) > i {
					logging.I("Replacing preset %q with software codec %q", b.formatFlags[i+1], b.videoCodecSoftware[1])
					b.formatFlags[i+1] = b.videoCodecSoftware[1]
				} else {
					logging.E("Unexpected end of format flags")
				}
			}
		case "-c:a":
			if len(b.audioCodec) == 2 {
				if len(b.formatFlags) >= i {
					b.formatFlags[i+1] = b.audioCodec[1]
				} else {
					logging.E("Unexpected end of format flags, %s needs an argument", entry)
				}
			}
		}
	}
}

// buildFinalCommand assembles the final FFmpeg command.
func (b *ffCommandBuilder) buildFinalCommand(gpuFlag string, hwAccel bool) ([]string, error) {
	args := make([]string, 0, b.calculateCommandCapacity(gpuFlag))
	if hwAccel {
		args = append(args, b.gpuAccel...)
	}
	args = append(args, "-y", "-i", b.inputFile)

	// Apply format flags if format flags exist
	if len(b.formatFlags) > 0 {
		args = append(args, b.formatFlags...)

		switch {
		case abstractions.IsSet(keys.TranscodeVideoFilter):
			args = append(args, "-vf", abstractions.GetString(keys.TranscodeVideoFilter))
		case gpuFlag == consts.AccelTypeVAAPI:
			args = append(args, consts.VaapiCompatibility...)
		}
	}

	// Add all -metadata commands
	for key, value := range b.metadataMap {

		// Reset builder
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
func (b *ffCommandBuilder) calculateCommandCapacity(gpuFlag string) int {
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

	if gpuFlag == consts.AccelTypeVAAPI {
		totalCapacity += len(consts.VaapiCompatibility)
	}

	if abstractions.IsSet(keys.TranscodeVideoFilter) {
		totalCapacity += 1 + len(abstractions.GetString(keys.TranscodeVideoFilter)) // "-vf" and flag
	}

	logging.D(3, "Total command capacity calculated as: %d", totalCapacity)
	return totalCapacity
}
