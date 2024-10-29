package metadata

import (
	consts "Metarr/internal/domain/constants"
	"Metarr/internal/types"
	logging "Metarr/internal/utils/logging"
)

// fillNFOTitles attempts to fill in title info from NFO
func fillNFOTitles(fd *types.FileData) bool {

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
	return true
}
