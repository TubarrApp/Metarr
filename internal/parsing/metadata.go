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

// GetContainerKeys returns valid tag names for the given key and container type.
func GetContainerKeys(key, extension string) string {
	switch extension {
	case sharedconsts.ExtWMV,
		sharedconsts.ExtASF:
		// ASF uses TitleCase and WM/ prefixes.
		switch key {

		case sharedtags.JArtist:
			return sharedtags.ASFArtist

		case sharedtags.JComposer:
			return sharedtags.ASFComposer

		case sharedtags.JDirector:
			return sharedtags.ASFDirector

		case sharedtags.JProducer:
			return sharedtags.ASFProducer

		case sharedtags.JSubtitle:
			return sharedtags.ASFSubtitle

		case sharedtags.JDescription:
			return sharedtags.ASFSubTitleDescription

		case sharedtags.JDate:
			return sharedtags.ASFEncodingTime

		case sharedtags.JTitle:
			return sharedtags.ASFTitle

		case sharedtags.JYear:
			return sharedtags.ASFYear
		}

	case sharedconsts.ExtAVI:
		// AVI uses RIFF INFO tags (4-character codes).
		switch key {
		case sharedtags.JLongDesc:
			return sharedtags.AVIComments

		case sharedtags.JActor:
			return sharedtags.AVIStar

		case sharedtags.JArtist:
			return sharedtags.AVIArtist

		case sharedtags.JDescription:
			return sharedtags.AVIComment

		case sharedtags.JReleaseDate:
			return sharedtags.AVIDateCreated

		case sharedtags.JProducer:
			return sharedtags.AVIEngineer

		case sharedtags.JSynopsis:
			return sharedtags.AVISubject

		case sharedtags.JTitle:
			return sharedtags.AVITitle

		case sharedtags.JYear:
			return sharedtags.AVIYear
		}

	case sharedconsts.ExtFLV:
		// FLV uses lowercase tags.
		switch key {
		case sharedtags.JDate:
			return sharedtags.FLVCreationDate
		}

	case sharedconsts.Ext3GP,
		sharedconsts.Ext3G2,
		sharedconsts.ExtF4V,
		sharedconsts.ExtM4V,
		sharedconsts.ExtMOV,
		sharedconsts.ExtMP4:
		// ISOBMFF uses lowercase tags.
		switch key {
		case sharedtags.JArtist:
			return sharedtags.ISOArtist

		case sharedtags.JComment:
			return sharedtags.ISOComment

		case sharedtags.JComposer:
			return sharedtags.ISOComposer

		case sharedtags.JCreationTime:
			return sharedtags.ISOCreationTime

		case sharedtags.JDate:
			return sharedtags.ISODate

		case sharedtags.JDescription:
			return sharedtags.ISODescription

		case sharedtags.JSynopsis:
			return sharedtags.ISOSynopsis

		case sharedtags.JTitle:
			return sharedtags.ISOTitle
		}

	case sharedconsts.ExtMKV,
		sharedconsts.ExtWEBM:
		// Matroska uses UPPERCASE tags.
		switch key {
		case sharedtags.JArtist:
			return sharedtags.MatroskaArtist

		case sharedtags.JComposer:
			return sharedtags.MatroskaComposer

		case sharedtags.JCreationTime:
			return sharedtags.MatroskaDateEncoded

		case sharedtags.JReleaseDate:
			return sharedtags.MatroskaDateReleased

		case sharedtags.JDescription:
			return sharedtags.MatroskaDescription

		case sharedtags.JDirector:
			return sharedtags.MatroskaDirector

		case sharedtags.JActor:
			return sharedtags.MatroskaLeadPerformer

		case sharedtags.JPerformer:
			return sharedtags.MatroskaPerformer

		case sharedtags.JProducer:
			return sharedtags.MatroskaProducer

		case sharedtags.JSubtitle:
			return sharedtags.MatroskaSubject

		case sharedtags.JSummary:
			return sharedtags.MatroskaSummary

		case sharedtags.JSynopsis:
			return sharedtags.MatroskaSynopsis

		case sharedtags.JTitle:
			return sharedtags.MatroskaTitle
		}

	case sharedconsts.ExtMTS,
		sharedconsts.ExtTS:
		// MPEG-TS uses specific service tags.
		switch key {
		case sharedtags.JArtist:
			return sharedtags.TSServiceProvider

		case sharedtags.JTitle:
			return sharedtags.TSServiceName

		}

	case sharedconsts.ExtOGM,
		sharedconsts.ExtOGV:
		// Ogg uses UPPERCASE Vorbis comments.
		switch key {
		case sharedtags.JArtist:
			return sharedtags.OggArtist

		case sharedtags.JComposer:
			return sharedtags.OggComposer

		case sharedtags.JDate:
			return sharedtags.OggDate

		case sharedtags.JDescription:
			return sharedtags.OggDescription

		case sharedtags.JPerformer:
			return sharedtags.OggPerformer

		case sharedtags.JSummary:
			return sharedtags.OggSummary

		case sharedtags.JTitle:
			return sharedtags.OggTitle
		}

	case sharedconsts.ExtRM,
		sharedconsts.ExtRMVB:
		// RealMedia uses TitleCase.
		switch key {
		case sharedtags.JAuthor:
			return sharedtags.RMAuthor

		case sharedtags.JDescription:
			return sharedtags.RMComment

		case sharedtags.JTitle:
			return sharedtags.RMTitle
		}

	}
	return ""
}
