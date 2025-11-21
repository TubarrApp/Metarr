// Package ffprobe helps determine the metadata already present in a video file.
//
// This is used to determine if a video file should be encoded.
package ffprobe

import (
	"metarr/internal/domain/consts"
	"metarr/internal/domain/logger"
	"metarr/internal/models"
	"strings"

	"github.com/TubarrApp/gocommon/sharedtags"
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
	// ASF.
	case consts.ExtASF,
		consts.ExtWMV:

		return tagDiffMap{
			sharedtags.ASFArtist: { // FFprobe access key.
				existing: strings.TrimSpace(ffData.Format.Tags.get(sharedtags.ASFArtist)), // FFprobe value. Do not remove "get", keys are hard to predict.
				new:      strings.TrimSpace(fd.MCredits.Artist),                           // Desired new value.
			},
			sharedtags.ASFComposer: {
				existing: strings.TrimSpace(ffData.Format.Tags.get(sharedtags.ASFComposer)),
				new:      strings.TrimSpace(fd.MCredits.Composer),
			},
			sharedtags.ASFDirector: {
				existing: strings.TrimSpace(ffData.Format.Tags.get(sharedtags.ASFDirector)),
				new:      strings.TrimSpace(fd.MCredits.Director),
			},
			sharedtags.ASFEncodingTime: {
				existing: getDatePart(ffData.Format.Tags.get(sharedtags.ASFEncodingTime)),
				new:      getDatePart(fd.MDates.Date),
			},
			sharedtags.ASFProducer: {
				existing: strings.TrimSpace(ffData.Format.Tags.get(sharedtags.ASFProducer)),
				new:      strings.TrimSpace(fd.MCredits.Producer),
			},
			sharedtags.ASFSubtitle: {
				existing: strings.TrimSpace(ffData.Format.Tags.get(sharedtags.ASFSubtitle)),
				new:      strings.TrimSpace(fd.MTitleDesc.Subtitle),
			},
			sharedtags.ASFSubTitleDescription: {
				existing: strings.TrimSpace(ffData.Format.Tags.get(sharedtags.ASFSubTitleDescription)),
				new:      strings.TrimSpace(fd.MTitleDesc.LongDescription),
			},
			sharedtags.ASFTitle: {
				existing: strings.TrimSpace(ffData.Format.Tags.get(sharedtags.ASFTitle)),
				new:      strings.TrimSpace(fd.MTitleDesc.Fulltitle),
			},
			sharedtags.ASFYear: {
				existing: getDatePart(ffData.Format.Tags.get(sharedtags.ASFYear)),
				new:      getDatePart(fd.MDates.Year),
			},
		}, true

	// AVI.
	case consts.ExtAVI:
		return tagDiffMap{
			sharedtags.AVIArtist: {
				existing: strings.TrimSpace(ffData.Format.Tags.get(sharedtags.AVIArtist)),
				new:      strings.TrimSpace(fd.MCredits.Artist),
			},
			sharedtags.AVIComment: {
				existing: strings.TrimSpace(ffData.Format.Tags.get(sharedtags.AVIComment)),
				new:      strings.TrimSpace(fd.MTitleDesc.Description),
			},
			sharedtags.AVIComments: {
				existing: strings.TrimSpace(ffData.Format.Tags.get(sharedtags.AVIComments)),
				new:      strings.TrimSpace(fd.MTitleDesc.LongDescription),
			},
			sharedtags.AVIDateCreated: {
				existing: getDatePart(ffData.Format.Tags.get(sharedtags.AVIDateCreated)),
				new:      getDatePart(fd.MDates.ReleaseDate),
			},
			sharedtags.AVIEngineer: {
				existing: strings.TrimSpace(ffData.Format.Tags.get(sharedtags.AVIEngineer)),
				new:      strings.TrimSpace(fd.MCredits.Producer),
			},
			sharedtags.AVIStar: {
				existing: strings.TrimSpace(ffData.Format.Tags.get(sharedtags.AVIStar)),
				new:      strings.TrimSpace(fd.MCredits.Actor),
			},
			sharedtags.AVISubject: {
				existing: strings.TrimSpace(ffData.Format.Tags.get(sharedtags.AVISubject)),
				new:      strings.TrimSpace(fd.MTitleDesc.Synopsis),
			},
			sharedtags.AVITitle: {
				existing: strings.TrimSpace(ffData.Format.Tags.get(sharedtags.AVITitle)),
				new:      strings.TrimSpace(fd.MTitleDesc.Fulltitle),
			},
			sharedtags.AVIYear: {
				existing: getDatePart(ffData.Format.Tags.get(sharedtags.AVIYear)),
				new:      getDatePart(fd.MDates.Year),
			},
		}, true

	// FLV.
	case consts.ExtFLV:
		return tagDiffMap{
			sharedtags.FLVCreationDate: {
				existing: getDatePart(ffData.Format.Tags.get(sharedtags.FLVCreationDate)),
				new:      getDatePart(fd.MDates.Date),
			},
		}, true

		// ISOBMM.
	case consts.Ext3GP,
		consts.Ext3G2,
		consts.ExtF4V,
		consts.ExtM4V,
		consts.ExtMOV,
		consts.ExtMP4:

		return tagDiffMap{
			sharedtags.ISOArtist: {
				existing: strings.TrimSpace(ffData.Format.Tags.get(sharedtags.ISOArtist)),
				new:      strings.TrimSpace(fd.MCredits.Artist),
			},
			sharedtags.ISOComment: {
				existing: strings.TrimSpace(ffData.Format.Tags.get(sharedtags.ISOComment)),
				new:      strings.TrimSpace(fd.MTitleDesc.Comment),
			},
			sharedtags.ISOComposer: {
				existing: strings.TrimSpace(ffData.Format.Tags.get(sharedtags.ISOComposer)),
				new:      strings.TrimSpace(fd.MCredits.Composer),
			},
			sharedtags.ISOCreationTime: {
				existing: getDatePart(ffData.Format.Tags.get(sharedtags.ISOCreationTime)),
				new:      getDatePart(fd.MDates.CreationTime),
			},
			sharedtags.JDate: {
				existing: getDatePart(ffData.Format.Tags.get(sharedtags.ISODate)),
				new:      getDatePart(fd.MDates.Date),
			},
			sharedtags.ISODescription: {
				existing: strings.TrimSpace(ffData.Format.Tags.get(sharedtags.ISODescription)),
				new:      strings.TrimSpace(fd.MTitleDesc.Description),
			},
			sharedtags.ISOSynopsis: {
				existing: strings.TrimSpace(ffData.Format.Tags.get(sharedtags.ISOSynopsis)),
				new:      strings.TrimSpace(fd.MTitleDesc.Synopsis),
			},
			sharedtags.ISOTitle: {
				existing: strings.TrimSpace(ffData.Format.Tags.get(sharedtags.ISOTitle)),
				new:      strings.TrimSpace(fd.MTitleDesc.Fulltitle),
			},
		}, true

		// Matroska.
	case consts.ExtMKV,
		consts.ExtWEBM:

		return tagDiffMap{
			sharedtags.MatroskaArtist: {
				existing: strings.TrimSpace(ffData.Format.Tags.get(sharedtags.MatroskaArtist)),
				new:      strings.TrimSpace(fd.MCredits.Artist),
			},
			sharedtags.MatroskaComposer: {
				existing: strings.TrimSpace(ffData.Format.Tags.get(sharedtags.MatroskaComposer)),
				new:      strings.TrimSpace(fd.MCredits.Composer),
			},
			sharedtags.MatroskaDateEncoded: {
				existing: getDatePart(ffData.Format.Tags.get(sharedtags.MatroskaDateEncoded)),
				new:      getDatePart(fd.MDates.CreationTime),
			},
			sharedtags.MatroskaDateReleased: {
				existing: getDatePart(ffData.Format.Tags.get(sharedtags.MatroskaDateReleased)),
				new:      getDatePart(fd.MDates.ReleaseDate),
			},
			sharedtags.MatroskaDescription: {
				existing: strings.TrimSpace(ffData.Format.Tags.get(sharedtags.MatroskaDescription)),
				new:      strings.TrimSpace(fd.MTitleDesc.LongDescription),
			},
			sharedtags.MatroskaDirector: {
				existing: strings.TrimSpace(ffData.Format.Tags.get(sharedtags.MatroskaDirector)),
				new:      strings.TrimSpace(fd.MCredits.Director),
			},
			sharedtags.MatroskaLeadPerformer: {
				existing: strings.TrimSpace(ffData.Format.Tags.get(sharedtags.MatroskaLeadPerformer)),
				new:      strings.TrimSpace(fd.MCredits.Actor),
			},
			sharedtags.MatroskaPerformer: {
				existing: strings.TrimSpace(ffData.Format.Tags.get(sharedtags.MatroskaPerformer)),
				new:      strings.TrimSpace(fd.MCredits.Performer),
			},
			sharedtags.MatroskaProducer: {
				existing: strings.TrimSpace(ffData.Format.Tags.get(sharedtags.MatroskaProducer)),
				new:      strings.TrimSpace(fd.MCredits.Producer),
			},
			sharedtags.MatroskaSubject: {
				existing: strings.TrimSpace(ffData.Format.Tags.get(sharedtags.MatroskaSubject)),
				new:      strings.TrimSpace(fd.MTitleDesc.Subtitle),
			},
			sharedtags.MatroskaSummary: {
				existing: strings.TrimSpace(ffData.Format.Tags.get(sharedtags.MatroskaSummary)),
				new:      strings.TrimSpace(fd.MTitleDesc.Summary),
			},
			sharedtags.MatroskaSynopsis: {
				existing: strings.TrimSpace(ffData.Format.Tags.get(sharedtags.MatroskaSynopsis)),
				new:      strings.TrimSpace(fd.MTitleDesc.Synopsis),
			},
			sharedtags.MatroskaTitle: {
				existing: strings.TrimSpace(ffData.Format.Tags.get(sharedtags.MatroskaTitle)),
				new:      strings.TrimSpace(fd.MTitleDesc.Fulltitle),
			},
		}, true

	// MPEG-TS.
	case consts.ExtMTS,
		consts.ExtTS:
		return tagDiffMap{
			sharedtags.TSServiceName: {
				existing: strings.TrimSpace(ffData.Format.Tags.get(sharedtags.TSServiceName)),
				new:      strings.TrimSpace(fd.MTitleDesc.Fulltitle),
			},
			sharedtags.TSServiceProvider: {
				existing: strings.TrimSpace(ffData.Format.Tags.get(sharedtags.TSServiceProvider)),
				new:      strings.TrimSpace(fd.MCredits.Artist),
			},
		}, true

		// Ogg.
	case consts.ExtOGM,
		consts.ExtOGV:

		return tagDiffMap{
			sharedtags.OggArtist: {
				existing: strings.TrimSpace(ffData.Format.Tags.get(sharedtags.OggArtist)),
				new:      strings.TrimSpace(fd.MCredits.Artist),
			},
			sharedtags.OggComposer: {
				existing: strings.TrimSpace(ffData.Format.Tags.get(sharedtags.OggComposer)),
				new:      strings.TrimSpace(fd.MCredits.Composer),
			},
			sharedtags.OggDate: {
				existing: getDatePart(ffData.Format.Tags.get(sharedtags.OggDate)),
				new:      getDatePart(fd.MDates.Date),
			},
			sharedtags.OggDescription: {
				existing: strings.TrimSpace(ffData.Format.Tags.get(sharedtags.OggDescription)),
				new:      strings.TrimSpace(fd.MTitleDesc.Description),
			},
			sharedtags.OggPerformer: {
				existing: strings.TrimSpace(ffData.Format.Tags.get(sharedtags.OggPerformer)),
				new:      strings.TrimSpace(fd.MCredits.Performer),
			},
			sharedtags.OggSummary: {
				existing: strings.TrimSpace(ffData.Format.Tags.get(sharedtags.OggSummary)),
				new:      strings.TrimSpace(fd.MTitleDesc.Summary),
			},
			sharedtags.OggTitle: {
				existing: strings.TrimSpace(ffData.Format.Tags.get(sharedtags.OggTitle)),
				new:      strings.TrimSpace(fd.MTitleDesc.Fulltitle),
			},
		}, true

		// RealMedia.
	case consts.ExtRM,
		consts.ExtRMVB:
		return tagDiffMap{
			sharedtags.RMAuthor: {
				existing: strings.TrimSpace(ffData.Format.Tags.get(sharedtags.RMAuthor)),
				new:      strings.TrimSpace(fd.MCredits.Author),
			},
			sharedtags.RMComment: {
				existing: strings.TrimSpace(ffData.Format.Tags.get(sharedtags.RMComment)),
				new:      strings.TrimSpace(fd.MTitleDesc.Description),
			},
			sharedtags.RMTitle: {
				existing: strings.TrimSpace(ffData.Format.Tags.get(sharedtags.RMTitle)),
				new:      strings.TrimSpace(fd.MTitleDesc.Fulltitle),
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
	if k, exists := tags[key]; exists {
		return k
	}

	// Try variants.
	if k, exists := tags[strings.ToLower(key)]; exists {
		return k
	}
	if k, exists := tags[strings.ToUpper(key)]; exists {
		return k
	}
	if k, exists := tags[strings.ToTitle(key)]; exists {
		return k
	}

	// Special WM/ case attempts:
	key, _, _ = strings.Cut(key, "WM/")
	key = tags.get(key)
	if key != "" {
		return key
	}

	return ""
}
