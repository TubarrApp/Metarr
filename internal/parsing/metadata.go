package parsing

import (
	"metarr/internal/domain/consts"
	"metarr/internal/domain/logger"
	"strings"
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
	case consts.Ext3GP,
		consts.Ext3G2,
		consts.ExtF4V,
		consts.ExtM4V,
		consts.ExtMOV,
		consts.ExtMP4:
		// Containers use lowercase keys (already stored as lowercase).
		return key

	case consts.ExtMKV,
		consts.ExtWEBM:
		// Matroska uses UPPERCASE tags.
		switch key {
		case consts.JArtist:
			return "ARTIST"
		case consts.JActor:
			return "LEAD_PERFORMER"
		case consts.JComposer:
			return "COMPOSER"
		case consts.JPerformer:
			return "PERFORMER"
		case consts.JProducer:
			return "PRODUCER"
		case consts.JDirector:
			return "DIRECTOR"
		case consts.JTitle:
			return "TITLE"
		case consts.JLongDesc:
			return "DESCRIPTION"
		case consts.JSummary:
			return "SUMMARY"
		case consts.JSynopsis:
			return "SYNOPSIS"
		case consts.JDescription:
			return "SUBJECT"
		case consts.JReleaseDate, consts.JDate:
			return "DATE_RELEASED"
		case consts.JCreationTime:
			return "DATE_ENCODED"
		case consts.JYear:
			return "DATE_RELEASED"
		}

	case consts.ExtWMV,
		consts.ExtASF:
		// WMV uses TitleCase and WM/ prefixes.
		switch key {
		case consts.JTitle:
			return "Title"
		case consts.JArtist:
			return "WM/AlbumArtist"
		case consts.JComposer:
			return "WM/Composer"
		case consts.JDirector:
			return "WM/Director"
		case consts.JProducer:
			return "WM/Producer"
		case consts.JSubtitle:
			return "WM/SubTitle"
		case consts.JLongDesc:
			return "WM/SubTitleDescription"
		case consts.JDate:
			return "WM/EncodingTime"
		case consts.JYear:
			return "WM/Year"
		}

	case consts.ExtOGM,
		consts.ExtOGV:
		// Ogg uses UPPERCASE Vorbis comments.
		switch key {
		case consts.JArtist:
			return "ARTIST"
		case consts.JPerformer:
			return "PERFORMER"
		case consts.JComposer:
			return "COMPOSER"
		case consts.JDescription, consts.JLongDesc:
			return "DESCRIPTION"
		case consts.JSummary, consts.JSynopsis:
			return "SUMMARY"
		case consts.JDate:
			return "DATE"
		case consts.JTitle:
			return "TITLE"
		}

	case consts.ExtAVI:
		// AVI uses RIFF INFO tags (4-character codes).
		switch key {
		case consts.JLongDesc:
			return "COMM"
		case consts.JArtist:
			return "IART"
		case consts.JDescription:
			return "ICMT"
		case consts.JReleaseDate:
			return "ICRD"
		case consts.JProducer:
			return "IENG"
		case consts.JTitle:
			return "INAM"
		case consts.JSynopsis:
			return "ISBJ"
		case consts.JYear, consts.JReleaseYear:
			return "YEAR"
		}

	case consts.ExtFLV:
		// FLV uses lowercase tags.
		switch key {
		case consts.JDate:
			return "creationdate"
		}

	case consts.ExtRM,
		consts.ExtRMVB:
		// RealMedia uses TitleCase.
		switch key {
		case consts.JAuthor:
			return "Author"
		case consts.JDescription:
			return "Comment"
		case consts.JTitle:
			return "Title"
		}

	case consts.ExtMTS,
		consts.ExtTS:
		// MPEG-TS uses specific service tags.
		switch key {
		case consts.JArtist:
			return "service_provider"
		case consts.JTitle:
			return "service_name"
		}

	default:
		// For unknown container types, use input key.
		return ""
	}
	return ""
}
