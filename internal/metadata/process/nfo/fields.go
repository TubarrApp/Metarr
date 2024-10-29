package metadata

import (
	consts "Metarr/internal/domain/constants"
	"Metarr/internal/types"
	logging "Metarr/internal/utils/logging"
	print "Metarr/internal/utils/print"
	"strings"
)

// FillNFO is the primary entrypoint for filling NFO metadata
// from an open file's read content
func FillNFO(fd *types.FileData) bool {

	var filled bool

	if ok := fillNFOTitles(fd); ok {
		filled = true
	}
	if ok := fillNFODescriptions(fd); ok {
		filled = true
	}
	if ok := fillNFOCredits(fd); ok {
		filled = true
	}

	print.CreateModelPrintout(fd, fd.NFOBaseName, "Fill metadata from NFO for file '%s'", fd.NFOFilePath)

	return filled
}

// fillNFODescriptions attempts to fill in title info from NFO
func fillNFODescriptions(fd *types.FileData) bool {

	d := fd.MTitleDesc
	n := fd.NFOData

	fieldMap := map[string]*string{
		consts.NDescription: &d.Description,
		consts.NPlot:        &d.LongDescription,
	}

	// Post-unmarshal clean
	cleanEmptyFields(fieldMap)

	if n.Description != "" {
		if d.Description == "" {
			d.Description = n.Description
		}
	}
	if n.Plot != "" {
		if d.Description == "" {
			d.Description = n.Plot
		}
		if d.LongDescription == "" {
			d.Description = n.Plot
		}
	}

	print.CreateModelPrintout(n, "", "Parsing NFO data")
	return true
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
func unpackNFO(fd *types.FileData, data map[string]interface{}, fieldMap map[string]*string) {
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

// unpackTitle unpacks common nested title elements to the model
func unpackTitle(fd *types.FileData, titleData map[string]interface{}) bool {
	t := fd.MTitleDesc
	filled := false

	for key, value := range titleData {
		switch key {
		case "main":
			if strVal, ok := value.(string); ok {
				logging.PrintD(3, "Setting main title to '%s'", strVal)
				t.Title = strVal
				filled = true
			}
		case "sub":
			if strVal, ok := value.(string); ok {
				logging.PrintD(3, "Setting subtitle to '%s'", strVal)
				t.Subtitle = strVal
				filled = true
			}
		default:
			logging.PrintD(1, "Unknown nested title element '%s', skipping...", key)
		}
	}
	return filled
}

func unpackCredits(fd *types.FileData, creditsData map[string]interface{}) bool {
	c := fd.MCredits
	filled := false

	// Recursive helper to search for "role" within nested maps
	var findRoles func(data map[string]interface{})
	findRoles = func(data map[string]interface{}) {
		// Check each key-value pair within the actor data
		for k, v := range data {
			if k == "role" {
				if role, ok := v.(string); ok {
					logging.PrintD(3, "Adding role '%s' to actors", role)
					c.Actors = append(c.Actors, role)
					filled = true
				}
			} else if nested, ok := v.(map[string]interface{}); ok {
				// Recursive call for further nested maps
				findRoles(nested)
			} else if nestedList, ok := v.([]interface{}); ok {
				// Handle lists of nested elements
				for _, item := range nestedList {
					if nestedMap, ok := item.(map[string]interface{}); ok {
						findRoles(nestedMap)
					}
				}
			}
		}
	}

	// Access the "cast" data to find "actor" entries
	if castData, ok := creditsData["cast"].(map[string]interface{}); ok {
		if actorsData, ok := castData["actor"].([]interface{}); ok {
			for _, actorData := range actorsData {
				if actorMap, ok := actorData.(map[string]interface{}); ok {
					if name, ok := actorMap["name"].(string); ok {
						logging.PrintD(3, "Adding actor name '%s'", name)
						c.Actors = append(c.Actors, name)
						filled = true
					}
					if role, ok := actorMap["role"].(string); ok {
						logging.PrintD(3, "Adding actor role '%s'", role)
						filled = true
					}
				}
			}
		} else {
			logging.PrintD(1, "'actor' key is present but not a valid structure")
		}
	} else {
		logging.PrintD(1, "'cast' key is missing or not a map")
	}

	return filled
}
