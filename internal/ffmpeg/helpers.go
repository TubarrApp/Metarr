package ffmpeg

import (
	"metarr/internal/abstractions"
	"metarr/internal/domain/keys"
	"metarr/internal/domain/logger"
	"strings"
)

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
