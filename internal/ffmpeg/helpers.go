package ffmpeg

import (
	"context"
	"fmt"
	"metarr/internal/abstractions"
	"metarr/internal/domain/consts"
	"metarr/internal/domain/keys"
	"metarr/internal/domain/logger"
	"strings"

	"github.com/TubarrApp/gocommon/sharedconsts"
	"github.com/TubarrApp/gocommon/sharedvalidation"
)

// fitCodecsToContainer returns the video and audio codecs to write into a file of extension outExt. A codec chosen to
// transcode to (or "copy") must fit, as it's never swapped for another. With none chosen, the video's own codec is
// kept, or if it doesn't fit, the preferred codec that does and has its encoder installed is used.
func fitCodecsToContainer(ctx context.Context, outExt, currentV, desiredV, currentA, desiredA string) (v, a string, err error) {
	// Video Codec:
	sourceV, errV := sharedvalidation.ValidateVideoCodec(currentV)
	switch {
	// Copy codec is desired but cannot be held by chosen output container.
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
		errV == nil &&
		!sharedvalidation.VideoCodecFits(outExt, sourceV):

		if desiredV = sharedvalidation.FittingVideoCodec(outExt, func(c string) bool {
			return encoderInstalled(ctx, consts.VCodecToFFVCodec[c])
		}); desiredV == "" {
			return "", "", fmt.Errorf("video codec %q can't go in a %s file, and no encoder for a codec that can is installed", sourceV, outExt)
		}
		logger.Pl.I("Video codec %q can't go in a %s file, so transcoding it to %q", sourceV, outExt, desiredV)
	}

	// Audio codec:
	sourceA, errA := sharedvalidation.ValidateAudioCodec(currentA)
	switch {
	// Copy codec is desired but cannot be held by chosen output container.
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
		errA == nil &&
		!sharedvalidation.AudioCodecFits(outExt, sourceA):

		if desiredA = sharedvalidation.FittingAudioCodec(outExt, func(c string) bool {
			return encoderInstalled(ctx, consts.ACodecToFFACodec[c])
		}); desiredA == "" {
			return "", "", fmt.Errorf("audio codec %q can't go in a %s file, and no encoder for a codec that can is installed", sourceA, outExt)
		}
		logger.Pl.I("Audio codec %q can't go in a %s file, so transcoding it to %q", sourceA, outExt, desiredA)
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
