// Package ffprobe helps determine the metadata already present in a video file.
//
// This is used to determine if a video file should be encoded.
package ffprobe

import (
	"cmp"
	"fmt"
	"math"
	"metarr/internal/abstractions"
	"metarr/internal/domain/keys"
	"metarr/internal/domain/logger"
	"metarr/internal/parsing"
	"slices"
	"strconv"
	"strings"

	"github.com/TubarrApp/gocommon/sharedconsts"
	"github.com/TubarrApp/gocommon/sharedmodels"
	"github.com/TubarrApp/gocommon/sharedparsing"
	"github.com/TubarrApp/gocommon/sharedtags"
	"github.com/TubarrApp/gocommon/sharedvalidation"
)

// fullHDPixels is the pixel count of a 1080p frame, which the compression thresholds are set for.
const fullHDPixels = 1920 * 1080

// ffprobeFormat contains metadata tags.
type ffprobeFormat struct {
	Tags ffprobeTags `json:"tags"`
}

// ffprobeProgram contains a program's metadata tags.
type ffprobeProgram struct {
	Tags ffprobeTags `json:"tags"`
}

// ffprobeOutput contains top-level FFprobe output.
type ffprobeOutput struct {
	Streams  []ffprobeStream  `json:"streams"`
	Programs []ffprobeProgram `json:"programs"`
	Format   ffprobeFormat    `json:"format"`
}

// ffprobeTags is a map of metadata key-value pairs.
//
// Different container formats use different key names (e.g., "artist" vs "ARTIST" vs "WM/AlbumArtist").
type ffprobeTags map[string]string

type ffprobeStream struct {
	Index       int         `json:"index"`
	CodecType   string      `json:"codec_type"`
	CodecName   string      `json:"codec_name"`
	Tags        ffprobeTags `json:"tags"`
	Disposition struct {
		AttachedPic int `json:"attached_pic"`
	} `json:"disposition"`
}

// getDiffMapForFiletype pairs each tag FFmpeg would write into the file with the value already in the video.
func getDiffMapForFiletype(e string, tags map[string]string, ffData ffprobeOutput) tagDiffMap {
	probed := ffData.containerTags(e)
	diffs := make(tagDiffMap, len(tags))
	for key, value := range tags {
		// Skip fields the container can't hold.
		tag := parsing.GetContainerKeys(key, e)
		if tag == "" {
			continue
		}

		// Look the tag up under the name FFprobe reports it as.
		existing := strings.TrimSpace(probed.get(probeKey(tag, e)))
		value = strings.TrimSpace(value)

		// Dates only compare their date part.
		if slices.Contains(sharedtags.DateKeys, key) {
			existing = getDatePart(existing)
			value = getDatePart(value)
		}
		diffs[tag] = tagDiff{existing: existing, new: value}
	}
	return diffs
}

// probeKey returns the name FFprobe reports a written tag under, where it differs from the tag.
func probeKey(tag, e string) string {
	switch strings.ToLower(e) {
	case sharedconsts.ExtASF,
		sharedconsts.ExtWMV:
		switch tag {
		case sharedtags.ASFAlbumArtist:
			return sharedtags.FFProbeAlbumArtist
		case sharedtags.ASFAlbumTitle:
			return sharedtags.FFProbeAlbum
		case sharedtags.ASFAuthor:
			return sharedtags.FFProbeArtist
		case sharedtags.ASFComposer:
			return sharedtags.FFProbeComposer
		case sharedtags.ASFDescription:
			return sharedtags.FFProbeComment
		case sharedtags.ASFGenre:
			return sharedtags.FFProbeGenre
		case sharedtags.ASFLanguage:
			return sharedtags.FFProbeLanguage
		case sharedtags.ASFPartOfSet:
			return sharedtags.FFProbeDisc
		case sharedtags.ASFPublisher:
			return sharedtags.FFProbePublisher
		case sharedtags.ASFTrackNumber:
			return sharedtags.FFProbeTrack
		}

	case sharedconsts.ExtAVI:
		switch tag {
		case sharedtags.AVIArtist:
			return sharedtags.FFProbeArtist
		case sharedtags.AVIComment:
			return sharedtags.FFProbeComment
		case sharedtags.AVICopyright:
			return sharedtags.FFProbeCopyright
		case sharedtags.AVIDateCreated:
			return sharedtags.FFProbeDate
		case sharedtags.AVIGenre:
			return sharedtags.FFProbeGenre
		case sharedtags.AVILanguage:
			return sharedtags.FFProbeLanguage
		case sharedtags.AVIProduct:
			return sharedtags.FFProbeProduct
		case sharedtags.AVISubject:
			return sharedtags.FFProbeSubject
		case sharedtags.AVITitle:
			return sharedtags.FFProbeTitle
		case sharedtags.AVITrackNumber:
			return sharedtags.FFProbeTrack
		}

	case sharedconsts.ExtMKV,
		sharedconsts.ExtWEBM:
		switch tag {
		case sharedtags.MatroskaLeadPerformer:
			return sharedtags.FFProbePerformer
		case sharedtags.MatroskaPartNumber:
			return sharedtags.FFProbeTrack
		}

	case sharedconsts.ExtOGM,
		sharedconsts.ExtOGV:
		switch tag {
		case sharedtags.OggAlbumArtist:
			return sharedtags.FFProbeAlbumArtist
		case sharedtags.OggDescription:
			return sharedtags.FFProbeComment
		case sharedtags.OggDiscNumber:
			return sharedtags.FFProbeDisc
		case sharedtags.OggTrackNumber:
			return sharedtags.FFProbeTrack
		}
	}
	return tag
}

// containerTags returns the tags FFprobe reports a container's metadata in.
func (o ffprobeOutput) containerTags(e string) ffprobeTags {
	switch strings.ToLower(e) {
	case sharedconsts.ExtMTS,
		sharedconsts.ExtTS:
		// MPEG-TS keeps its service tags on the program.
		if len(o.Programs) > 0 {
			return o.Programs[0].Tags
		}

	case sharedconsts.ExtOGM,
		sharedconsts.ExtOGV:
		// Ogg keeps its tags on each stream.
		if len(o.Streams) > 0 {
			return o.Streams[0].Tags
		}
	}
	return o.Format.Tags
}

// getDatePart safely extracts the date part before 'T' if it exists.
func getDatePart(timeStr string) string {
	timeStr = strings.TrimSpace(timeStr)
	if beforeT, _, _ := strings.Cut(timeStr, "T"); beforeT != "" {
		return beforeT
	}
	return timeStr
}

// printArray provides a simple print of metadata captured by FFprobe.
func printArray(s []string) {
	str := strings.Join(s, ", ")
	logger.Pl.D(4, "FFprobe captured %s", str)
}

// get grabs key based on casing.
func (tags ffprobeTags) get(key string) string {
	// Direct match.
	if k, exists := tags[key]; exists {
		return k
	}

	// Try variants.
	if k, exists := tags[strings.ToLower(key)]; exists {
		return k
	}
	if k, exists := tags[strings.ToUpper(key)]; exists {
		return k
	}
	if k, exists := tags[strings.ToTitle(key)]; exists {
		return k
	}

	// Special WM/ case attempts:
	// If key contains "WM/", try the part after it with case variants.
	_, after, found := strings.Cut(key, "WM/")
	if found && after != "" {
		// Try direct match without recursion.
		if k, exists := tags[after]; exists {
			return k
		}
		if k, exists := tags[strings.ToLower(after)]; exists {
			return k
		}
		if k, exists := tags[strings.ToUpper(after)]; exists {
			return k
		}
		if k, exists := tags[strings.ToTitle(after)]; exists {
			return k
		}
	}
	// Also try adding "WM/" prefix if not already present.
	if !found { // WM/ was not in the original key.
		wmKey := "WM/" + key
		if k, exists := tags[wmKey]; exists {
			return k
		}
		if k, exists := tags[strings.ToLower(wmKey)]; exists {
			return k
		}
		if k, exists := tags[strings.ToUpper(wmKey)]; exists {
			return k
		}
		if k, exists := tags[strings.ToTitle(wmKey)]; exists {
			return k
		}
	}

	return ""
}

// getVideoCompressionLevel determines the compression level of a video stream from its bitrate, using the file's less the audio's when the stream has none.
func getVideoCompressionLevel(video *ffprobeCodecOutputStream, fileBitRate, audioBitRate, inputFile string) (sharedconsts.CompressionLevel, error) {
	// Validate video dimensions.
	if video.Width <= 0 || video.Height <= 0 {
		return sharedconsts.CompressionBlank, fmt.Errorf("invalid video dimensions: %dx%d", video.Width, video.Height)
	}

	// Parse average frame rate.
	numerator, denominator, ok := strings.Cut(video.AvgFrameRate, "/")
	if !ok {
		return sharedconsts.CompressionBlank, fmt.Errorf("invalid average frame rate %q", video.AvgFrameRate)
	}

	num, err := strconv.ParseFloat(numerator, 64)
	if err != nil {
		return sharedconsts.CompressionBlank, fmt.Errorf("invalid average frame rate %q: %w", video.AvgFrameRate, err)
	}

	den, err := strconv.ParseFloat(denominator, 64)
	if err != nil || den <= 0 {
		return sharedconsts.CompressionBlank, fmt.Errorf("invalid average frame rate %q", video.AvgFrameRate)
	}

	frameRate := num / den
	if frameRate <= 0 {
		return sharedconsts.CompressionBlank, fmt.Errorf("invalid average frame rate %q", video.AvgFrameRate)
	}

	// Parse video bitrate, else the whole file's.
	bitRate, err := strconv.ParseFloat(cmp.Or(video.BitRate, fileBitRate), 64)
	if err != nil || bitRate <= 0 {
		return sharedconsts.CompressionBlank, fmt.Errorf("no video bitrate (stream %q, file %q)", video.BitRate, fileBitRate)
	}

	// Take the audio out of the file's bitrate.
	if video.BitRate == "" {
		if audioRate, err := strconv.ParseFloat(audioBitRate, 64); err == nil && audioRate < bitRate {
			bitRate -= audioRate
		}
	}

	// Count the pixels and frames by the bits they need: 4K needs about 2.8 times the bits of 1080p, and 60 fps 1.5 times those of 30 fps.
	pixels := fullHDPixels * math.Pow(float64(video.Width)*float64(video.Height)/fullHDPixels, 0.75)
	frames := 30 * math.Pow(frameRate/30, 0.585)

	// Scale the thresholds to how many bits the codec needs, giving the bitrates high and medium compression run up to.
	codec, _ := sharedvalidation.ValidateVideoCodec(video.CodecName)
	scale := pixels * frames * videoBitsScale(codec)
	highBelow := sharedconsts.BPPPFHighCompressionThreshold * scale
	mediumBelow := sharedconsts.BPPPFMediumCompressionThreshold * scale

	// The user's thresholds for the codec, from its resolution line or else its plain line, replace them.
	byCodec, _ := sharedparsing.ParseVideoCompressionThresholds(abstractions.GetStringSlice(keys.VideoCompressionThresholds))
	set, ok := byCodec[codec][sharedparsing.ResolutionTier(min(video.Width, video.Height))]
	if !ok {
		set = byCodec[codec][0]
	}
	highBelow, mediumBelow = withThresholds(highBelow, mediumBelow, set)

	// Determine compression level.
	compressionLevel := "low compression"
	var compression sharedconsts.CompressionLevel
	switch {
	case bitRate < highBelow:
		compression = sharedconsts.CompressionHigh
		compressionLevel = "high compression"
	case bitRate < mediumBelow:
		compression = sharedconsts.CompressionMedium
		compressionLevel = "average compression"
	default:
		compression = sharedconsts.CompressionLow
	}

	logger.Pl.I("Video '%s''s %s stream bit rate (%.0f), dimensions (%dx%d), and frame rate (%f) determine the video has %s, as high compression is below %.0f and medium below %.0f.",
		inputFile, video.CodecName, bitRate, video.Width, video.Height, frameRate, compressionLevel, highBelow, mediumBelow)
	return compression, nil
}

// withThresholds puts the user's thresholds in place of the ones they give, moving any other so the two don't cross.
func withThresholds(highBelow, mediumBelow float64, set sharedmodels.CompressionThresholds) (float64, float64) {
	if set.HighBelow > 0 {
		highBelow = float64(set.HighBelow)
		mediumBelow = max(mediumBelow, highBelow)
	}
	if set.MediumBelow > 0 {
		mediumBelow = float64(set.MediumBelow)
		highBelow = min(highBelow, mediumBelow)
	}
	return highBelow, mediumBelow
}

// videoBitsScale returns how many bits a video codec needs for the same picture, compared with H.264.
func videoBitsScale(codec string) float64 {
	switch codec {
	case sharedconsts.VCodecAV1:
		return 0.5
	case sharedconsts.VCodecHEVC, sharedconsts.VCodecVP9:
		return 0.6
	case sharedconsts.VCodecMPEG2:
		return 2
	default:
		return 1
	}
}

// getAudioCompressionLevel determines the compression level of an audio stream.
func getAudioCompressionLevel(audio *ffprobeCodecOutputStream, inputFile string) (sharedconsts.CompressionLevel, error) {
	if audio == nil {
		return sharedconsts.CompressionBlank, fmt.Errorf("audio stream is nil")
	}

	// Validate audio channels.
	if audio.Channels <= 0 {
		return sharedconsts.CompressionBlank, fmt.Errorf("invalid audio channel count %d", audio.Channels)
	}

	// Parse audio bitrate.
	bitRate, err := strconv.ParseFloat(audio.BitRate, 64)
	if err != nil || bitRate <= 0 {
		return sharedconsts.CompressionBlank, fmt.Errorf("invalid audio bitrate %q", audio.BitRate)
	}

	// Scale the per-channel thresholds to the channel count and how many bits the codec needs.
	codec, _ := sharedvalidation.ValidateAudioCodec(audio.CodecName)
	scale := float64(audio.Channels) * audioBitsScale(codec)
	highBelow := sharedconsts.AudioBPSPCHighCompressionThreshold * scale
	mediumBelow := sharedconsts.AudioBPSPCMediumCompressionThreshold * scale

	// The user's thresholds for the codec replace them.
	byCodec, _ := sharedparsing.ParseAudioCompressionThresholds(abstractions.GetStringSlice(keys.AudioCompressionThresholds))
	highBelow, mediumBelow = withThresholds(highBelow, mediumBelow, byCodec[codec])

	// Determine compression level.
	compressionLevel := "low compression"
	var compression sharedconsts.CompressionLevel
	switch {
	case bitRate < highBelow:
		compression = sharedconsts.CompressionHigh
		compressionLevel = "high compression"
	case bitRate < mediumBelow:
		compression = sharedconsts.CompressionMedium
		compressionLevel = "average compression"
	default:
		compression = sharedconsts.CompressionLow
	}

	logger.Pl.I("Video '%s's %s audio stream bit rate (%.0f) and channels (%d) determine the audio has %s, as high compression is below %.0f and medium below %.0f.",
		inputFile, audio.CodecName, bitRate, audio.Channels, compressionLevel, highBelow, mediumBelow)
	return compression, nil
}

// audioBitsScale returns how many bits an audio codec needs for the same sound, compared with MP3.
func audioBitsScale(codec string) float64 {
	switch codec {
	case sharedconsts.ACodecOpus:
		return 0.6
	case sharedconsts.ACodecAAC, sharedconsts.ACodecEAC3, sharedconsts.ACodecVorbis:
		return 0.75
	case sharedconsts.ACodecDTS, sharedconsts.ACodecMP2:
		return 1.5
	default:
		return 1
	}
}
