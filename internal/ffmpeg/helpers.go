package ffmpeg

import (
	"context"
	"fmt"
	"metarr/internal/abstractions"
	"metarr/internal/domain/consts"
	"metarr/internal/domain/keys"
	"metarr/internal/domain/logger"
	"metarr/internal/ffprobe"
	"metarr/internal/parsing"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/TubarrApp/gocommon/sharedconsts"
	"github.com/TubarrApp/gocommon/sharedmodels"
	"github.com/TubarrApp/gocommon/sharedparsing"
	"github.com/TubarrApp/gocommon/sharedvalidation"
)

// fitVideoCodecToContainer returns the video codec to write into a file of extension outputExt, from inputFile.
// A codec chosen to transcode to (or "copy") must fit, as it's never swapped for another. With none chosen, the video's
// own codec is kept, or if it doesn't fit, the preferred codec that does and has its encoder installed is used.
func fitVideoCodecToContainer(ctx context.Context, inputFile, outputExt, inputFileCodec, targetCodec string) (string, error) {
	sourceVCodec, errV := sharedvalidation.ValidateVideoCodec(inputFileCodec)
	readOnlyIn := parsing.ReadOnlyExt(filepath.Ext(inputFile))

	// Check the desired codec against the input file's codec and the output container's capabilities.
	switch targetCodec {
	case sharedconsts.VCodecCopy:
		if !canCopyCodec(ctx, inputFile, outputExt, "V", readOnlyIn, inputFileCodec, errV,
			errV == nil && sharedvalidation.VideoCodecFits(outputExt, sourceVCodec)) {
			if errV != nil {
				return "", fmt.Errorf("can't copy the video unchanged: video codec %q can't go in a %s file", inputFileCodec, outputExt)
			}
			return "", fmt.Errorf("can't copy the video unchanged: %w",
				sharedvalidation.CodecMismatchError(sharedconsts.Video, sourceVCodec, outputExt))
		}

	case "":
		if !canCopyCodec(ctx, inputFile, outputExt, "V", readOnlyIn, inputFileCodec, errV,
			errV == nil && sharedvalidation.VideoCodecFits(outputExt, sourceVCodec)) {
			for _, codec := range sharedvalidation.FittingVideoCodecs(outputExt) {
				if encoderInstalled(ctx, consts.VCodecToFFVCodec[codec]) {
					targetCodec = codec
					break
				}
			}
			if targetCodec == "" {
				return "", fmt.Errorf("video codec %q can't go in a %s file, and no encoder for a codec that can is installed", inputFileCodec, outputExt)
			}
			logger.Pl.I("Video codec %q can't go in a %s file, so transcoding it to %q", inputFileCodec, outputExt, targetCodec)
		}

	default:
		if !sharedvalidation.VideoCodecFits(outputExt, targetCodec) {
			return "", sharedvalidation.CodecMismatchError(sharedconsts.Video, targetCodec, outputExt)
		}
	}

	return targetCodec, nil
}

// fitAudioCodecToContainer returns the audio codec to write into a file of extension outputExt, from inputFile, by the
// same rules as fitVideoCodecToContainer.
func fitAudioCodecToContainer(ctx context.Context, inputFile, outputExt, inputFileCodec, targetCodec string) (string, error) {
	sourceACodec, errA := sharedvalidation.ValidateAudioCodec(inputFileCodec)
	readOnlyIn := parsing.ReadOnlyExt(filepath.Ext(inputFile))

	switch targetCodec {
	case sharedconsts.ACodecCopy:
		if !canCopyCodec(ctx, inputFile, outputExt, "a", readOnlyIn, inputFileCodec, errA,
			errA == nil && sharedvalidation.AudioCodecFits(outputExt, sourceACodec)) {
			if errA != nil {
				return "", fmt.Errorf("can't copy the audio unchanged: audio codec %q can't go in a %s file", inputFileCodec, outputExt)
			}
			return "", fmt.Errorf("can't copy the audio unchanged: %w",
				sharedvalidation.CodecMismatchError(sharedconsts.Audio, sourceACodec, outputExt))
		}
		if sourceACodec == sharedconsts.ACodecAAC && aacCopyBreaksInAVI(inputFile, outputExt) {
			return "", fmt.Errorf("can't copy the audio unchanged: AAC from a %s file can't go in a %s file, as its timing comes out wrong", filepath.Ext(inputFile), outputExt)
		}

	case "":
		if !canCopyCodec(ctx, inputFile, outputExt, "a", readOnlyIn, inputFileCodec, errA,
			errA == nil && sharedvalidation.AudioCodecFits(outputExt, sourceACodec)) {
			for _, codec := range sharedvalidation.FittingAudioCodecs(outputExt) {
				if encoderInstalled(ctx, consts.ACodecToFFACodec[codec]) {
					targetCodec = codec
					break
				}
			}

			if targetCodec == "" {
				return "", fmt.Errorf("audio codec %q can't go in a %s file, and no encoder for a codec that can is installed", inputFileCodec, outputExt)
			}

			logger.Pl.I("Audio codec %q can't go in a %s file, so transcoding it to %q", inputFileCodec, outputExt, targetCodec)
		}

	default:
		if !sharedvalidation.AudioCodecFits(outputExt, targetCodec) {
			return "", sharedvalidation.CodecMismatchError(sharedconsts.Audio, targetCodec, outputExt)
		}
	}

	return targetCodec, nil
}

// aacCopyBreaksInAVI reports whether AAC copied from inputFile into a file of extension outputExt comes out with the wrong
// timing: FFmpeg keeps every frame, but squeezes 10 seconds of audio into about 2.
func aacCopyBreaksInAVI(inputFile, outputExt string) bool {
	if !strings.EqualFold(outputExt, sharedconsts.ExtAVI) {
		return false
	}
	switch strings.ToLower(filepath.Ext(inputFile)) {
	case sharedconsts.ExtASF, sharedconsts.ExtAVI, sharedconsts.ExtWMV:
		return true
	}
	return false
}

// canCopyCodec reports whether a codec can be copied into a file of extension outputExt, from inputFile, given the codec's validation error and whether it fits the output container.
func canCopyCodec(ctx context.Context, inputFile, outputExt, stream string, readOnlyIn bool, codec string, validationErr error, fits bool) bool {
	switch {
	case codec == "":
		return true

	case validationErr == nil:
		return fits

	case readOnlyIn:
		return false

	default:
		return copySurvives(ctx, inputFile, stream, outputExt)
	}
}

// copySurvives reports whether a file's first stream of a type ("V", "a") stays playable copied into a file of extension
// outputExt, by trying its first few seconds: FFmpeg can finish without error, yet leave the stream undecodable or gone.
func copySurvives(ctx context.Context, inputFile, stream, outputExt string) bool {
	// A stream is safe within its own container.
	if strings.EqualFold(filepath.Ext(inputFile), outputExt) {
		return true
	}

	// Make a folder for the trial copy.
	dir, err := os.MkdirTemp("", "metarr-copy-check-")
	if err != nil {
		logger.Pl.W("Could not make a folder to check copying %q, so it won't be copied: %v", inputFile, err)
		return false
	}
	defer func() {
		if err := os.RemoveAll(dir); err != nil {
			logger.Pl.W("Could not remove copy check folder %q: %v", dir, err)
		}
	}()

	// Copy the stream's first few seconds.
	trial := filepath.Join(dir, "trial"+outputExt)
	args := slices.Concat(
		[]string{"-v", "error", "-y"},
		consts.FFmpegGeneratePTS,
		[]string{"-t", consts.CopyCheckSeconds, "-i", inputFile, "-map", "0:" + stream + ":0", "-c", "copy"},
		outputMuxerArgs(outputExt),
		[]string{trial},
	)
	if err := exec.CommandContext(ctx, sharedconsts.ProgramFFmpeg, args...).Run(); err != nil {
		return false
	}

	// It survived if it reads back as the same codec, with at least half of what the source has over that time.
	sourceCodec, sourceFrames := ffprobe.ProbeFirstFrames(ctx, inputFile, stream)
	trialCodec, trialFrames := ffprobe.ProbeFirstFrames(ctx, trial, stream)
	return trialCodec == sourceCodec && trialFrames > 0 && trialFrames >= sourceFrames/2
}

// encoderInstalled reports whether FFmpeg lists the encoder (or its list couldn't be read, so it can't be judged).
func encoderInstalled(ctx context.Context, encoder string) bool {
	encoders := availableEncoders(ctx)
	return encoders == "" || (encoder != "" && strings.Contains(encoders, " "+encoder+" "))
}

// mapsCompressionLevels reports whether a codec map has a line for a compression level, like lc_h264.
func mapsCompressionLevels(key string) bool {
	cMap, ok := abstractions.Get(key).(map[sharedmodels.FromCodec]string)
	if !ok {
		return false
	}
	for from := range cMap {
		if from.Compression != sharedconsts.CompressionBlank {
			return true
		}
	}
	return false
}

// getOutputCodecString returns the codec required according to mapping.
func getOutputCodecString(key string, currentCodec sharedmodels.FromCodec, codecType sharedconsts.MediaType) (outputCodec string) {
	if !abstractions.IsSet(key) {
		return
	}

	// A stream the file doesn't have, or that couldn't be read, has nothing to transcode.
	if currentCodec.Codec == "" || currentCodec.Codec == sharedconsts.CodecCopy {
		return
	}

	// Extract codec from map.
	cMapInterface := abstractions.Get(key)
	cMap, ok := cMapInterface.(map[sharedmodels.FromCodec]string)
	if !ok {
		logger.Pl.DevE("Got wrong type %T for %s codec map", cMapInterface, codecType)
		return
	}

	// Check for an exact codec and compression-level mapping.
	outputCodec, ok = cMap[currentCodec]
	if !ok {
		// Fall back to the codec mapping without a compression level.
		outputCodec, ok = cMap[sharedmodels.NewFromCodec(currentCodec.Codec, sharedconsts.CompressionBlank)]
	}
	if !ok {
		// Fall back to the plain codec, kept under a blank source codec, for codecs the map doesn't list.
		outputCodec = cMap[sharedmodels.NewFromCodec("", sharedconsts.CompressionBlank)]
	}

	// Normalize.
	outputCodec = strings.ToLower(outputCodec)
	outputCodec = strings.ReplaceAll(outputCodec, " ", "")
	outputCodec = strings.ReplaceAll(outputCodec, ".", "")

	return outputCodec
}

// reencodeVideo reports whether a video should be encoded again in its own codec: a "codec:codec" pair always does, and
// with no "copy" or other codec named for it, so does a -pix_fmt in its codec's args that it isn't in.
func reencodeVideo(ctx context.Context, inputFile string, currentCodec sharedmodels.FromCodec) bool {
	// A stream the file doesn't have, or that couldn't be read, has nothing to encode.
	if currentCodec.Codec == "" || currentCodec.Codec == sharedconsts.CodecCopy {
		return false
	}

	// Read what was named, as the codec map's default "copy" looks the same as one asked for.
	plain, pairs, err := sharedparsing.ParseCodecMappings(abstractions.GetStringSlice(keys.TranscodeVideoCodecInput), sharedvalidation.ValidateVideoCodec)
	if err != nil {
		logger.Pl.DevE("Video codec entries passed validation but can't be read: %v", err)
		return false
	}

	// A pair for this video, or a plain codec other than its own, decides.
	if to := namedCodec(currentCodec, plain, pairs); to != "" {
		return to == currentCodec.Codec
	}

	// Only the pixel format can be checked on a finished video, so other args would encode it again on every run.
	args := videoEncoderArgs(currentCodec.Codec)
	if args.PixFmt != "" {
		_, _, sourcePixFmt := ffprobe.ProbeVideoFrame(ctx, inputFile)
		if sourcePixFmt != "" && !pixFmtMatches(currentCodec.Codec, args.PixFmt, sourcePixFmt) {
			logger.Pl.I("Video %q is %s rather than the %s asked, so encoding it again as %s", inputFile, sourcePixFmt, args.PixFmt, currentCodec.Codec)
			return true
		}
	}

	// A filter that won't run changes the picture of some videos and not others.
	if args.VideoFilter != "" {
		logger.Pl.W("Video %q is already %s, so its -vf isn't applied.", inputFile, currentCodec.Codec)
	}
	return false
}

// reencodeAudio reports whether audio should be encoded again in its own codec: a "codec:codec" pair always does, and
// with no "copy" or other codec named for it, so does an -ac or -ar in its codec's args that it doesn't have.
func reencodeAudio(ctx context.Context, inputFile string, currentCodec sharedmodels.FromCodec) bool {
	// A stream the file doesn't have, or that couldn't be read, has nothing to encode.
	if currentCodec.Codec == "" || currentCodec.Codec == sharedconsts.CodecCopy {
		return false
	}

	// Read what was named, as the codec map's default "copy" looks the same as one asked for.
	plain, pairs, err := sharedparsing.ParseCodecMappings(abstractions.GetStringSlice(keys.TranscodeAudioCodecInput), sharedvalidation.ValidateAudioCodec)
	if err != nil {
		logger.Pl.DevE("Audio codec entries passed validation but can't be read: %v", err)
		return false
	}

	// A pair for this audio, or a plain codec other than its own, decides.
	if to := namedCodec(currentCodec, plain, pairs); to != "" {
		return to == currentCodec.Codec
	}

	// Only the channels and sample rate can be checked on finished audio, so other args would encode it again on every run.
	args := audioEncoderArgs(currentCodec.Codec)
	if args.Channels != "" || args.SampleRate != "" {
		channels, sampleRate := ffprobe.ProbeAudioFormat(ctx, inputFile)
		if asked, err := strconv.Atoi(args.Channels); err == nil && channels != 0 && channels != asked {
			logger.Pl.I("Audio in %q has %d channels rather than the %d asked, so encoding it again as %s", inputFile, channels, asked, currentCodec.Codec)
			return true
		}

		// Opus always reads back as 48 kHz, whatever rate it was encoded at, so its rate can't be read.
		if currentCodec.Codec == sharedconsts.ACodecOpus {
			sampleRate = 0
		}
		if asked, err := strconv.Atoi(args.SampleRate); err == nil && sampleRate != 0 && sampleRate != asked {
			logger.Pl.I("Audio in %q is %d Hz rather than the %d asked, so encoding it again as %s", inputFile, sampleRate, asked, currentCodec.Codec)
			return true
		}
	}

	// A filter that won't run changes the sound of some videos and not others.
	if args.Filter != "" {
		logger.Pl.W("Audio in %q is already %s, so its -af isn't applied.", inputFile, currentCodec.Codec)
	}
	return false
}

// namedCodec returns the codec named for a stream already in its codec: its pair's target, else a plain codec other than its own, else "".
func namedCodec(currentCodec sharedmodels.FromCodec, plain string, pairs map[sharedmodels.FromCodec]string) string {
	to, isPair := pairs[currentCodec]
	if !isPair {
		to = pairs[sharedmodels.NewFromCodec(currentCodec.Codec, sharedconsts.CompressionBlank)]
	}
	if to == "" && plain != currentCodec.Codec {
		to = plain
	}
	return to
}

// videoEncoderArgs returns the args set for encoding to a video codec.
func videoEncoderArgs(codec string) sharedmodels.EncoderArgs {
	if !abstractions.IsSet(keys.TranscodeVideoArgsMap) {
		return sharedmodels.EncoderArgs{}
	}
	argsMap, ok := abstractions.Get(keys.TranscodeVideoArgsMap).(map[string]sharedmodels.EncoderArgs)
	if !ok {
		logger.Pl.DevE("Got wrong type %T for video encoder args map", abstractions.Get(keys.TranscodeVideoArgsMap))
	}
	return argsMap[codec]
}

// audioEncoderArgs returns the args set for encoding to an audio codec.
func audioEncoderArgs(codec string) sharedmodels.AudioEncoderArgs {
	argsMap, err := sharedparsing.ParseAudioEncoderArgs(abstractions.GetStringSlice(keys.TranscodeAudioArgs))
	if err != nil {
		logger.Pl.DevE("Audio encoder args passed validation but can't be read: %v", err)
	}
	return argsMap[codec]
}
