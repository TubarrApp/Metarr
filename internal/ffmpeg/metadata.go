package ffmpeg

import (
	"metarr/internal/models"
	"strings"

	"github.com/TubarrApp/gocommon/sharedtags"
)

// addAllMetadata combines all metadata into a single map.
func (b *ffCommandBuilder) addAllMetadata(fd *models.FileData) {
	b.addTitlesDescs(fd.MTitleDesc)
	b.addCredits(fd.MCredits)
	b.addDates(fd.MDates)
	b.addShowInfo(fd.MShowData)
	b.addOtherMetadata(fd.MOther)
}

// addTitlesDescs adds all title/description-related metadata.
func (b *ffCommandBuilder) addTitlesDescs(t *models.MetadataTitlesDescs) {
	// Prefer fulltitle.
	if t.Fulltitle != "" {
		t.Title = t.Fulltitle
	}
	if t.Fulltitle == "" && t.Title != "" {
		t.Fulltitle = t.Title
	}

	// Prefer long (non-truncated) description.
	if t.LongDescription == "" {
		if t.LongUnderscoreDescription != "" {
			t.LongDescription = t.LongUnderscoreDescription
		}
		if t.Description != "" {
			t.LongDescription = t.Description
		}
	}
	if t.LongUnderscoreDescription == "" {
		if t.LongDescription != "" {
			t.LongUnderscoreDescription = t.LongDescription
		}
	}
	if t.LongDescription != "" {
		t.Description = t.LongDescription
	}

	fields := map[string]string{
		sharedtags.JTitle:       t.Title,
		sharedtags.JSubtitle:    t.Subtitle,
		sharedtags.JDescription: t.Description,
		sharedtags.JLongDesc:    t.LongDescription,
		sharedtags.JSummary:     t.Summary,
		sharedtags.JSynopsis:    t.Synopsis,
	}

	for field, value := range fields {
		if value != "" {
			b.metadataMap[field] = value
		}
	}
}

// addCredits adds all credit-related metadata.
func (b *ffCommandBuilder) addCredits(c *models.MetadataCredits) {
	// Single value credits.
	fields := map[string]string{
		sharedtags.JActor:     c.Actor,
		sharedtags.JAuthor:    c.Author,
		sharedtags.JArtist:    c.Artist,
		sharedtags.JCreator:   c.Creator,
		sharedtags.JStudio:    c.Studio,
		sharedtags.JPublisher: c.Publisher,
		sharedtags.JProducer:  c.Producer,
		sharedtags.JPerformer: c.Performer,
		sharedtags.JComposer:  c.Composer,
		sharedtags.JDirector:  c.Director,
		sharedtags.JWriter:    c.Writer,
	}

	for field, value := range fields {
		if value != "" {
			b.metadataMap[field] = value
		}
	}

	// Array credits (length already checked in function).
	b.addArrayMetadata(sharedtags.JActor, c.Actors)
	b.addArrayMetadata(sharedtags.JComposer, c.Composers)
	b.addArrayMetadata(sharedtags.JArtist, c.Artists)
	b.addArrayMetadata(sharedtags.JStudio, c.Studios)
	b.addArrayMetadata(sharedtags.JPerformer, c.Performers)
	b.addArrayMetadata(sharedtags.JProducer, c.Producers)
	b.addArrayMetadata(sharedtags.JPublisher, c.Publishers)
	b.addArrayMetadata(sharedtags.JDirector, c.Directors)
	b.addArrayMetadata(sharedtags.JWriter, c.Writers)
}

// addDates adds all date-related metadata.
func (b *ffCommandBuilder) addDates(d *models.MetadataDates) {
	fields := map[string]string{
		sharedtags.JCreationTime:        d.CreationTime,
		sharedtags.JDate:                d.Date,
		sharedtags.JOriginallyAvailable: d.OriginallyAvailableAt,
		sharedtags.JReleaseDate:         d.ReleaseDate,
		sharedtags.JUploadDate:          d.UploadDate,
		sharedtags.JYear:                d.Year,
	}

	for field, value := range fields {
		if value != "" {
			b.metadataMap[field] = value
		}
	}
}

// addShowInfo adds all show info related metadata.
func (b *ffCommandBuilder) addShowInfo(s *models.MetadataShowData) {
	fields := map[string]string{
		"episode_id":    s.EpisodeID,
		"episode_sort":  s.EpisodeSort,
		"season_number": s.SeasonNumber,
		"season_title":  s.SeasonTitle,
		"show":          s.Show,
	}

	for field, value := range fields {
		if value != "" {
			b.metadataMap[field] = value
		}
	}
}

// addOtherMetadata adds other related metadata.
func (b *ffCommandBuilder) addOtherMetadata(o *models.MetadataOtherData) {
	fields := map[string]string{
		"genre":    o.Genre,
		"hd_video": o.HDVideo,
		"language": o.Language,
	}

	for field, value := range fields {
		if value != "" {
			b.metadataMap[field] = value
		}
	}
}

// addArrayMetadata combines array values with existing metadata.
func (b *ffCommandBuilder) addArrayMetadata(key string, values []string) {
	if len(values) == 0 {
		return
	}

	existing, exists := b.metadataMap[key]
	newValue := strings.Join(values, "; ")

	b.builder.Reset()
	if exists && existing != "" {
		for i, v := range values {
			if i > 0 {
				b.builder.WriteString("; ")
			}
			b.builder.WriteString(v)
		}
	} else {
		b.metadataMap[key] = newValue
	}
}
