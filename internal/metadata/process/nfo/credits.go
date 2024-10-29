package metadata

import (
	consts "Metarr/internal/domain/constants"
	"Metarr/internal/types"
)

// fillNFODescriptions attempts to fill in title info from NFO
func fillNFOCredits(fd *types.FileData) bool {

	c := fd.MCredits
	n := fd.NFOData

	fieldMap := map[string]*string{
		consts.NActors:            &c.Actor,
		consts.NDirector:          &c.Director,
		consts.NProductionCompany: &c.Publisher,
		consts.NStudio:            &c.Studio,
		consts.NWriter:            &c.Writer,
		consts.NProducer:          &c.Producer,
	}

	// Post-unmarshal clean
	cleanEmptyFields(fieldMap)

	if n.Actors != nil {
		for _, actor := range n.Actors {
			c.Actors = append(c.Actors, actor.Name)
		}
	}
	if n.Directors != nil {
		c.Directors = append(c.Directors, n.Directors...)
	}
	if n.Producers != nil {
		c.Producers = append(c.Producers, n.Producers...)
	}
	if n.Writers != nil {
		c.Writers = append(c.Writers, n.Writers...)
	}
	if n.Publishers != nil {
		c.Publishers = append(c.Publishers, n.Publishers...)
	}
	if n.Studios != nil {
		c.Studios = append(c.Studios, n.Studios...)
	}

	return true
}
