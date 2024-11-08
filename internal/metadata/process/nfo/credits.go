package metadata

import (
	consts "metarr/internal/domain/constants"
	"metarr/internal/models"
	logging "metarr/internal/utils/logging"
	"strings"
)

// fillNFODescriptions attempts to fill in title info from NFO
func fillNFOCredits(fd *models.FileData) bool {

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
		fillSingleCredits(c.Actors, &c.Actor)
	}
	if n.Directors != nil {
		c.Directors = append(c.Directors, n.Directors...)
		fillSingleCredits(c.Directors, &c.Director)
	}
	if n.Producers != nil {
		c.Producers = append(c.Producers, n.Producers...)
		fillSingleCredits(c.Producers, &c.Producer)
	}
	if n.Writers != nil {
		c.Writers = append(c.Writers, n.Writers...)
		fillSingleCredits(c.Writers, &c.Writer)
	}
	if n.Publishers != nil {
		c.Publishers = append(c.Publishers, n.Publishers...)
		fillSingleCredits(c.Publishers, &c.Publisher)
	}
	if n.Studios != nil {
		c.Studios = append(c.Studios, n.Studios...)
		fillSingleCredits(c.Studios, &c.Studio)
	}

	return true
}

// fillSingleCredits fills empty singular credits fields from
// filled arrays
func fillSingleCredits(entries []string, target *string) {

	if target == nil {
		logging.D(1, "Target string is nil, skipping...")
		return
	}

	if *target != "" {
		logging.D(1, "Target string is not empty, skipping...")
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

func unpackCredits(fd *models.FileData, creditsData map[string]interface{}) bool {
	c := fd.MCredits
	filled := false

	// Recursive helper to search for "role" within nested maps
	var findRoles func(data map[string]interface{})
	findRoles = func(data map[string]interface{}) {
		// Check each key-value pair within the actor data
		for k, v := range data {
			if k == "role" {
				if role, ok := v.(string); ok {
					logging.D(3, "Adding role '%s' to actors", role)
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
						logging.D(3, "Adding actor name '%s'", name)
						c.Actors = append(c.Actors, name)
						filled = true
					}
					if role, ok := actorMap["role"].(string); ok {
						logging.D(3, "Adding actor role '%s'", role)
						filled = true
					}
				}
			}
		} else {
			logging.D(1, "'actor' key is present but not a valid structure")
		}
	} else {
		logging.D(1, "'cast' key is missing or not a map")
	}

	return filled
}
