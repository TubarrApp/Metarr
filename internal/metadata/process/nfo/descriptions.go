package metadata

import (
	"metarr/internal/domain/consts"
	"metarr/internal/models"
	print "metarr/internal/utils/print"
)

// fillNFODescriptions attempts to fill in title info from NFO
func fillNFODescriptions(fd *models.FileData) bool {

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

	print.CreateModelPrintout(fd, fd.NFOFilePath, "Parsing NFO descriptions")
	return true
}
