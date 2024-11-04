package metadata

import (
	"Metarr/internal/models"
	logging "Metarr/internal/utils/logging"
	print "Metarr/internal/utils/print"
	"strings"
)

// FillNFO is the primary entrypoint for filling NFO metadata
// from an open file's read content
func FillNFO(fd *models.FileData) bool {

	var filled bool

	if ok := fillNFOTimestamps(fd); ok {
		filled = true
	}

	if ok := fillNFOTitles(fd); ok {
		filled = true
	}

	if ok := fillNFODescriptions(fd); ok {
		filled = true
	}

	if ok := fillNFOCredits(fd); ok {
		filled = true
	}

	if ok := fillNFOWebData(fd); ok {
		filled = true
	}

	print.CreateModelPrintout(fd, fd.NFOBaseName, "Fill metadata from NFO for file '%s'", fd.NFOFilePath)

	return filled
}

// Clean up empty fields from fieldmap
func cleanEmptyFields(fieldMap map[string]*string) {
	for _, value := range fieldMap {
		if strings.TrimSpace(*value) == "" {
			*value = ""
		}
	}
}

// nestedLoop parses content recursively and returns a nested map
func nestedLoop(content string) map[string]interface{} {
	nested := make(map[string]interface{})

	logging.PrintD(2, "Parsing content in nestedLoop: %s", content)

	for len(content) > 0 {
		if strings.HasPrefix(content, "<?xml") || strings.HasPrefix(content, "<?") {
			endIdx := strings.Index(content, "?>")
			if endIdx == -1 {
				logging.PrintE(0, "Malformed XML declaration in content: %s", content)
				break
			}
			content = content[endIdx+2:]
			logging.PrintD(2, "Skipping XML declaration, remaining content: %s", content)
			continue
		}

		// Find the opening tag
		openIdx := strings.Index(content, "<")
		if openIdx == -1 {
			break // No more tags
		}

		openIdxClose := strings.Index(content, ">")
		if openIdxClose == -1 {
			logging.PrintE(0, "No valid tag close bracket for entry beginning %s", content[openIdx:])
			break // No closing tag bracket
		}

		// Get the tag name and check if it is self-closing
		tag := content[openIdx+1 : openIdxClose]
		isSelfClosing := strings.HasSuffix(tag, "/")
		tag = strings.TrimSuffix(tag, "/") // Remove trailing / if present

		if isSelfClosing {
			// Self-closing tag; skip over and move to the next
			content = content[openIdxClose+1:]
			logging.PrintD(2, "Skipping self-closing tag: %s", tag)
			continue
		}

		// Look for the corresponding closing tag
		closeTag := "</" + tag + ">"
		closeIdx := strings.Index(content, closeTag)
		if closeIdx == -1 {
			// No closing tag; skip this tag and continue
			content = content[openIdxClose+1:]
			logging.PrintD(2, "Skipping tag without end tag: %s", tag)
			continue
		}

		// Extract the inner content between tags
		innerContent := content[openIdxClose+1 : closeIdx]
		logging.PrintD(2, "Found inner content for tag '%s': %s", tag, innerContent)

		// Recursive call if innerContent contains nested tags
		if strings.Contains(innerContent, "<") && strings.Contains(innerContent, ">") {
			logging.PrintD(2, "Recursively parsing nested content for tag '%s'", tag)
			nested[tag] = nestedLoop(innerContent)
		} else {
			logging.PrintD(2, "Assigning inner content to tag '%s': %s", tag, innerContent)
			nested[tag] = innerContent
		}

		// Move past the processed tag
		content = content[closeIdx+len(closeTag):]
		logging.PrintD(2, "Remaining content after parsing tag '%s': %s", tag, content)
	}

	logging.PrintD(2, "Final parsed structure from nestedLoop: %v", nested)
	return nested
}

// unpackNFO unpacks an NFO map back to the model
func unpackNFO(fd *models.FileData, data map[string]interface{}, fieldMap map[string]*string) {
	logging.PrintD(3, "Unpacking NFO map...")

	// Access the top-level "movie" key
	movieData, ok := data["movie"].(map[string]interface{})
	if !ok {
		logging.PrintE(0, "Missing 'movie' key in data, unable to unpack")
		return
	}

	for field, fieldVal := range fieldMap {
		if fieldVal == nil {
			logging.PrintE(0, "Field value is null, continuing...")
			continue
		}

		// Look for the field in the movie data
		val, exists := movieData[field]
		if !exists {
			continue // Field does not exist in this map
		}

		switch v := val.(type) {
		case string:
			logging.PrintD(3, "Setting field '%s' to '%s'", field, v)
			*fieldVal = v
		case map[string]interface{}:
			switch field {

			case "title":
				logging.PrintD(3, "Unpacking nested 'title' map...")
				unpackTitle(fd, v)
			case "cast":
				logging.PrintD(3, "Unpacking nested 'cast' map...")
				unpackCredits(fd, v)
			}
		default:
			logging.PrintD(1, "Unknown field type for '%s', skipping...", field)
		}
	}
}
