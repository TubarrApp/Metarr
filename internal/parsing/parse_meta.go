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
func (mtp *MetaTemplateParser) FillMetaTemplateTag(inputStr string, j map[string]any) (result string) {
	openTagIdx := strings.Index(inputStr, "{{")
	closeTagIdx := strings.Index(inputStr, "}}")
	if openTagIdx < 0 || closeTagIdx < 0 || closeTagIdx < openTagIdx {
		return inputStr
	}

	// Bounds check
	if openTagIdx+2 > len(inputStr) || closeTagIdx+2 > len(inputStr) {
		return inputStr
	}

	// Ensure content between open and close tags
	if openTagIdx+2 > closeTagIdx {
		return inputStr
	}

	tag := inputStr[:openTagIdx] + mtp.fillTag(inputStr[openTagIdx+2:closeTagIdx], j) + inputStr[closeTagIdx+2:]
	return mtp.FillMetaTemplateTag(tag, j)
}

// fillTag finds the matching string for a given template tag.
func (mtp *MetaTemplateParser) fillTag(template string, j map[string]any) (result string) {
	if template == "" || j[template] == nil {
		return template
	}

	// Search map for template key
	for k, v := range j {
		if k == template {
			strVal, ok := v.(string)
			if !ok {
				logging.E("JSON key %v does not contain a string value (variable is of type %T), not parsing (file %q)", template, mtp.jsonFileName)
				return ""
			}
			return strVal
		}
	}
	logging.D(1, "Value for JSON key %q does not exist in file %q", template, mtp.jsonFileName)
	return ""
}
