// Package ffprobe helps determine the metadata already present in a video file.
//
// This is used to determine if a video file should be encoded.
package ffprobe

import (
	"metarr/internal/domain/consts"
	"metarr/internal/domain/logger"
	"metarr/internal/models"
	"metarr/internal/parsing"
	"strings"
)

// ffprobeFormat contains metadata tags.
type ffprobeFormat struct {
	Tags ffprobeTags `json:"tags"`
}

// ffprobeOutput contains top-level FFprobe output.
type ffprobeOutput struct {
	Streams []ffprobeStream `json:"streams"`
	Format  ffprobeFormat   `json:"format"`
}

// ffprobeTags is a map of metadata key-value pairs.
//
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
				existing: strings.TrimSpace(ffData.Format.Tags.get(consts.JArtist)), // FFprobe value.
				new:      strings.TrimSpace(fd.MCredits.Artist),                     // Desired new value.
			},
			consts.JComposer: {
				existing: strings.TrimSpace(ffData.Format.Tags.get(consts.JComposer)),
				new:      strings.TrimSpace(fd.MCredits.Composer),
			},
			consts.JCreationTime: {
				existing: getDatePart(ffData.Format.Tags.get(consts.JCreationTime)),
				new:      getDatePart(fd.MDates.CreationTime),
			},
			consts.JDate: {
				existing: getDatePart(ffData.Format.Tags.get(consts.JDate)),
				new:      getDatePart(fd.MDates.Date),
			},
			consts.JDescription: {
				existing: strings.TrimSpace(ffData.Format.Tags.get(consts.JDescription)),
				new:      strings.TrimSpace(fd.MTitleDesc.Description),
			},
			consts.JSynopsis: {
				existing: strings.TrimSpace(ffData.Format.Tags.get(consts.JSynopsis)),
				new:      strings.TrimSpace(fd.MTitleDesc.Synopsis),
			},
			consts.JTitle: {
				existing: strings.TrimSpace(ffData.Format.Tags.get(consts.JTitle)),
				new:      strings.TrimSpace(fd.MTitleDesc.Title),
			},
		}, true

	case consts.ExtMKV,
		consts.ExtWEBM:

		return tagDiffMap{
			parsing.GetContainerKeys(consts.JArtist, e): {
				existing: strings.TrimSpace(ffData.Format.Tags.get("ARTIST")),
				new:      strings.TrimSpace(fd.MCredits.Artist),
			},
			parsing.GetContainerKeys(consts.JActor, e): {
				existing: strings.TrimSpace(ffData.Format.Tags.get("LEAD_PERFORMER")),
				new:      strings.TrimSpace(fd.MCredits.Actor),
			},
			parsing.GetContainerKeys(consts.JComposer, e): {
				existing: strings.TrimSpace(ffData.Format.Tags.get("COMPOSER")),
				new:      strings.TrimSpace(fd.MCredits.Composer),
			},
			parsing.GetContainerKeys(consts.JReleaseDate, e): {
				existing: getDatePart(ffData.Format.Tags.get("DATE_RELEASED")),
				new:      getDatePart(fd.MDates.ReleaseDate),
			},
			parsing.GetContainerKeys(consts.JLongDesc, e): {
				existing: strings.TrimSpace(ffData.Format.Tags.get("DESCRIPTION")),
				new:      strings.TrimSpace(fd.MTitleDesc.LongDescription),
			},
			parsing.GetContainerKeys(consts.JSummary, e): {
				existing: strings.TrimSpace(ffData.Format.Tags.get("SUMMARY")),
				new:      strings.TrimSpace(fd.MTitleDesc.Summary),
			},
			parsing.GetContainerKeys(consts.JDescription, e): {
				existing: strings.TrimSpace(ffData.Format.Tags.get("SUBJECT")),
				new:      strings.TrimSpace(fd.MTitleDesc.Description),
			},
			parsing.GetContainerKeys(consts.JTitle, e): {
				existing: strings.TrimSpace(ffData.Format.Tags.get("TITLE")),
				new:      strings.TrimSpace(fd.MTitleDesc.Title),
			},
		}, true

	case consts.ExtASF,
		consts.ExtWMV:

		return tagDiffMap{
			parsing.GetContainerKeys(consts.JTitle, e): {
				existing: strings.TrimSpace(ffData.Format.Tags.get("Title")),
				new:      strings.TrimSpace(fd.MTitleDesc.Title),
			},
			parsing.GetContainerKeys(consts.JArtist, e): {
				existing: strings.TrimSpace(ffData.Format.Tags.get("WM/AlbumArtist")),
				new:      strings.TrimSpace(fd.MCredits.Artist),
			},
			parsing.GetContainerKeys(consts.JComposer, e): {
				existing: strings.TrimSpace(ffData.Format.Tags.get("WM/Composer")),
				new:      strings.TrimSpace(fd.MCredits.Composer),
			},
			parsing.GetContainerKeys(consts.JDirector, e): {
				existing: strings.TrimSpace(ffData.Format.Tags.get("WM/Director")),
				new:      strings.TrimSpace(fd.MCredits.Director),
			},
			parsing.GetContainerKeys(consts.JDate, e): {
				existing: getDatePart(ffData.Format.Tags.get("WM/EncodingTime")),
				new:      getDatePart(fd.MDates.Date),
			},
			parsing.GetContainerKeys(consts.JProducer, e): {
				existing: strings.TrimSpace(ffData.Format.Tags.get("WM/Producer")),
				new:      strings.TrimSpace(fd.MCredits.Producer),
			},
			parsing.GetContainerKeys(consts.JSubtitle, e): {
				existing: strings.TrimSpace(ffData.Format.Tags.get("WM/SubTitle")),
				new:      strings.TrimSpace(fd.MTitleDesc.Subtitle),
			},
			parsing.GetContainerKeys(consts.JLongDesc, e): {
				existing: strings.TrimSpace(ffData.Format.Tags.get("WM/SubTitleDescription")),
				new:      strings.TrimSpace(fd.MTitleDesc.LongDescription),
			},
			parsing.GetContainerKeys(consts.JYear, e): {
				existing: getDatePart(ffData.Format.Tags.get("WM/Year")),
				new:      getDatePart(fd.MDates.Year),
			},
		}, true

	case consts.ExtOGM,
		consts.ExtOGV:

		return tagDiffMap{
			parsing.GetContainerKeys(consts.JArtist, e): {
				existing: strings.TrimSpace(ffData.Format.Tags.get("ARTIST")),
				new:      strings.TrimSpace(fd.MCredits.Artist),
			},
			parsing.GetContainerKeys(consts.JPerformer, e): {
				existing: strings.TrimSpace(ffData.Format.Tags.get("PERFORMER")),
				new:      strings.TrimSpace(fd.MCredits.Artist),
			},
			parsing.GetContainerKeys(consts.JComposer, e): {
				existing: strings.TrimSpace(ffData.Format.Tags.get("COMPOSER")),
				new:      strings.TrimSpace(fd.MCredits.Composer),
			},
			parsing.GetContainerKeys(consts.JDate, e): {
				existing: getDatePart(ffData.Format.Tags.get("DATE")),
				new:      getDatePart(fd.MDates.Date),
			},
			parsing.GetContainerKeys(consts.JDescription, e): {
				existing: strings.TrimSpace(ffData.Format.Tags.get("DESCRIPTION")),
				new:      strings.TrimSpace(fd.MTitleDesc.Description),
			},
			parsing.GetContainerKeys(consts.JSummary, e): {
				existing: strings.TrimSpace(ffData.Format.Tags.get("SUMMARY")),
				new:      strings.TrimSpace(fd.MTitleDesc.Summary),
			},
		}, true

	case consts.ExtAVI:
		return tagDiffMap{
			parsing.GetContainerKeys(consts.JLongDesc, e): { // Comments.
				existing: strings.TrimSpace(ffData.Format.Tags.get("COMM")),
				new:      strings.TrimSpace(fd.MTitleDesc.LongDescription),
			},
			parsing.GetContainerKeys(consts.JArtist, e): { // Artist.
				existing: strings.TrimSpace(ffData.Format.Tags.get("IART")),
				new:      strings.TrimSpace(fd.MCredits.Artist),
			},
			parsing.GetContainerKeys(consts.JDescription, e): { // Comment.
				existing: strings.TrimSpace(ffData.Format.Tags.get("ICMT")),
				new:      strings.TrimSpace(fd.MTitleDesc.Description),
			},
			parsing.GetContainerKeys(consts.JReleaseDate, e): { // Date created.
				existing: getDatePart(ffData.Format.Tags.get("ICRD")),
				new:      getDatePart(fd.MDates.ReleaseDate),
			},
			parsing.GetContainerKeys(consts.JProducer, e): { // Engineer.
				existing: strings.TrimSpace(ffData.Format.Tags.get("IENG")),
				new:      strings.TrimSpace(fd.MCredits.Producer),
			},
			parsing.GetContainerKeys(consts.JTitle, e): { // Title.
				existing: strings.TrimSpace(ffData.Format.Tags.get("INAM")),
				new:      strings.TrimSpace(fd.MTitleDesc.Title),
			},
			parsing.GetContainerKeys(consts.JSynopsis, e): { // Subject.
				existing: strings.TrimSpace(ffData.Format.Tags.get("ISBJ")),
				new:      strings.TrimSpace(fd.MTitleDesc.Synopsis),
			},
			parsing.GetContainerKeys(consts.JActor, e): { // Starring.
				existing: strings.TrimSpace(ffData.Format.Tags.get("STAR")),
				new:      strings.TrimSpace(fd.MCredits.Actor),
			},
			parsing.GetContainerKeys(consts.JYear, e): { // Year.
				existing: getDatePart(ffData.Format.Tags.get("YEAR")),
				new:      getDatePart(fd.MDates.Year),
			},
		}, true

	case consts.ExtFLV:
		return tagDiffMap{
			parsing.GetContainerKeys(consts.JDate, e): {
				existing: getDatePart(ffData.Format.Tags.get("creationdate")),
				new:      getDatePart(fd.MDates.Date),
			},
		}, true

	case consts.ExtRM,
		consts.ExtRMVB:
		return tagDiffMap{
			parsing.GetContainerKeys(consts.JAuthor, e): {
				existing: strings.TrimSpace(ffData.Format.Tags.get("Author")),
				new:      strings.TrimSpace(fd.MCredits.Author),
			},
			parsing.GetContainerKeys(consts.JDescription, e): {
				existing: strings.TrimSpace(ffData.Format.Tags.get("Comment")),
				new:      strings.TrimSpace(fd.MTitleDesc.Description),
			},
			parsing.GetContainerKeys(consts.JTitle, e): {
				existing: strings.TrimSpace(ffData.Format.Tags.get("Title")),
				new:      strings.TrimSpace(fd.MTitleDesc.Title),
			},
		}, true

	case consts.ExtMTS,
		consts.ExtTS:
		return tagDiffMap{
			parsing.GetContainerKeys(consts.JArtist, e): {
				existing: strings.TrimSpace(ffData.Format.Tags.get("service_provider")),
				new:      strings.TrimSpace(fd.MCredits.Artist),
			},
			parsing.GetContainerKeys(consts.JTitle, e): {
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
