package ffmpeg

import (
	"context"
	"fmt"
	"metarr/internal/abstractions"
	"metarr/internal/domain/consts"
	"metarr/internal/domain/keys"
	"metarr/internal/domain/logger"
	"metarr/internal/parsing"
	"strings"

	"github.com/TubarrApp/gocommon/sharedconsts"
	"github.com/TubarrApp/gocommon/sharedvalidation"
)

// fitCodecsToContainer returns the video and audio codecs to write into a file of extension outExt, from one of inExt.
// A codec chosen to transcode to (or "copy") must fit, as it's never swapped for another. With none chosen, the video's
// own codec is kept, or if it doesn't fit, the preferred codec that does and has its encoder installed is used.
func fitCodecsToContainer(ctx context.Context, inExt, outExt, currentV, desiredV, currentA, desiredA string) (v, a string, err error) {
	// Codecs Metarr doesn't know can't be copied out of a read-only container (RealVideo and Cook go only in .rm).
	readOnlyIn := parsing.ReadOnlyExt(inExt)

	// Video Codec:
	sourceV, errV := sharedvalidation.ValidateVideoCodec(currentV)
	stuckV := readOnlyIn && errV != nil && currentV != ""
	switch {
	// Copy codec is desired but cannot be held by chosen output container.
	case desiredV == sharedconsts.VCodecCopy && stuckV:
		return "", "", fmt.Errorf("can't copy the video unchanged: video codec %q can't go in a %s file", currentV, outExt)

	case desiredV == sharedconsts.VCodecCopy &&
		errV == nil &&
		!sharedvalidation.VideoCodecFits(outExt, sourceV):

		return "", "", fmt.Errorf("can't copy the video unchanged: %w", sharedvalidation.CodecMismatchError("video", sourceV, outExt))

		// Non-copy codec chosen cannot be held by chosen output container.
	case desiredV != "" &&
		desiredV != sharedconsts.VCodecCopy &&
		!sharedvalidation.VideoCodecFits(outExt, desiredV):

		return "", "", sharedvalidation.CodecMismatchError("video", desiredV, outExt)

		// Source file codec cannot be held by chosen output container.
	case desiredV == "" &&
		(stuckV || errV == nil && !sharedvalidation.VideoCodecFits(outExt, sourceV)):

		if desiredV = sharedvalidation.FittingVideoCodec(outExt, func(c string) bool {
			return encoderInstalled(ctx, consts.VCodecToFFVCodec[c])
		}); desiredV == "" {
			return "", "", fmt.Errorf("video codec %q can't go in a %s file, and no encoder for a codec that can is installed", currentV, outExt)
		}
		logger.Pl.I("Video codec %q can't go in a %s file, so transcoding it to %q", currentV, outExt, desiredV)
	}

	// Audio codec:
	sourceA, errA := sharedvalidation.ValidateAudioCodec(currentA)
	stuckA := readOnlyIn && errA != nil && currentA != ""
	switch {
	// Copy codec is desired but cannot be held by chosen output container.
	case desiredA == sharedconsts.ACodecCopy && stuckA:
		return "", "", fmt.Errorf("can't copy the audio unchanged: audio codec %q can't go in a %s file", currentA, outExt)

	case desiredA == sharedconsts.ACodecCopy &&
		errA == nil &&
		!sharedvalidation.AudioCodecFits(outExt, sourceA):

		return "", "", fmt.Errorf("can't copy the audio unchanged: %w", sharedvalidation.CodecMismatchError("audio", sourceA, outExt))

		// Non-copy codec chosen cannot be held by chosen output container.
	case desiredA != "" &&
		desiredA != sharedconsts.ACodecCopy &&
		!sharedvalidation.AudioCodecFits(outExt, desiredA):

		return "", "", sharedvalidation.CodecMismatchError("audio", desiredA, outExt)

		// Source file codec cannot be held by chosen output container.
	case desiredA == "" &&
		(stuckA || errA == nil && !sharedvalidation.AudioCodecFits(outExt, sourceA)):

		if desiredA = sharedvalidation.FittingAudioCodec(outExt, func(c string) bool {
			return encoderInstalled(ctx, consts.ACodecToFFACodec[c])
		}); desiredA == "" {
			return "", "", fmt.Errorf("audio codec %q can't go in a %s file, and no encoder for a codec that can is installed", currentA, outExt)
		}
		logger.Pl.I("Audio codec %q can't go in a %s file, so transcoding it to %q", currentA, outExt, desiredA)
	}

	// Return compatible codecs
	return desiredV, desiredA, nil
}

// encoderInstalled reports whether FFmpeg lists the encoder (or its list couldn't be read, so it can't be judged).
func encoderInstalled(ctx context.Context, encoder string) bool {
	encoders := availableEncoders(ctx)
	return encoders == "" || (encoder != "" && strings.Contains(encoders, " "+encoder+" "))
}

// getOutputVideoCodecString returns the codec required according to mapping.
func getOutputVideoCodecString(currentVCodec string) (outputCodec string) {
	if !abstractions.IsSet(keys.TranscodeVideoCodecMap) {
		return
	}

	// Extract codec from map.
	cMapInterface := abstractions.Get(keys.TranscodeVideoCodecMap)
	cMap, ok := cMapInterface.(map[string]string)
	if !ok {
		logger.Pl.E("Dev Error: Got wrong type %T for video codec map", cMapInterface)
		return
	}
	codec := cMap[currentVCodec]

	// Normalize.
	codec = strings.ToLower(codec)
	codec = strings.ReplaceAll(codec, " ", "")
	codec = strings.ReplaceAll(codec, ".", "")

	return codec
}

// getOutputAudioCodecString returns the codec required according to mapping.
func getOutputAudioCodecString(currentACodec string) (outputCodec string) {
	if !abstractions.IsSet(keys.TranscodeAudioCodecMap) {
		return
	}

	// Extract codec from map.
	cMapInterface := abstractions.Get(keys.TranscodeAudioCodecMap)
	cMap, ok := cMapInterface.(map[string]string)
	if !ok {
		logger.Pl.E("Dev Error: Got wrong type %T for audio codec map", cMapInterface)
		return
	}
	codec := cMap[currentACodec]

	// Normalize.
	codec = strings.ToLower(codec)
	codec = strings.ReplaceAll(codec, " ", "")
	codec = strings.ReplaceAll(codec, ".", "")

	return codec
}
