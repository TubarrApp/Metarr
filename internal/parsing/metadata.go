package parsing

import (
	"encoding/json"
	"metarr/internal/domain/logger"
	"strconv"
	"strings"

	"github.com/TubarrApp/gocommon/sharedconsts"
	"github.com/TubarrApp/gocommon/sharedtags"
)

// MetaTemplateParser is used for parsing meta template tags into filled strings.
type MetaTemplateParser struct {
	jsonFileName string
}

// NewMetaTemplateParser grants access to metadata template parsing functions.
func NewMetaTemplateParser(jsonFileName string) *MetaTemplateParser {
	return &MetaTemplateParser{
		jsonFileName: jsonFileName,
	}
}

// FillMetaTemplateTag returns the original string filled with inferred data.
//
// If no valid substitutions occur, returns the original input string.
func (mtp *MetaTemplateParser) FillMetaTemplateTag(inputStr string, j map[string]any) (result string, isTemplate bool) {
	openTagIdx := strings.Index(inputStr, "{{")
	closeTagIdx := strings.Index(inputStr, "}}")
	if openTagIdx < 0 || closeTagIdx < 0 || closeTagIdx < openTagIdx {
		return inputStr, false
	}

	return mtp.fillMetaTemplateTags(inputStr, j), true
}

// fillMetaTemplateTags fills every tag it can in one pass, leaving unfillable tags as written.
//
// Filled values aren't scanned again, so a value containing "{{...}}" stays literal.
func (mtp *MetaTemplateParser) fillMetaTemplateTags(s string, j map[string]any) string {
	var b strings.Builder
	for {
		open := strings.Index(s, "{{")
		if open < 0 {
			break
		}
		closeRel := strings.Index(s[open+2:], "}}")
		if closeRel < 0 {
			break
		}
		end := open + 2 + closeRel

		b.WriteString(s[:open])
		if val, ok := mtp.fillTag(strings.TrimSpace(s[open+2:end]), j); ok {
			b.WriteString(val)
		} else {
			b.WriteString(s[open : end+2])
		}
		s = s[end+2:]
	}
	b.WriteString(s)
	return b.String()
}

// fillTag returns the metadata value for a template tag, and whether it has a usable value.
func (mtp *MetaTemplateParser) fillTag(template string, j map[string]any) (result string, success bool) {
	v, found := j[template]
	if template == "" || !found || v == nil {
		logger.Pl.D(1, "Value for JSON key %q does not exist in file %q", template, mtp.jsonFileName)
		return "", false
	}
	switch v := v.(type) {
	case string:
		result = v
	case float64:
		result = strconv.FormatFloat(v, 'f', -1, 64)
	case json.Number:
		result = v.String()
	case bool:
		result = strconv.FormatBool(v)
	default:
		logger.Pl.E("JSON key %v does not contain a text or number value (variable is of type %T), not parsing (file %q)", template, v, mtp.jsonFileName)
		return "", false
	}
	if result == "" {
		logger.Pl.E("JSON key %v is empty, not parsing (file %q)", template, mtp.jsonFileName)
		return "", false
	}
	return result, true
}

// GetContainerKeys returns the tag a metadata field is written as in a container, or "" if the container can't hold it.
func GetContainerKeys(key, extension string) string {
	switch strings.ToLower(extension) {
	case sharedconsts.ExtWMV,
		sharedconsts.ExtASF:
		// ASF takes any key, using the Windows Media names where they exist.
		switch key {
		case sharedtags.JAlbum:
			return sharedtags.ASFAlbumTitle

		case sharedtags.JAlbumArtist:
			return sharedtags.ASFAlbumArtist

		case sharedtags.JArtist:
			return sharedtags.ASFAuthor

		case sharedtags.JAuthor:
			return "" // The artist is written as the author.

		case sharedtags.JComment:
			return sharedtags.ASFDescription

		case sharedtags.JComposer:
			return sharedtags.ASFComposer

		case sharedtags.JCopyright:
			return sharedtags.ASFCopyright

		case sharedtags.JDescription:
			return sharedtags.ASFSubTitleDescription

		case sharedtags.JDirector:
			return sharedtags.ASFDirector

		case sharedtags.JDiscNumber:
			return sharedtags.ASFPartOfSet

		case sharedtags.JGenre:
			return sharedtags.ASFGenre

		case sharedtags.JLanguage:
			return sharedtags.ASFLanguage

		case sharedtags.JProducer:
			return sharedtags.ASFProducer

		case sharedtags.JPublisher:
			return sharedtags.ASFPublisher

		case sharedtags.JReleaseDate:
			return sharedtags.ASFOriginalReleaseTime

		case sharedtags.JSubtitle:
			return sharedtags.ASFSubtitle

		case sharedtags.JTitle:
			return sharedtags.ASFTitle

		case sharedtags.JTrackNumber:
			return sharedtags.ASFTrackNumber

		case sharedtags.JWriter:
			return sharedtags.ASFWriter

		case sharedtags.JYear:
			return sharedtags.ASFYear

		default:
			return key
		}

	case sharedconsts.ExtAVI:
		// AVI only takes its RIFF INFO tags (4-character codes).
		switch key {
		case sharedtags.JAlbum:
			return sharedtags.AVIProduct

		case sharedtags.JArtist:
			return sharedtags.AVIArtist

		case sharedtags.JCopyright:
			return sharedtags.AVICopyright

		case sharedtags.JDescription:
			return sharedtags.AVIComment

		case sharedtags.JGenre:
			return sharedtags.AVIGenre

		case sharedtags.JKeywords:
			return sharedtags.AVIKeywords

		case sharedtags.JLanguage:
			return sharedtags.AVILanguage

		case sharedtags.JProducer:
			return sharedtags.AVIEngineer

		case sharedtags.JReleaseDate:
			return sharedtags.AVIDateCreated

		case sharedtags.JSynopsis:
			return sharedtags.AVISubject

		case sharedtags.JTitle:
			return sharedtags.AVITitle

		case sharedtags.JTrackNumber:
			return sharedtags.AVITrackNumber
		}

	case sharedconsts.ExtFLV:
		// FLV takes any key.
		switch key {
		case sharedtags.JDate:
			return sharedtags.FLVCreationDate

		default:
			return key
		}

	case sharedconsts.ExtF4V,
		sharedconsts.ExtM4V,
		sharedconsts.ExtMP4:
		// MP4 only takes its iTunes tags.
		switch key {
		case sharedtags.JAlbum:
			return sharedtags.MP4Album

		case sharedtags.JAlbumArtist:
			return sharedtags.MP4AlbumArtist

		case sharedtags.JArtist:
			return sharedtags.MP4Artist

		case sharedtags.JComment:
			return sharedtags.MP4Comment

		case sharedtags.JComposer:
			return sharedtags.MP4Composer

		case sharedtags.JCopyright:
			return sharedtags.MP4Copyright

		case sharedtags.JCreationTime:
			return sharedtags.MP4CreationTime

		case sharedtags.JDate:
			return sharedtags.MP4Date

		case sharedtags.JDescription:
			return sharedtags.MP4Description

		case sharedtags.JDiscNumber:
			return sharedtags.MP4Disc

		case sharedtags.JEpisodeID:
			return sharedtags.MP4EpisodeID

		case sharedtags.JEpisodeSort:
			return sharedtags.MP4EpisodeSort

		case sharedtags.JGenre:
			return sharedtags.MP4Genre

		case sharedtags.JHDVideo:
			return sharedtags.MP4HDVideo

		case sharedtags.JKeywords:
			return sharedtags.MP4Keywords

		case sharedtags.JNetwork:
			return sharedtags.MP4Network

		case sharedtags.JSeasonNumber:
			return sharedtags.MP4SeasonNumber

		case sharedtags.JShow:
			return sharedtags.MP4Show

		case sharedtags.JSynopsis:
			return sharedtags.MP4Synopsis

		case sharedtags.JTitle:
			return sharedtags.MP4Title

		case sharedtags.JTrackNumber:
			return sharedtags.MP4Track
		}

	case sharedconsts.ExtMOV:
		// MOV only takes its QuickTime tags. The author and keywords are left out, as FFprobe can't read them back.
		switch key {
		case sharedtags.JAlbum:
			return sharedtags.MOVAlbum

		case sharedtags.JArtist:
			return sharedtags.MOVArtist

		case sharedtags.JCopyright:
			return sharedtags.MOVCopyright

		case sharedtags.JCreationTime:
			return sharedtags.MOVCreationTime

		case sharedtags.JDate:
			return sharedtags.MOVDate

		case sharedtags.JDescription:
			return sharedtags.MOVComment

		case sharedtags.JGenre:
			return sharedtags.MOVGenre

		case sharedtags.JTitle:
			return sharedtags.MOVTitle
		}

	case sharedconsts.Ext3GP,
		sharedconsts.Ext3G2:
		// 3GP only takes its 3GPP tags.
		switch key {
		case sharedtags.JAlbum:
			return sharedtags.ThreeGPAlbum

		case sharedtags.JArtist:
			return sharedtags.ThreeGPArtist

		case sharedtags.JAuthor:
			return sharedtags.ThreeGPAuthor

		case sharedtags.JCopyright:
			return sharedtags.ThreeGPCopyright

		case sharedtags.JCreationTime:
			return sharedtags.ThreeGPCreationTime

		case sharedtags.JDescription:
			return sharedtags.ThreeGPComment

		case sharedtags.JGenre:
			return sharedtags.ThreeGPGenre

		case sharedtags.JTitle:
			return sharedtags.ThreeGPTitle

		case sharedtags.JYear:
			return sharedtags.ThreeGPDate
		}

	case sharedconsts.ExtMKV,
		sharedconsts.ExtWEBM:
		// Matroska takes any key, using the Matroska names where they exist.
		switch key {
		case sharedtags.JActor:
			return sharedtags.MatroskaActor

		case sharedtags.JArtist:
			return sharedtags.MatroskaArtist

		case sharedtags.JComment:
			return sharedtags.MatroskaComment

		case sharedtags.JComposer:
			return sharedtags.MatroskaComposer

		case sharedtags.JCopyright:
			return sharedtags.MatroskaCopyright

		case sharedtags.JCreationTime:
			return sharedtags.MatroskaDateEncoded

		case sharedtags.JDescription:
			return sharedtags.MatroskaDescription

		case sharedtags.JDirector:
			return sharedtags.MatroskaDirector

		case sharedtags.JGenre:
			return sharedtags.MatroskaGenre

		case sharedtags.JKeywords:
			return sharedtags.MatroskaKeywords

		case sharedtags.JPerformer:
			return sharedtags.MatroskaLeadPerformer

		case sharedtags.JProducer:
			return sharedtags.MatroskaProducer

		case sharedtags.JPublisher:
			return sharedtags.MatroskaPublisher

		case sharedtags.JReleaseDate:
			return sharedtags.MatroskaDateReleased

		case sharedtags.JStudio:
			return sharedtags.MatroskaProductionStudio

		case sharedtags.JSubtitle:
			return sharedtags.MatroskaSubtitle

		case sharedtags.JSummary:
			return sharedtags.MatroskaSummary

		case sharedtags.JSynopsis:
			return sharedtags.MatroskaSynopsis

		case sharedtags.JTitle:
			return sharedtags.MatroskaTitle

		case sharedtags.JTrackNumber:
			return sharedtags.MatroskaPartNumber

		case sharedtags.JWebpageURL:
			return sharedtags.MatroskaURL

		case sharedtags.JWriter:
			return sharedtags.MatroskaWrittenBy

		default:
			return key
		}

	case sharedconsts.ExtMTS,
		sharedconsts.ExtTS:
		// MPEG-TS only takes its service tags.
		switch key {
		case sharedtags.JArtist:
			return sharedtags.TSServiceProvider

		case sharedtags.JTitle:
			return sharedtags.TSServiceName
		}

	case sharedconsts.ExtOGM,
		sharedconsts.ExtOGV:
		// Ogg takes any key, using the Vorbis comment names where they exist.
		switch key {
		case sharedtags.JAlbum:
			return sharedtags.OggAlbum

		case sharedtags.JAlbumArtist:
			return sharedtags.OggAlbumArtist

		case sharedtags.JArtist:
			return sharedtags.OggArtist

		case sharedtags.JComment:
			return "" // FFmpeg writes a comment as the description.

		case sharedtags.JComposer:
			return sharedtags.OggComposer

		case sharedtags.JCopyright:
			return sharedtags.OggCopyright

		case sharedtags.JDate:
			return sharedtags.OggDate

		case sharedtags.JDescription:
			return sharedtags.OggDescription

		case sharedtags.JDiscNumber:
			return sharedtags.OggDiscNumber

		case sharedtags.JGenre:
			return sharedtags.OggGenre

		case sharedtags.JPerformer:
			return sharedtags.OggPerformer

		case sharedtags.JPublisher:
			return sharedtags.OggOrganization

		case sharedtags.JSummary:
			return sharedtags.OggSummary

		case sharedtags.JTitle:
			return sharedtags.OggTitle

		case sharedtags.JTrackNumber:
			return sharedtags.OggTrackNumber

		default:
			return key
		}

	case sharedconsts.ExtRM,
		sharedconsts.ExtRMVB:
		// RealMedia only takes its four tags.
		switch key {
		case sharedtags.JAuthor:
			return sharedtags.RMAuthor

		case sharedtags.JCopyright:
			return sharedtags.RMCopyright

		case sharedtags.JDescription:
			return sharedtags.RMComment

		case sharedtags.JTitle:
			return sharedtags.RMTitle
		}
	}
	return ""
}
