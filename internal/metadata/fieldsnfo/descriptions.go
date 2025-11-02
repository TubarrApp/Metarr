package fieldsnfo

import (
	"metarr/internal/domain/consts"
	"metarr/internal/models"
	"metarr/internal/utils/logging"
	"metarr/internal/utils/printout"
)

// fillNFODescriptions attempts to fill in descriptions from NFO.
func fillNFODescriptions(fd *models.FileData) (filled bool) {
	d := fd.MTitleDesc
	n := fd.NFOData

	fieldMap := map[string]*string{
		consts.NDescription: &d.Description,
		consts.NPlot:        &d.LongDescription,
	}

	// Post-unmarshal clean
	cleanEmptyFields(fieldMap)
	printMap := make(map[string]string, len(fieldMap))

	defer func() {
		if logging.Level > 0 && len(printMap) > 0 {
			printout.PrintGrabbedFields("descriptions", printMap)
		}
	}()

	if n.Description != "" {
		if d.Description == "" {
			d.Description = n.Description
		}
		if d.LongDescription == "" {
			d.LongDescription = n.Description
		}
	}

	if n.Plot != "" {
		if d.Description == "" {
			d.Description = n.Plot
		}
		if d.LongDescription == "" {
			d.LongDescription = n.Plot
		}
	}

	if d.Synopsis == "" {
		switch {
		case n.Plot != "":
			d.Synopsis = n.Plot
		case n.Description != "":
			d.Summary = n.Description
		case d.LongDescription != "":
			d.Synopsis = d.LongDescription
		case d.Description != "":
			d.Synopsis = d.Description
		}
	}

	if d.Summary == "" {
		switch {
		case n.Plot != "":
			d.Summary = n.Plot
		case n.Description != "":
			d.Summary = n.Description
		case d.LongDescription != "":
			d.Summary = d.LongDescription
		case d.Description != "":
			d.Summary = d.Description
		}
	}

	if d.Comment == "" {
		switch {
		case n.Plot != "":
			d.Comment = n.Plot
		case n.Description != "":
			d.Comment = n.Description
		case d.LongDescription != "":
			d.Comment = d.LongDescription
		case d.Description != "":
			d.Comment = d.Description
		}
	}

	if d.LongDescription != "" {
		printMap[consts.NDescription] = d.LongDescription
	}

	if d.Description != "" {
		printMap[consts.NDescription] = d.Description
	}

	return true
}
