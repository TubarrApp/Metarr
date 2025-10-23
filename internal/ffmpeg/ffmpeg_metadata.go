package ffmpeg

import (
	"metarr/internal/domain/consts"
	"metarr/internal/models"
	"strings"
)

// addAllMetadata combines all metadata into a single map
func (b *ffCommandBuilder) addAllMetadata(fd *models.FileData) {
	b.addTitlesDescs(fd.MTitleDesc)
	b.addCredits(fd.MCredits)
	b.addDates(fd.MDates)
	b.addShowInfo(fd.MShowData)
	b.addOtherMetadata(fd.MOther)
}

// addTitlesDescs adds all title/description-related metadata
func (b *ffCommandBuilder) addTitlesDescs(t *models.MetadataTitlesDescs) {
	// Prefer fulltitle if possible (also exists in the JSON processing func)
	if t.Title == "" && t.Fulltitle != "" {
		t.Title = t.Fulltitle
	}

	if t.LongDescription == "" && t.LongUnderscoreDescription != "" {
		t.LongDescription = t.LongUnderscoreDescription
	}

	fields := map[string]string{
		consts.JTitle:       t.Title,
		consts.JSubtitle:    t.Subtitle,
		consts.JDescription: t.Description,
		consts.JLongDesc:    t.LongDescription,
		consts.JSummary:     t.Summary,
		consts.JSynopsis:    t.Synopsis,
	}

	for field, value := range fields {
		if value != "" {
			b.metadataMap[field] = value
		}
	}
}

// addCredits adds all credit-related metadata
func (b *ffCommandBuilder) addCredits(c *models.MetadataCredits) {
	// Single value credits
	fields := map[string]string{
		consts.JActor:     c.Actor,
		consts.JAuthor:    c.Author,
		consts.JArtist:    c.Artist,
		consts.JCreator:   c.Creator,
		consts.JStudio:    c.Studio,
		consts.JPublisher: c.Publisher,
		consts.JProducer:  c.Producer,
		consts.JPerformer: c.Performer,
		consts.JComposer:  c.Composer,
		consts.JDirector:  c.Director,
		consts.JWriter:    c.Writer,
	}

	for field, value := range fields {
		if value != "" {
			b.metadataMap[field] = value
		}
	}

	// Array credits (length already checked in function)
	b.addArrayMetadata(consts.JActor, c.Actors)
	b.addArrayMetadata(consts.JComposer, c.Composers)
	b.addArrayMetadata(consts.JArtist, c.Artists)
	b.addArrayMetadata(consts.JStudio, c.Studios)
	b.addArrayMetadata(consts.JPerformer, c.Performers)
	b.addArrayMetadata(consts.JProducer, c.Producers)
	b.addArrayMetadata(consts.JPublisher, c.Publishers)
	b.addArrayMetadata(consts.JDirector, c.Directors)
	b.addArrayMetadata(consts.JWriter, c.Writers)
}

// addDates adds all date-related metadata
func (b *ffCommandBuilder) addDates(d *models.MetadataDates) {
	fields := map[string]string{
		consts.JCreationTime:        d.CreationTime,
		consts.JDate:                d.Date,
		consts.JOriginallyAvailable: d.OriginallyAvailableAt,
		consts.JReleaseDate:         d.ReleaseDate,
		consts.JUploadDate:          d.UploadDate,
		consts.JYear:                d.Year,
	}

	for field, value := range fields {
		if value != "" {
			b.metadataMap[field] = value
		}
	}
}

// addShowInfo adds all show info related metadata
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

// addOtherMetadata adds other related metadata
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

// addArrayMetadata combines array values with existing metadata
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
