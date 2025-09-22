package ffmpeg

import (
	"fmt"
	"metarr/internal/cfg"
	"metarr/internal/domain/consts"
	"metarr/internal/domain/keys"
	"metarr/internal/models"
	"metarr/internal/utils/logging"
	"os/exec"
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

	gpuFlag, transcodeCodec, useAccel := b.getHWAccelFlags()

	if useAccel {
		b.setGPUAcceleration(gpuFlag)
		b.setGPUAccelerationCodec(gpuFlag, transcodeCodec)
	}

	// Get codecs
	if !useAccel && cfg.IsSet(keys.TranscodeCodec) {
		b.setVideoSoftwareCodec()
	}
	b.setAudioCodec()

	b.setDefaultFormatFlags(outExt)
	b.setUserFormatFlags()
	b.addAllMetadata(fd)

	// Return the fully appended argument string
	return b.buildFinalCommand(gpuFlag, useAccel)
}

// setAudioCodec gets the audio codec for transcode operations.
func (b *ffCommandBuilder) setAudioCodec() {
	if !cfg.IsSet(keys.TranscodeAudioCodec) {
		return
	}

	codec := cfg.GetString(keys.TranscodeAudioCodec)
	codec = strings.ToLower(codec)
	codec = strings.ReplaceAll(codec, " ", "")
	codec = strings.ReplaceAll(codec, ".", "")

	switch codec {
	case "aac":
		b.audioCodec = []string{"-c:a", codec}
	default:
		b.audioCodec = consts.AudioCodecCopy[:]
	}
}

// setVideoSoftwareCodec gets the audio codec for transcode operations.
func (b *ffCommandBuilder) setVideoSoftwareCodec() {
	if !cfg.IsSet(keys.TranscodeCodec) {
		return
	}

	codec := cfg.GetString(keys.TranscodeCodec)
	codec = strings.ToLower(codec)
	codec = strings.ReplaceAll(codec, " ", "")
	codec = strings.ReplaceAll(codec, ".", "")

	switch codec {
	case "h264", "x264":
		b.videoCodecSoftware = consts.VideoToH264[:]
	case "hevc", "h265":
		b.videoCodecSoftware = consts.VideoToH265[:]
	default:
		b.videoCodecSoftware = nil
	}
}

// setGPUAcceleration sets appropriate GPU acceleration flags.
func (b *ffCommandBuilder) setGPUAcceleration(gpuFlag string) {
	switch gpuFlag {
	case "auto":
		b.gpuAccel = consts.AutoHWAccel[:]
	case "nvenc":
		b.gpuAccel = consts.NvidiaAccel[:]
	case "qsv":
		b.gpuAccel = consts.IntelAccel[:]
	case "vaapi":
		b.gpuAccel = consts.VaapiAccel[:]
	default:
		logging.E(0, "Invalid hardware transcode flag %q, using software transcode...", gpuFlag)
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
func (b *ffCommandBuilder) getHWAccelFlags() (gpuFlag, transcodeCodec string, useHWAccel bool) {

	// Should use GPU?
	if !cfg.IsSet(keys.UseGPU) {
		return "", "", false
	}

	// Check GPU flag
	gpuFlag = cfg.GetString(keys.UseGPU)
	gpuFlag = strings.ToLower(gpuFlag)

	if gpuFlag == "" {
		logging.I("HW acceleration flags disabled, using software encode/decode")
		return "", "", false
	}

	// Fetch transcode codec
	if cfg.IsSet(keys.TranscodeCodec) {
		transcodeCodec = cfg.GetString(keys.TranscodeCodec)
	}

	// GPU flag but no codec
	if gpuFlag != "auto" && transcodeCodec == "" {
		logging.E(0, "Non-auto hardware acceleration (HW accel type entered: %q) requires a codec specified (e.g. h264), falling back to software transcode...", gpuFlag, transcodeCodec)
		return "", "", false
	}

	// Check HW acceleration compatability
	vCodec, _, err := b.checkCodecs()
	if err != nil {
		return "", "", false
	}
	vCodec = strings.ToLower(vCodec)

	if gpuMap, exists := unsafeHardwareEncode[gpuFlag]; exists {
		if unsafe, ok := gpuMap[vCodec]; ok && unsafe {
			logging.I("Codec in input file %v is %v, which is not reliably safe for hardware transcoding of type %v. Falling back to software transcode.", b.inputFile, vCodec, gpuFlag)
			return "", "", false
		}
	}

	return gpuFlag, transcodeCodec, true
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
						devDir := []string{"-vaapi_device", cfg.GetString(keys.TranscodeDeviceDir)}
						b.gpuAccel = append(b.gpuAccel, devDir...)
					}

					// QSV
					if strings.Contains(b.gpuAccelCodec[1], "qsv") {
						devDir := []string{"-qsv_device", cfg.GetString(keys.TranscodeDeviceDir)}
						b.gpuAccel = append(b.gpuAccel, devDir...)
					}
				} else {
					logging.E(0, "Unexpected end of format flags")
				}
			}

			// Software codec case
			if len(b.videoCodecSoftware) == 2 {
				if len(b.formatFlags) > i {
					logging.I("Replacing preset %q with software codec %q", b.formatFlags[i+1], b.videoCodecSoftware[1])
					b.formatFlags[i+1] = b.videoCodecSoftware[1]
				} else {
					logging.E(0, "Unexpected end of format flags")
				}
			}

		case "-c:a":
			if len(b.audioCodec) == 2 {
				if len(b.formatFlags) >= i {
					b.formatFlags[i+1] = b.audioCodec[1]
				} else {
					logging.E(0, "Unexpected end of format flags")
				}
			}
		}
	}
}

// buildFinalCommand assembles the final FFmpeg command.
func (b *ffCommandBuilder) buildFinalCommand(gpuFlag string, hwAccel bool) ([]string, error) {
	args := make([]string, 0, calculateCommandCapacity(b))

	if hwAccel {
		args = append(args, b.gpuAccel...)
	}

	args = append(args, "-y", "-i", b.inputFile)

	if len(b.audioCodec) > 0 {
		args = append(args, b.audioCodec...)
	}

	// Apply format flags if format flags exist
	if len(b.formatFlags) > 0 {
		args = append(args, b.formatFlags...)

		switch {
		case cfg.IsSet(keys.TranscodeVideoFilter):
			args = append(args, "-vf", cfg.GetString(keys.TranscodeVideoFilter))
		case gpuFlag == "vaapi":
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

	args = append(args, b.outputFile)

	return args, nil
}

// calculateCommandCapacity determines the total length needed for the command.
func calculateCommandCapacity(b *ffCommandBuilder) int {
	const (
		base = 2 + // "-y", "-i"
			1 + // <input file>
			1 + // "--codec"
			1 // <output file>

		mapArgMultiply = 1 + // "-metadata" for each metadata entry
			1 // "key=value" for each metadata entry
	)

	totalCapacity := base
	totalCapacity += (len(b.metadataMap) * mapArgMultiply)
	totalCapacity += len(b.gpuAccel)
	totalCapacity += len(b.audioCodec)
	totalCapacity += len(b.videoCodecSoftware)
	totalCapacity += len(consts.AutoHWAccel)
	totalCapacity += len(b.gpuAccelCodec)
	totalCapacity += len(b.formatFlags)

	logging.D(3, "Total command capacity calculated as: %d", totalCapacity)
	return totalCapacity
}

// checkCodecs checks the input codec to determine if a straight remux is possible.
func (b *ffCommandBuilder) checkCodecs() (videoCodec, audioCodec string, err error) {

	if b.inputFile == "" {
		return "", "", fmt.Errorf("input file is empty, cannot check codecs")
	}

	cmd := exec.Command("ffprobe",
		"-v", "error",
		"-select_streams", "v:0",
		"-show_entries", "stream=codec_name",
		"-of", "default=noprint_wrappers=1:nokey=1",
		b.inputFile,
	)

	videoCodecBytes, err := cmd.Output()
	if err != nil {
		return "", "", fmt.Errorf("cannot read video codec: %v", err)
	}
	videoCodec = strings.TrimSpace(string(videoCodecBytes))

	cmd = exec.Command("ffprobe",
		"-v", "error",
		"-select_streams", "a:0", // first audio stream
		"-show_entries", "stream=codec_name",
		"-of", "default=noprint_wrappers=1:nokey=1",
		b.inputFile,
	)

	audioCodecBytes, err := cmd.Output()
	if err != nil {
		return "", "", fmt.Errorf("cannot read audio codec: %v", err)
	}
	audioCodec = strings.TrimSpace(string(audioCodecBytes))

	logging.D(2, "Detected codecs - video: %s, audio: %s", videoCodec, audioCodec)

	return videoCodec, audioCodec, nil
}
