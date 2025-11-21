package fieldsnfo

import (
	"metarr/internal/domain/logger"
	"metarr/internal/models"
	"metarr/internal/utils/printout"
	"strings"

	"github.com/TubarrApp/gocommon/logging"
	"github.com/TubarrApp/gocommon/sharedtags"
)

// fillNFODescriptions attempts to fill in title info from NFO.
func fillNFOCredits(fd *models.FileData) (filled bool) {
	c := fd.MCredits
	n := fd.NFOData

	fieldMap := map[string]*string{
		sharedtags.NActors:            &c.Actor,
		sharedtags.NDirector:          &c.Director,
		sharedtags.NProductionCompany: &c.Publisher,
		sharedtags.NStudio:            &c.Studio,
		sharedtags.NWriter:            &c.Writer,
		sharedtags.NProducer:          &c.Producer,
	}

	// Post-unmarshal clean.
	cleanEmptyFields(fieldMap)
	printMap := make(map[string]string, len(fieldMap))
	defer func() {
		if len(printMap) > 0 {
			if logging.Level > 0 && len(printMap) > 0 {
				printout.PrintGrabbedFields("time and date", printMap)
			}
		}
	}()

	if n.Actors != nil {
		for _, actor := range n.Actors {
			c.Actors = append(c.Actors, actor.Name)
		}
		fillSingleCredits(c.Actors, &c.Actor)
		printMap[sharedtags.NActors] = strings.Join(c.Actors, ",")
	}
	if n.Directors != nil {
		c.Directors = append(c.Directors, n.Directors...)
		fillSingleCredits(c.Directors, &c.Director)
		printMap[sharedtags.NDirector] = strings.Join(c.Directors, ",")
	}
	if n.Producers != nil {
		c.Producers = append(c.Producers, n.Producers...)
		fillSingleCredits(c.Producers, &c.Producer)
		printMap[sharedtags.NProducer] = strings.Join(c.Producers, ",")
	}
	if n.Writers != nil {
		c.Writers = append(c.Writers, n.Writers...)
		fillSingleCredits(c.Writers, &c.Writer)
		printMap[sharedtags.NWriter] = strings.Join(c.Writers, ",")
	}
	if n.Publishers != nil {
		c.Publishers = append(c.Publishers, n.Publishers...)
		fillSingleCredits(c.Publishers, &c.Publisher)
		printMap[sharedtags.NPublisher] = strings.Join(c.Publishers, ",")
	}
	if n.Studios != nil {
		c.Studios = append(c.Studios, n.Studios...)
		fillSingleCredits(c.Studios, &c.Studio)
		printMap[sharedtags.NStudio] = strings.Join(c.Studios, ",")
	}
	return true
}

// fillSingleCredits fills empty singular credits fields from filled arrays.
func fillSingleCredits(entries []string, target *string) {
	if target == nil {
		logger.Pl.D(1, "Target string is nil, skipping...")
		return
	}

	if *target != "" {
		logger.Pl.D(1, "Target string is not empty, skipping...")
		return
	}

	filtered := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry != "" {
			filtered = append(filtered, entry)
		}
	}

	*target = strings.Join(filtered, ", ")
}
