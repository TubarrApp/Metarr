package parsing

import (
	"metarr/internal/utils/logging"
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
func (mtp *MetaTemplateParser) FillMetaTemplateTag(inputStr string, j map[string]any) (result string) {
	result, replaced := mtp.fillMetaTemplateTagRecursive(inputStr, j)
	if !replaced {
		return inputStr // No successful replacements, return original
	}
	return result
}

func (mtp *MetaTemplateParser) fillMetaTemplateTagRecursive(inputStr string, j map[string]any) (result string, anyReplaced bool) {
	openTagIdx := strings.Index(inputStr, "{{")
	closeTagIdx := strings.Index(inputStr, "}}")
	if openTagIdx < 0 || closeTagIdx < 0 || closeTagIdx < openTagIdx {
		return inputStr, false
	}

	// Bounds check
	if openTagIdx+2 > len(inputStr) || closeTagIdx+2 > len(inputStr) {
		return inputStr, false
	}

	// Ensure content between open and close tags
	if openTagIdx+2 > closeTagIdx {
		return inputStr, false
	}

	tagContent := inputStr[openTagIdx+2 : closeTagIdx]
	replacement, succeeded := mtp.fillTag(tagContent, j)

	// If fillTag failed, return original with no replacement flag
	if !succeeded {
		return inputStr, false
	}

	tag := inputStr[:openTagIdx] + replacement + inputStr[closeTagIdx+2:]
	recursiveResult, recursiveReplaced := mtp.fillMetaTemplateTagRecursive(tag, j)
	return recursiveResult, true || recursiveReplaced
}

// fillTag finds the matching string for a given template tag.
//
// Returns the replacement string and whether it succeeded.
func (mtp *MetaTemplateParser) fillTag(template string, j map[string]any) (result string, success bool) {
	if template == "" || j[template] == nil {
		return "", false
	}

	// Search map for template key
	for k, v := range j {
		if k == template {
			strVal, ok := v.(string)
			if !ok || strVal == "" {
				logging.E("JSON key %v does not contain a valid string value (variable is of type %T), not parsing (file %q)", template, v, mtp.jsonFileName)
				return "", false
			}
			return strVal, true
		}
	}

	logging.D(1, "Value for JSON key %q does not exist in file %q", template, mtp.jsonFileName)
	return "", false
}
