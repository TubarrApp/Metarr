package metaconversion

import (
	"fmt"
	"metarr/internal/domain/logger"
	"metarr/internal/models"
	"path/filepath"
	"strings"

	"github.com/TubarrApp/gocommon/sharedconsts"
)

// NFO constants.
const (
	nfoRootMovie   = "movie"
	nfoRootEpisode = "episodedetails"
	xmlDeclaration = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>`
)

// BuildNFOContents builds the NFO contents from the provided FileData.
func BuildNFOContents(fd *models.FileData) (result string, err error) {
	if fd == nil {
		return "", fmt.Errorf("FileData is nil for input, cannot build NFO contents")
	}

	if strings.ToLower(filepath.Ext(fd.MetaFilePath)) == sharedconsts.MExtNFO {
		logger.Pl.D(1, "Meta file path %q already NFO, skipping NFO conversion...", fd.MetaFilePath)
		return "", nil
	}

	tags := ""
	// Add title.
	if fd.MTitleDesc != nil {
		tags += addTitlesAndDescriptionsToNFO(fd.MTitleDesc)
	}

	// Add year.
	if fd.MDates != nil {
		tags += addDatesToNFO(fd.MDates)
	}

	// Add show data.
	if fd.MShowData != nil {
		tags += addShowDataToNFO(fd.MShowData)
	}

	// Add credits to NFO.
	if fd.MCredits != nil {
		tags += addCreditsToNFO(fd.MCredits)
	}

	if tags == "" {
		logger.Pl.D(1, "No metadata fields populated for %q, no NFO contents to write", fd.MetaFilePath)
		return "", nil
	}

	root := nfoRootFor(fd.MShowData)
	return fmt.Sprintf("%s\n<%s>\n%s</%s>\n", xmlDeclaration, root, tags, root), nil
}

// nfoRootFor picks the NFO root element based on whether the file looks like a TV episode.
func nfoRootFor(s *models.MetadataShowData) string {
	if s == nil {
		return nfoRootMovie
	}
	if s.EpisodeID != "" || s.SeasonNumber != "" {
		return nfoRootEpisode
	}
	return nfoRootMovie
}

// addTitlesAndDescriptionsToNFO adds title and description information to the NFO string based on the provided MetadataTitleDesc.
func addTitlesAndDescriptionsToNFO(t *models.MetadataTitlesDescs) (o string) {
	if t == nil {
		logger.Pl.W("MetadataTitlesDescs is nil, skipping titles and descriptions addition to NFO.")
		return ""
	}

	// Add title.
	if t.Fulltitle != "" {
		o += indentNFO(fmt.Sprintf("<title>%s</title>\n", escapeNFO(t.Fulltitle)), 1)
	} else if t.Title != "" {
		o += indentNFO(fmt.Sprintf("<title>%s</title>\n", escapeNFO(t.Title)), 1)
	}

	// Add plot.
	if t.Description != "" {
		o += indentNFO(fmt.Sprintf("<plot>%s</plot>\n", escapeNFO(t.Description)), 1)
	} else if t.LongDescription != "" {
		o += indentNFO(fmt.Sprintf("<plot>%s</plot>\n", escapeNFO(t.LongDescription)), 1)
	}

	// Add outline.
	if t.Subtitle != "" {
		o += indentNFO(fmt.Sprintf("<outline>%s</outline>\n", escapeNFO(t.Subtitle)), 1)
	} else if t.Synopsis != "" {
		o += indentNFO(fmt.Sprintf("<outline>%s</outline>\n", escapeNFO(t.Synopsis)), 1)
	} else if t.Summary != "" {
		o += indentNFO(fmt.Sprintf("<outline>%s</outline>\n", escapeNFO(t.Summary)), 1)
	}

	return o
}

// addDatesToNFO adds date information to the NFO string based on the provided MetadataDates.
func addDatesToNFO(d *models.MetadataDates) (o string) {
	if d == nil {
		logger.Pl.W("MetadataDates is nil, skipping dates addition to NFO.")
		return ""
	}

	// Year.
	if d.Year != "" {
		o += indentNFO(fmt.Sprintf("<year>%s</year>\n", escapeNFO(d.Year)), 1)
	}

	// Premiered.
	if premiered := firstNonEmpty(d.OriginallyAvailableAt, d.CreationTime, d.ReleaseDate); premiered != "" {
		o += indentNFO(fmt.Sprintf("<premiered>%s</premiered>\n", escapeNFO(premiered)), 1)
	}

	// Released.
	if released := firstNonEmpty(d.ReleaseDate, d.OriginallyAvailableAt, d.CreationTime); released != "" {
		o += indentNFO(fmt.Sprintf("<released>%s</released>\n", escapeNFO(released)), 1)
	}

	return o
}

// addCreditsToNFO adds credits information to the NFO string based on the provided MetadataCredits.
func addCreditsToNFO(c *models.MetadataCredits) (o string) {
	if c == nil {
		logger.Pl.W("MetadataCredits is nil, skipping credits addition to NFO.")
		return ""
	}

	// Add directors.
	for _, d := range makeUniqueArray(c.Directors, c.Director) {
		o += indentNFO(fmt.Sprintf("<director>%s</director>\n", escapeNFO(d)), 1)
	}

	// Add producers.
	for _, p := range makeUniqueArray(c.Producers, c.Producer) {
		o += indentNFO(fmt.Sprintf("<producer>%s</producer>\n", escapeNFO(p)), 1)
	}

	// Add actors.
	for _, a := range makeUniqueArray(c.Actors, c.Actor) {
		o += indentNFO("<actor>\n", 1)
		o += indentNFO(fmt.Sprintf("<name>%s</name>\n", escapeNFO(a)), 2)
		o += indentNFO("</actor>\n", 1)
	}

	// Add writers.
	for _, w := range makeUniqueArray(c.Writers, c.Writer) {
		o += indentNFO(fmt.Sprintf("<writer>%s</writer>\n", escapeNFO(w)), 1)
	}

	// Add publishers.
	for _, pub := range makeUniqueArray(c.Publishers, c.Publisher) {
		o += indentNFO(fmt.Sprintf("<publisher>%s</publisher>\n", escapeNFO(pub)), 1)
	}

	return o
}

// addShowDataToNFO adds show data information to the NFO string based on the provided MetadataShowData.
func addShowDataToNFO(s *models.MetadataShowData) (o string) {
	if s == nil {
		logger.Pl.W("MetadataShowData is nil, skipping show data addition to NFO.")
		return ""
	}

	// Under an episodedetails root the show name belongs in showtitle. Else use set for movies.
	if s.Show != "" {
		if nfoRootFor(s) == nfoRootEpisode {
			o += indentNFO(fmt.Sprintf("<showtitle>%s</showtitle>\n", escapeNFO(s.Show)), 1)
		} else {
			o += indentNFO(fmt.Sprintf("<set>%s</set>\n", escapeNFO(s.Show)), 1)
		}
	}
	if s.EpisodeID != "" {
		o += indentNFO(fmt.Sprintf("<episode>%s</episode>\n", escapeNFO(s.EpisodeID)), 1)
	}
	if s.SeasonTitle != "" {
		o += indentNFO(fmt.Sprintf("<seasontitle>%s</seasontitle>\n", escapeNFO(s.SeasonTitle)), 1)
	}
	if s.SeasonNumber != "" {
		o += indentNFO(fmt.Sprintf("<season>%s</season>\n", escapeNFO(s.SeasonNumber)), 1)
	}

	return o
}

// firstNonEmpty returns the first non-empty string in the provided values.
func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}
