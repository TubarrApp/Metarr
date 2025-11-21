package parsing

import (
	"metarr/internal/domain/consts"
	"metarr/internal/domain/logger"
	"strings"

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

	result, anyValid := mtp.fillMetaTemplateTagRecursive(inputStr, j)
	if !anyValid {
		return inputStr, true // No valid replacements, return unchanged.
	}
	return result, true
}

// fillMetaTemplateTagRecursive fills in all templating tags in the string.
func (mtp *MetaTemplateParser) fillMetaTemplateTagRecursive(inputStr string, j map[string]any) (result string, anyReplaced bool) {
	openTagIdx := strings.Index(inputStr, "{{")
	closeTagIdx := strings.Index(inputStr, "}}")
	if openTagIdx < 0 || closeTagIdx < 0 || closeTagIdx < openTagIdx {
		return inputStr, false
	}

	// Bounds check.
	if openTagIdx+2 > len(inputStr) || closeTagIdx+2 > len(inputStr) {
		return inputStr, false
	}

	// Ensure content between open and close tags.
	if openTagIdx+2 > closeTagIdx {
		return inputStr, false
	}

	tagContent := inputStr[openTagIdx+2 : closeTagIdx]
	replacement, succeeded := mtp.fillTag(tagContent, j)

	// If fillTag failed, return original with no replacement flag.
	if !succeeded {
		return inputStr, false
	}

	tag := inputStr[:openTagIdx] + replacement + inputStr[closeTagIdx+2:]
	recursiveResult, _ := mtp.fillMetaTemplateTagRecursive(tag, j)
	return recursiveResult, true
}

// fillTag finds the matching string for a given template tag.
//
// Returns the replacement string and whether it succeeded.
func (mtp *MetaTemplateParser) fillTag(template string, j map[string]any) (result string, success bool) {
	if template == "" || j[template] == nil {
		return "", false
	}
	// Search map for template key.
	for k, v := range j {
		if k == template {
			strVal, ok := v.(string)
			if !ok || strVal == "" {
				logger.Pl.E("JSON key %v does not contain a valid string value (variable is of type %T), not parsing (file %q)", template, v, mtp.jsonFileName)
				return "", false
			}
			return strVal, true
		}
	}
	logger.Pl.D(1, "Value for JSON key %q does not exist in file %q", template, mtp.jsonFileName)
	return "", false
}

// GetContainerKeys returns valid tag names for the given key and container type.
func GetContainerKeys(key, extension string) string {
	switch extension {
	case consts.ExtWMV,
		consts.ExtASF:
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

	case consts.ExtAVI:
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

	case consts.ExtFLV:
		// FLV uses lowercase tags.
		switch key {
		case sharedtags.JDate:
			return sharedtags.FLVCreationDate
		}

	case consts.Ext3GP,
		consts.Ext3G2,
		consts.ExtF4V,
		consts.ExtM4V,
		consts.ExtMOV,
		consts.ExtMP4:
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

	case consts.ExtMKV,
		consts.ExtWEBM:
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

	case consts.ExtMTS,
		consts.ExtTS:
		// MPEG-TS uses specific service tags.
		switch key {
		case sharedtags.JArtist:
			return sharedtags.TSServiceProvider

		case sharedtags.JTitle:
			return sharedtags.TSServiceName

		}

	case consts.ExtOGM,
		consts.ExtOGV:
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

	case consts.ExtRM,
		consts.ExtRMVB:
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
