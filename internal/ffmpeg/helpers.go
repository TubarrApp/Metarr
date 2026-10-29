package ffmpeg

import (
	"context"
	"errors"
	"fmt"
	"metarr/internal/abstractions"
	"metarr/internal/domain/consts"
	"metarr/internal/domain/keys"
	"metarr/internal/domain/logger"
	"metarr/internal/parsing"
	"strings"
	"unicode"

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
	// sourceV = Normalized codec name (e.g. h.264 becomes h264, h265 becomes hevc).
	// uncopyableV = Unknown non-blank codec in read-only container (RealVideo).
	// desiredV = Codec desired by user, e.g. 'copy' or 'hevc'.
	// errV = Codec not a valid codec in map (or no alias matched). See 'sharedconsts.ValidVideoCodecs'.
	sourceV, errV := sharedvalidation.ValidateVideoCodec(currentV)
	uncopyableV := readOnlyIn && errV != nil && currentV != ""

	switch {
	case desiredV == sharedconsts.VCodecCopy && uncopyableV: // Copy codec is desired but cannot be held by chosen output container.
		return "", "", fmt.Errorf("can't copy the video unchanged: video codec %q can't go in a %s file", currentV, outExt)

	case desiredV == sharedconsts.VCodecCopy &&
		errV == nil &&
		!sharedvalidation.VideoCodecFits(outExt, sourceV): // Desires 'copy', no error, but target container can't hold the input codec.

		return "", "", fmt.Errorf("can't copy the video unchanged: %w", sharedvalidation.CodecMismatchError("video", sourceV, outExt))

	case desiredV != "" &&
		desiredV != sharedconsts.VCodecCopy &&
		!sharedvalidation.VideoCodecFits(outExt, desiredV): // Non-copy codec chosen cannot be held by chosen output container.

		return "", "", sharedvalidation.CodecMismatchError("video", desiredV, outExt)

	case desiredV == "" &&
		(uncopyableV ||
			(errV == nil && !sharedvalidation.VideoCodecFits(outExt, sourceV))): // Source file codec cannot be held by chosen output container.

		// Use the most preferred codec that fits and has its encoder installed.
		for _, c := range sharedvalidation.FittingVideoCodecs(outExt) {
			if encoderInstalled(ctx, consts.VCodecToFFVCodec[c]) {
				desiredV = c
				break
			}
		}
		if desiredV == "" {
			return "", "", fmt.Errorf("video codec %q can't go in a %s file, and no encoder for a codec that can is installed", currentV, outExt)
		}

		logger.Pl.I("Video codec %q can't go in a %s file, so transcoding it to %q", currentV, outExt, desiredV)
	}

	// Audio codec:
	// sourceA = Normalized codec name (e.g. aaclc becomes aac).
	// uncopyableA = Unknown non-blank codec in read-only container (Cook).
	// desiredA = Codec desired by user, e.g. 'copy' or 'aac'.
	// errA = Codec not a valid codec in map (or no alias matched). See 'sharedconsts.ValidAudioCodecs'.
	sourceA, errA := sharedvalidation.ValidateAudioCodec(currentA)
	uncopyableA := readOnlyIn && errA != nil && currentA != ""

	switch {
	case desiredA == sharedconsts.ACodecCopy && uncopyableA: // Copy codec is desired but cannot be held by chosen output container.
		return "", "", fmt.Errorf("can't copy the audio unchanged: audio codec %q can't go in a %s file", currentA, outExt)

	case desiredA == sharedconsts.ACodecCopy &&
		errA == nil &&
		!sharedvalidation.AudioCodecFits(outExt, sourceA): // Desires 'copy', no error, but target container can't hold the input codec.

		return "", "", fmt.Errorf("can't copy the audio unchanged: %w", sharedvalidation.CodecMismatchError("audio", sourceA, outExt))

	case desiredA != "" &&
		desiredA != sharedconsts.ACodecCopy &&
		!sharedvalidation.AudioCodecFits(outExt, desiredA): // Non-copy codec chosen cannot be held by chosen output container.

		return "", "", sharedvalidation.CodecMismatchError("audio", desiredA, outExt)

	case desiredA == "" &&
		(uncopyableA ||
			(errA == nil && !sharedvalidation.AudioCodecFits(outExt, sourceA))): // Source file codec cannot be held by chosen output container.

		// Use the most preferred codec that fits and has its encoder installed.
		for _, c := range sharedvalidation.FittingAudioCodecs(outExt) {
			if encoderInstalled(ctx, consts.ACodecToFFACodec[c]) {
				desiredA = c
				break
			}
		}
		if desiredA == "" {
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
		logger.Pl.DevE("Got wrong type %T for video codec map", cMapInterface)
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
		logger.Pl.DevE("Got wrong type %T for audio codec map", cMapInterface)
		return
	}
	codec := cMap[currentACodec]

	// Normalize.
	codec = strings.ToLower(codec)
	codec = strings.ReplaceAll(codec, " ", "")
	codec = strings.ReplaceAll(codec, ".", "")

	return codec
}

// splitArgs splits user input arguments in shell style.
func splitArgs(s string) ([]string, error) {
	var args []string
	var arg strings.Builder
	var quote rune
	escaped := false
	started := false

	for _, r := range s {
		if escaped {
			arg.WriteRune(r)
			escaped = false
			started = true
			continue
		}

		if r == '\\' && quote != '\'' {
			escaped = true
			started = true
			continue
		}

		if quote != 0 {
			if r == quote {
				quote = 0
			} else {
				arg.WriteRune(r)
			}
			continue
		}

		switch {
		case r == '\'' || r == '"':
			quote = r
			started = true
		case unicode.IsSpace(r):
			if started {
				args = append(args, arg.String())
				arg.Reset()
				started = false
			}
		default:
			arg.WriteRune(r)
			started = true
		}
	}

	if escaped {
		return nil, errors.New("trailing escape character")
	}
	if quote != 0 {
		return nil, errors.New("unclosed quote")
	}
	if started {
		args = append(args, arg.String())
	}

	return args, nil
}
