package metadata

import (
	consts "Metarr/internal/domain/constants"
	"Metarr/internal/models"
	logging "Metarr/internal/utils/logging"
)

// fillNFOTitles attempts to fill in title info from NFO
func fillNFOTitles(fd *models.FileData) bool {

	t := fd.MTitleDesc
	n := fd.NFOData

	fieldMap := map[string]*string{
		consts.NTitle:         &t.Title,
		consts.NOriginalTitle: &t.FallbackTitle,
		consts.NTagline:       &t.Subtitle,
	}

	// Post-unmarshal clean
	cleanEmptyFields(fieldMap)

	logging.PrintI("Grab NFO metadata: %v", t)

	if n.Title.Main != "" {
		if t.Title == "" {
			t.Title = n.Title.Main
		}
	}
	if n.Title.Original != "" {
		if t.FallbackTitle == "" {
			t.FallbackTitle = n.Title.Original
		}
		if t.Title == "" {
			t.Title = n.Title.Original
		}
	}
	if n.Title.Sub != "" {
		if t.Subtitle == "" {
			t.Subtitle = n.Title.Sub
		}
	}
	if n.Title.PlainText != "" {
		if t.Title == "" {
			t.Title = n.Title.PlainText
		}
	}
	return true
}

// unpackTitle unpacks common nested title elements to the model
func unpackTitle(fd *models.FileData, titleData map[string]interface{}) bool {
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
