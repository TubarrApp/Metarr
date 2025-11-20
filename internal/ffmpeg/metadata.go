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

// getContainerKeys returns all valid tag names for the given canonical key and container type.
func getContainerKeys(key, extension string) []string {
	switch extension {
	case consts.Ext3GP,
		consts.Ext3G2,
		consts.ExtF4V,
		consts.ExtM4V,
		consts.ExtMOV,
		consts.ExtMP4:
		// Containers use lowercase keys (already stored as lowercase).
		return []string{key}

	case consts.ExtMKV,
		consts.ExtWEBM:
		// Matroska uses UPPERCASE tags.
		switch key {
		case consts.JArtist:
			return []string{"ARTIST", "LEAD_PERFORMER"}
		case consts.JComposer:
			return []string{"COMPOSER"}
		case consts.JPerformer:
			return []string{"PERFORMER"}
		case consts.JProducer:
			return []string{"PRODUCER"}
		case consts.JDirector:
			return []string{"DIRECTOR"}
		case consts.JTitle:
			return []string{"TITLE"}
		case consts.JDescription:
			return []string{"DESCRIPTION", "SUMMARY"}
		case consts.JSummary:
			return []string{"SUMMARY"}
		case consts.JSynopsis:
			return []string{"SYNOPSIS"}
		case consts.JLongDesc:
			return []string{"SUBJECT", "KEYWORDS"}
		case consts.JReleaseDate, consts.JDate:
			return []string{"DATE_RELEASED"}
		case consts.JCreationTime:
			return []string{"DATE_ENCODED"}
		case consts.JYear:
			return []string{"DATE_RELEASED"}
		default:
			// For unknown keys, try uppercase
			return []string{strings.ToUpper(key)}
		}

	case consts.ExtWMV,
		consts.ExtASF:
		// WMV uses TitleCase and WM/ prefixes.
		switch key {
		case consts.JTitle:
			return []string{"Title"}
		case consts.JArtist:
			return []string{"WM/AlbumArtist"}
		case consts.JComposer:
			return []string{"WM/Composer"}
		case consts.JDirector:
			return []string{"WM/Director"}
		case consts.JProducer:
			return []string{"WM/Producer"}
		case consts.JDescription:
			return []string{"WM/SubTitle", "WM/SubTitleDescription"}
		case consts.JDate:
			return []string{"WM/EncodingTime"}
		case consts.JYear:
			return []string{"WM/Year"}
		default:
			return []string{key}
		}

	case consts.ExtOGM,
		consts.ExtOGV:
		// Ogg uses UPPERCASE Vorbis comments.
		switch key {
		case consts.JArtist:
			return []string{"ARTIST", "PERFORMER"}
		case consts.JComposer:
			return []string{"COMPOSER"}
		case consts.JDescription:
			return []string{"DESCRIPTION"}
		case consts.JSummary, consts.JSynopsis:
			return []string{"SUMMARY"}
		case consts.JDate:
			return []string{"DATE"}
		case consts.JTitle:
			return []string{"TITLE"}
		default:
			return []string{strings.ToUpper(key)}
		}

	case consts.ExtAVI:
		// AVI uses RIFF INFO tags (4-character codes).
		switch key {
		case consts.JArtist:
			return []string{"IART"}
		case consts.JComposer:
			return []string{"IENG", "ITCH"}
		case consts.JTitle:
			return []string{"INAM"}
		case consts.JDescription:
			return []string{"ISBJ"}
		case consts.JSynopsis:
			return []string{"ICMT"}
		case consts.JDate:
			return []string{"ICRD"}
		default:
			return []string{key}
		}

	case consts.ExtFLV:
		// FLV uses lowercase tags.
		switch key {
		case consts.JDate:
			return []string{"creationdate"}
		default:
			return []string{strings.ToLower(key)}
		}

	case consts.ExtRM,
		consts.ExtRMVB:
		// RealMedia uses TitleCase.
		switch key {
		case consts.JAuthor:
			return []string{"Author"}
		case consts.JDescription:
			return []string{"Comment"}
		case consts.JTitle:
			return []string{"Title"}
		default:
			return []string{key}
		}

	case consts.ExtMTS,
		consts.ExtTS:
		// MPEG-TS uses specific service tags.
		switch key {
		case consts.JArtist:
			return []string{"service_provider"}
		case consts.JTitle:
			return []string{"service_name"}
		default:
			return []string{key}
		}

	default:
		// For unknown container types, use input key.
		return []string{key}
	}
}
