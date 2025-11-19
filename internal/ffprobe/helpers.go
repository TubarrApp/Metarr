// Package ffprobe helps determine the metadata already present in a video file.
//
// This is used to determine if a video file should be encoded.
package ffprobe

import (
	"metarr/internal/domain/consts"
	"metarr/internal/domain/logger"
	"metarr/internal/models"
	"strings"
)

type ffprobeFormat struct {
	Tags ffprobeTags `json:"tags"`
}

type ffprobeOutput struct {
	Streams []ffprobeStream `json:"streams"`
	Format  ffprobeFormat   `json:"format"`
}

// ffprobeTags is a map of metadata key-value pairs.
// Different container formats use different key names (e.g., "artist" vs "ARTIST" vs "WM/AlbumArtist").
type ffprobeTags map[string]string

type ffprobeStream struct {
	Index       int    `json:"index"`
	CodecType   string `json:"codec_type"`
	CodecName   string `json:"codec_name"`
	Disposition struct {
		AttachedPic int `json:"attached_pic"`
	} `json:"disposition"`
}

// getDiffMapForFiletype returns a struct map with values.
func getDiffMapForFiletype(e string, fd *models.FileData, ffData ffprobeOutput) (tagMap tagDiffMap, exists bool) {
	switch e {
	case consts.Ext3GP,
		consts.Ext3G2,
		consts.ExtF4V,
		consts.ExtM4V,
		consts.ExtMOV,
		consts.ExtMP4:

		return tagDiffMap{
			consts.JArtist: { // FFprobe access key.
				existing: strings.TrimSpace(ffData.Format.Tags.get("artist")), // FFprobe value.
				new:      strings.TrimSpace(fd.MCredits.Artist),               // Desired new value.
			},
			consts.JComposer: {
				existing: strings.TrimSpace(ffData.Format.Tags.get("composer")),
				new:      strings.TrimSpace(fd.MCredits.Composer),
			},
			consts.JCreationTime: {
				existing: getDatePart(ffData.Format.Tags.get("creation_time")),
				new:      getDatePart(fd.MDates.CreationTime),
			},
			consts.JDate: {
				existing: strings.TrimSpace(ffData.Format.Tags.get("date")),
				new:      strings.TrimSpace(fd.MDates.Date),
			},
			consts.JDescription: {
				existing: strings.TrimSpace(ffData.Format.Tags.get("description")),
				new:      strings.TrimSpace(fd.MTitleDesc.Description),
			},
			consts.JSynopsis: {
				existing: strings.TrimSpace(ffData.Format.Tags.get("synopsis")),
				new:      strings.TrimSpace(fd.MTitleDesc.Synopsis),
			},
			consts.JTitle: {
				existing: strings.TrimSpace(ffData.Format.Tags.get("title")),
				new:      strings.TrimSpace(fd.MTitleDesc.Title),
			},
		}, true

	case consts.ExtMKV,
		consts.ExtWEBM:

		return tagDiffMap{
			"ARTIST/LEAD_PERFORMER": {
				existing: strings.TrimSpace(ffData.Format.Tags.get("ARTIST")),
				new:      strings.TrimSpace(fd.MCredits.Artist),
			},
			"COMPOSER": {
				existing: strings.TrimSpace(ffData.Format.Tags.get("COMPOSER")),
				new:      strings.TrimSpace(fd.MCredits.Composer),
			},
			"DATE_RELEASED": {
				existing: getDatePart(ffData.Format.Tags.get("DATE_RELEASED")),
				new:      getDatePart(fd.MDates.Date),
			},
			"DESCRIPTION/SUMMARY": {
				existing: strings.TrimSpace(ffData.Format.Tags.get("DESCRIPTION")),
				new:      strings.TrimSpace(fd.MTitleDesc.Description),
			},
			"SUBJECT/KEYWORDS": {
				existing: strings.TrimSpace(ffData.Format.Tags.get("KEYWORDS")),
				new:      strings.TrimSpace(fd.MTitleDesc.LongDescription),
			},
			"TITLE": {
				existing: strings.TrimSpace(ffData.Format.Tags.get("TITLE")),
				new:      strings.TrimSpace(fd.MTitleDesc.Title),
			},
		}, true

	case consts.ExtASF,
		consts.ExtWMV:

		return tagDiffMap{
			"Title": {
				existing: strings.TrimSpace(ffData.Format.Tags.get("Title")),
				new:      strings.TrimSpace(fd.MTitleDesc.Title),
			},
			"WM/AlbumArtist": {
				existing: strings.TrimSpace(ffData.Format.Tags.get("WM/AlbumArtist")),
				new:      strings.TrimSpace(fd.MCredits.Artist),
			},
			"WM/Composer": {
				existing: strings.TrimSpace(ffData.Format.Tags.get("WM/Composer")),
				new:      strings.TrimSpace(fd.MCredits.Composer),
			},
			"WM/Director": {
				existing: strings.TrimSpace(ffData.Format.Tags.get("WM/Director")),
				new:      strings.TrimSpace(fd.MCredits.Director),
			},
			"WM/EncodingTime": {
				existing: getDatePart(ffData.Format.Tags.get("WM/EncodingTime")),
				new:      getDatePart(fd.MDates.Date),
			},
			"WM/Producer": {
				existing: strings.TrimSpace(ffData.Format.Tags.get("WM/Producer")),
				new:      strings.TrimSpace(fd.MCredits.Producer),
			},
			"WM/SubTitle": {
				existing: strings.TrimSpace(ffData.Format.Tags.get("WM/SubTitle")),
				new:      strings.TrimSpace(fd.MTitleDesc.Description),
			},
			"WM/SubTitleDescription": {
				existing: strings.TrimSpace(ffData.Format.Tags.get("WM/SubTitleDescription")),
				new:      strings.TrimSpace(fd.MTitleDesc.Description),
			},
			"WM/Year": {
				existing: getDatePart(ffData.Format.Tags.get("WM/Year")),
				new:      getDatePart(fd.MDates.Year),
			},
		}, true

	case consts.ExtOGM,
		consts.ExtOGV:

		return tagDiffMap{
			"ARTIST/PERFORMER": {
				existing: strings.TrimSpace(ffData.Format.Tags.get("ARTIST")),
				new:      strings.TrimSpace(fd.MCredits.Artist),
			},
			"COMPOSER": {
				existing: strings.TrimSpace(ffData.Format.Tags.get("COMPOSER")),
				new:      strings.TrimSpace(fd.MCredits.Composer),
			},
			"DATE": {
				existing: getDatePart(ffData.Format.Tags.get("DATE")),
				new:      getDatePart(fd.MDates.Date),
			},
			"DESCRIPTION": {
				existing: strings.TrimSpace(ffData.Format.Tags.get("DESCRIPTION")),
				new:      strings.TrimSpace(fd.MTitleDesc.Description),
			},
			"DESCRIPTION/SUMMARY": {
				existing: strings.TrimSpace(ffData.Format.Tags.get("DESCRIPTION")),
				new:      strings.TrimSpace(fd.MTitleDesc.Synopsis),
			},
		}, true

	case consts.ExtAVI:
		return tagDiffMap{
			"IART": {
				existing: strings.TrimSpace(ffData.Format.Tags.get("IART")),
				new:      strings.TrimSpace(fd.MCredits.Artist),
			},
			"ICRD": {
				existing: getDatePart(ffData.Format.Tags.get("ICRD")),
				new:      getDatePart(fd.MDates.Date),
			},
			"ICMT": {
				existing: strings.TrimSpace(ffData.Format.Tags.get("ICMT")),
				new:      strings.TrimSpace(fd.MTitleDesc.Synopsis),
			},
			"IENG": {
				existing: strings.TrimSpace(ffData.Format.Tags.get("IENG")),
				new:      strings.TrimSpace(fd.MCredits.Composer),
			},
			"INAM": {
				existing: strings.TrimSpace(ffData.Format.Tags.get("INAM")),
				new:      strings.TrimSpace(fd.MTitleDesc.Title),
			},
			"ISBJ": {
				existing: strings.TrimSpace(ffData.Format.Tags.get("ISBJ")),
				new:      strings.TrimSpace(fd.MTitleDesc.Description),
			},
			"ITCH": {
				existing: strings.TrimSpace(ffData.Format.Tags.get("ITCH")),
				new:      strings.TrimSpace(fd.MCredits.Composer),
			},
		}, true

	case consts.ExtFLV:
		return tagDiffMap{
			"creationdate": {
				existing: getDatePart(ffData.Format.Tags.get("creationdate")),
				new:      getDatePart(fd.MDates.Date),
			},
		}, true

	case consts.ExtRM,
		consts.ExtRMVB:
		return tagDiffMap{
			"Author": {
				existing: strings.TrimSpace(ffData.Format.Tags.get("Author")),
				new:      strings.TrimSpace(fd.MCredits.Author),
			},
			"Comment": {
				existing: strings.TrimSpace(ffData.Format.Tags.get("Comment")),
				new:      strings.TrimSpace(fd.MTitleDesc.Description),
			},
			"Title": {
				existing: strings.TrimSpace(ffData.Format.Tags.get("Title")),
				new:      strings.TrimSpace(fd.MTitleDesc.Title),
			},
		}, true

	case consts.ExtMTS,
		consts.ExtTS:
		return tagDiffMap{
			"service_provider": {
				existing: strings.TrimSpace(ffData.Format.Tags.get("service_provider")),
				new:      strings.TrimSpace(fd.MCredits.Artist),
			},
			"service_name": {
				existing: strings.TrimSpace(ffData.Format.Tags.get("service_name")),
				new:      strings.TrimSpace(fd.MTitleDesc.Title),
			},
		}, true
	}
	return tagDiffMap{}, false
}

// getDatePart safely extracts the date part before 'T' if it exists.
func getDatePart(timeStr string) string {
	timeStr = strings.TrimSpace(timeStr)
	if beforeT, _, _ := strings.Cut(timeStr, "T"); beforeT != "" {
		return beforeT
	}
	return timeStr
}

// printArray provides a simple print of metadata captured by FFprobe.
func printArray(s []string) {
	str := strings.Join(s, ", ")
	logger.Pl.I("FFprobe captured %s", str)
}

// get grabs key based on casing.
func (tags ffprobeTags) get(key string) string {
	// Direct match.
	if k, ok := tags[key]; ok {
		return k
	}

	// Try variants.
	if k, ok := tags[strings.ToLower(key)]; ok {
		return k
	}
	if k, ok := tags[strings.ToUpper(key)]; ok {
		return k
	}
	if k, ok := tags[strings.ToTitle(key)]; ok {
		return k
	}
	return ""
}
