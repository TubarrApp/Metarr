package nfofields

import (
	"metarr/internal/domain/consts"
	"metarr/internal/models"
	"metarr/internal/utils/logging"
)

// fillNFOTitles attempts to fill in title info from NFO
func fillNFOTitles(fd *models.FileData) bool {

	t := fd.MTitleDesc
	n := fd.NFOData

	fieldMap := map[string]*string{
		consts.NTitle:         &t.Title,
		consts.NOriginalTitle: &t.Fulltitle,
		consts.NTagline:       &t.Subtitle,
	}

	// Post-unmarshal clean
	cleanEmptyFields(fieldMap)

	logging.I("Grab NFO metadata: %v", t)

	if n.Title.Main != "" {
		if t.Title == "" {
			t.Title = n.Title.Main
		}
	}
	if n.Title.Original != "" {
		if t.Fulltitle == "" {
			t.Fulltitle = n.Title.Original
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

// unpackTitle unpacks common nested title elements to the model.
func unpackTitle(fd *models.FileData, titleData map[string]any) bool {
	t := fd.MTitleDesc
	filled := false

	for key, value := range titleData {
		switch key {
		case "main":
			if strVal, ok := value.(string); ok {
				logging.D(3, "Setting main title to %q", strVal)
				t.Title = strVal
				filled = true
			}
		case "sub":
			if strVal, ok := value.(string); ok {
				logging.D(3, "Setting subtitle to %q", strVal)
				t.Subtitle = strVal
				filled = true
			}
		default:
			logging.D(1, "Unknown nested title element %q, skipping...", key)
		}
	}
	return filled
}
