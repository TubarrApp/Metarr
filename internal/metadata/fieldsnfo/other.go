package fieldsnfo

import (
	"cmp"
	"metarr/internal/models"
	"strings"
)

// fillNFOShowData fills show details, such as the series, season and episode.
func fillNFOShowData(fd *models.FileData) (filled bool) {
	s := fd.MShowData
	n := fd.NFOData
	s.Show = cmp.Or(strings.TrimSpace(n.ShowInfo.Show), strings.TrimSpace(n.Set))
	s.SeasonNumber = strings.TrimSpace(n.ShowInfo.SeasonNumber)
	s.EpisodeID = strings.TrimSpace(n.ShowInfo.EpisodeID)

	// The network falls back to the channel.
	s.Network = fd.MCredits.Channel
	return s.Show != "" || s.SeasonNumber != "" || s.EpisodeID != ""
}

// fillNFOOtherData fills the remaining details, such as the genre and keywords.
func fillNFOOtherData(fd *models.FileData) (filled bool) {
	o := fd.MOther
	n := fd.NFOData
	o.Genre = strings.Join(n.Genres, ", ")
	o.Keywords = strings.Join(n.Tags, ", ")
	o.Language = strings.TrimSpace(n.Language)
	o.HDVideo = strings.TrimSpace(n.HDVideo)
	return o.Genre != "" || o.Keywords != "" || o.Language != "" || o.HDVideo != ""
}
