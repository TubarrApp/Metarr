package metadata

import (
	"Metarr/internal/types"
	"strings"
)

// addCredits adds all credit-related metadata
func (b *CommandBuilder) addTitlesDescs(t *types.MetadataTitlesDescs) {

	if t.Title == "" && t.FallbackTitle != "" {
		t.Title = t.FallbackTitle
	}
	if t.LongDescription == "" && t.Long_Description != "" {
		t.LongDescription = t.Long_Description
	}

	fields := map[string]string{
		"title":           t.Title,
		"subtitle":        t.Subtitle,
		"description":     t.Description,
		"longdescription": t.LongDescription,
		"summary":         t.Summary,
		"synopsis":        t.Synopsis,
	}

	for field, value := range fields {
		if value != "" {
			b.metadataMap[field] = value
		}
	}
}

// addCredits adds all credit-related metadata
func (b *CommandBuilder) addCredits(c *types.MetadataCredits) {

	// Single value credits
	fields := map[string]string{
		"actor":     c.Actor,
		"author":    c.Author,
		"artist":    c.Artist,
		"creator":   c.Creator,
		"studio":    c.Studio,
		"publisher": c.Publisher,
		"producer":  c.Producer,
		"performer": c.Performer,
		"composer":  c.Composer,
		"director":  c.Director,
		"writer":    c.Writer,
	}

	for field, value := range fields {
		if value != "" {
			b.metadataMap[field] = value
		}
	}

	// Array credits
	b.addArrayMetadata("actor", c.Actors)
	b.addArrayMetadata("composer", c.Composers)
	b.addArrayMetadata("artist", c.Artists)
	b.addArrayMetadata("studio", c.Studios)
	b.addArrayMetadata("performer", c.Performers)
	b.addArrayMetadata("producer", c.Producers)
	b.addArrayMetadata("publisher", c.Publishers)
	b.addArrayMetadata("director", c.Directors)
	b.addArrayMetadata("writer", c.Writers)
}

// addCredits adds all date-related metadata
func (b *CommandBuilder) addDates(d *types.MetadataDates) {

	fields := map[string]string{
		"creation_time":           d.Creation_Time,
		"date":                    d.Date,
		"originally_available_at": d.Originally_Available_At,
		"release_date":            d.ReleaseDate,
		"upload_date":             d.UploadDate,
		"year":                    d.Year,
	}

	for field, value := range fields {
		if value != "" {
			b.metadataMap[field] = value
		}
	}
}

// addShowInfo adds all show info related metadata
func (b *CommandBuilder) addShowInfo(s *types.MetadataShowData) {

	fields := map[string]string{
		"episode_id":    s.Episode_ID,
		"episode_sort":  s.Episode_Sort,
		"season_number": s.Season_Number,
		"season_title":  s.Season_Title,
		"show":          s.Show,
	}

	for field, value := range fields {
		if value != "" {
			b.metadataMap[field] = value
		}
	}
}

// addOtherMetadata adds other related metadata
func (b *CommandBuilder) addOtherMetadata(o *types.MetadataOtherData) {

	fields := map[string]string{
		"genre":    o.Genre,
		"hd_video": o.HD_Video,
		"language": o.Language,
	}

	for field, value := range fields {
		if value != "" {
			b.metadataMap[field] = value
		}
	}
}

// addArrayMetadata combines array values with existing metadata
func (b *CommandBuilder) addArrayMetadata(key string, values []string) {
	if len(values) == 0 {
		return
	}

	existing, exists := b.metadataMap[key]
	newValue := strings.Join(values, "; ")

	if exists && existing != "" {
		b.metadataMap[key] = existing + "; " + newValue
	} else {
		b.metadataMap[key] = newValue
	}
}
