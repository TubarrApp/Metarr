// Package ffprobe helps determine the metadata already present in a video file.
//
// This is used to determine if a video file should be encoded.
package ffprobe

import (
	"metarr/internal/domain/logger"
	"metarr/internal/parsing"
	"slices"
	"strings"

	"github.com/TubarrApp/gocommon/sharedconsts"
	"github.com/TubarrApp/gocommon/sharedtags"
)

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
	logger.Pl.I("FFprobe captured %s", str)
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
