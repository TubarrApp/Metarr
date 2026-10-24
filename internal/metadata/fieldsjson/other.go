package fieldsjson

import (
	"cmp"
	"metarr/internal/models"
	"strconv"
	"strings"

	"github.com/TubarrApp/gocommon/sharedtags"
)

// fillShowData fills show details, such as the series, season and episode.
func fillShowData(fd *models.FileData, json map[string]any) {
	s := fd.MShowData
	s.Show = jsonText(json, sharedtags.JShow, sharedtags.JSeries)
	s.SeasonNumber = jsonText(json, sharedtags.JSeasonNumber)
	s.SeasonTitle = jsonText(json, sharedtags.JSeasonTitle, sharedtags.JSeason)
	s.EpisodeID = jsonText(json, sharedtags.JEpisodeID)
	s.EpisodeSort = jsonText(json, sharedtags.JEpisodeSort, sharedtags.JEpisodeNumber)

	// The network falls back to the channel.
	s.Network = cmp.Or(jsonText(json, sharedtags.JNetwork), fd.MCredits.Channel)
}

// fillOtherData fills the remaining details, such as the genre, album and keywords.
func fillOtherData(fd *models.FileData, json map[string]any) {
	o := fd.MOther
	o.Album = jsonText(json, sharedtags.JAlbum)
	o.Copyright = jsonText(json, sharedtags.JCopyright, sharedtags.JLicense)
	o.Disc = jsonText(json, sharedtags.JDiscNumber)
	o.Genre = jsonText(json, sharedtags.JGenre, sharedtags.JGenres, sharedtags.JCategories)
	o.Keywords = jsonText(json, sharedtags.JKeywords, sharedtags.JTags)
	o.Language = jsonText(json, sharedtags.JLanguage)
	o.Track = jsonText(json, sharedtags.JTrackNumber)

	// HD video falls back to one worked out from the video's height.
	o.HDVideo = cmp.Or(jsonText(json, sharedtags.JHDVideo), hdVideoFromHeight(json[sharedtags.JHeight]))
}

// jsonText returns the value of the first key holding text, a number or a list of text, as text.
func jsonText(json map[string]any, keys ...string) string {
	for _, k := range keys {
		switch v := json[k].(type) {
		case string:
			if v != "" {
				return v
			}
		case float64:
			return strconv.FormatFloat(v, 'f', -1, 64)
		case []any:
			// Join the list's text entries.
			parts := make([]string, 0, len(v))
			for _, e := range v {
				if s, ok := e.(string); ok && s != "" {
					parts = append(parts, s)
				}
			}
			if len(parts) > 0 {
				return strings.Join(parts, ", ")
			}
		}
	}
	return ""
}

// hdVideoFromHeight returns the iTunes HD video flag for a height: 2 for 1080p and up, 1 for 720p, and 0 below.
func hdVideoFromHeight(h any) string {
	height, ok := h.(float64)
	if !ok || height <= 0 {
		return ""
	}

	switch {
	case height >= 1080:
		return "2"
	case height >= 720:
		return "1"
	}
	return "0"
}
