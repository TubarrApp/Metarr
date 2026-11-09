package parsing

import (
	"io"
	"testing"

	"metarr/internal/dates"
	"metarr/internal/domain/logger"
	"metarr/internal/models"

	"github.com/TubarrApp/gocommon/sharedconsts"
	"github.com/TubarrApp/gocommon/sharedenums"
	"github.com/TubarrApp/gocommon/sharedtags"
)

// ******************************** Container keys ************************************************************************************

// TestGetContainerKeys tests the tag a field is written as in each kind of container.
func TestGetContainerKeys(t *testing.T) {
	for _, c := range []struct {
		key, ext, want string
	}{
		// MP4 family takes the iTunes tags.
		{sharedtags.JShow, ".mp4", sharedtags.MP4Show},
		{sharedtags.JAlbumArtist, ".m4v", sharedtags.MP4AlbumArtist},
		{sharedtags.JTrackNumber, ".f4v", sharedtags.MP4Track},
		{sharedtags.JActor, ".mp4", ""},
		{sharedtags.JTitle, ".MP4", sharedtags.MP4Title},

		// MOV and 3GP take fewer, putting the description in the comment.
		{sharedtags.JDescription, ".mov", sharedtags.MOVComment},
		{sharedtags.JComment, ".mov", ""},
		{sharedtags.JSynopsis, ".mov", ""},
		{sharedtags.JAuthor, ".mov", ""},
		{sharedtags.JAuthor, ".3gp", sharedtags.ThreeGPAuthor},
		{sharedtags.JYear, ".3g2", sharedtags.ThreeGPDate},

		// AVI only takes INFO tags FFmpeg writes.
		{sharedtags.JKeywords, ".avi", sharedtags.AVIKeywords},
		{sharedtags.JActor, ".avi", ""},
		{sharedtags.JYear, ".avi", ""},

		// Containers taking any key keep the field's own name where there's no standard one.
		{sharedtags.JActor, ".mkv", sharedtags.MatroskaActor},
		{sharedtags.JPerformer, ".webm", sharedtags.MatroskaLeadPerformer},
		{sharedtags.JSeasonTitle, ".mkv", sharedtags.JSeasonTitle},
		{sharedtags.JComment, ".ogv", ""},
		{sharedtags.JShow, ".ogm", sharedtags.JShow},
		{sharedtags.JArtist, ".wmv", sharedtags.ASFAuthor},
		{sharedtags.JAuthor, ".asf", ""},
		{sharedtags.JShow, ".flv", sharedtags.JShow},

		// Containers with no or few tags.
		{sharedtags.JTitle, ".ts", sharedtags.TSServiceName},
		{sharedtags.JDescription, ".mts", ""},
		{sharedtags.JCopyright, ".rmvb", sharedtags.RMCopyright},
		{sharedtags.JTitle, ".vob", ""},
	} {
		if got := GetContainerKeys(c.key, c.ext); got != c.want {
			t.Errorf("GetContainerKeys(%q, %q) = %q, want %q", c.key, c.ext, got, c.want)
		}
	}
}

// ******************************** Directories ***************************************************************************************

// TestDateTemplateTags tests the year, month and day tags and their directives.
func TestDateTemplateTags(t *testing.T) {
	dp := NewDirectoryParser(&models.FileData{
		MDates: &models.MetadataDates{Date: "2026-03-07", UploadDate: "20260307", Year: "2026"},
	})

	for tag, want := range map[string]string{
		"year":              "2026",
		"year:yyyy":         "2026",
		"year:yy":           "26",
		"month":             "03",
		"month:number":      "03",
		"month:abbreviated": "Mar",
		"Month:Word":        "March",
		"day":               "07",
	} {
		got, err := dp.replace(tag)
		if err != nil || got != want {
			t.Errorf("replace(%q) = %q, %v; want %q", tag, got, err, want)
		}
	}

	dp.FD.MWebData = &models.MetadataWebData{VideoURL: "https://www.youtube.com/watch?v=abc"}
	if got, err := dp.replace("video_url"); err != nil || got != "www.youtube.com/watch?v=abc" {
		t.Errorf("replace(video_url) = %q, %v; want the URL without its scheme", got, err)
	}
	dp.FD.MWebData.WebpageURL, dp.FD.MWebData.VideoURL = "https://www.youtube.com/watch?v=abc", "https://cdn.example/stream.mp4"
	if got, err := dp.replace("video_url"); err != nil || got != "www.youtube.com/watch?v=abc" {
		t.Errorf("replace(video_url) = %q, %v; want the webpage URL over the media stream URL", got, err)
	}

	for _, tag := range []string{"year:yyy", "month:short", "day:word", "video_title:yy"} {
		if got, err := dp.replace(tag); err == nil {
			t.Errorf("replace(%q) = %q, want an error", tag, got)
		}
	}
}

// TestDateTemplateTagsFromUploadDate tests month and day come from a date matching the year when Date is empty.
func TestDateTemplateTagsFromUploadDate(t *testing.T) {
	dp := NewDirectoryParser(&models.FileData{
		MDates: &models.MetadataDates{UploadDate: "20251231", Year: "2025"},
	})
	if got, err := dp.replace("month:word"); err != nil || got != "December" {
		t.Errorf("replace(month:word) = %q, %v; want December", got, err)
	}

	// No full date at all.
	dp = NewDirectoryParser(&models.FileData{MDates: &models.MetadataDates{Year: "2025"}})
	if _, err := dp.replace("day"); err == nil {
		t.Error("replace(day) with no full date succeeded, want an error")
	}
}

// ******************************** Files *********************************************************************************************

// TestHoldsThumbnail tests only the containers thumbnails are written to report holding one, whatever the extension's case.
func TestHoldsThumbnail(t *testing.T) {
	for ext, want := range map[string]bool{
		sharedconsts.ExtMP4:  true,
		sharedconsts.ExtM4V:  true,
		sharedconsts.ExtMKV:  true,
		".MP4":               true,
		sharedconsts.ExtMOV:  false,
		sharedconsts.ExtWEBM: false,
		sharedconsts.ExtAVI:  false,
		"":                   false,
	} {
		if got := HoldsThumbnail(ext); got != want {
			t.Errorf("HoldsThumbnail(%q) = %v, want %v", ext, got, want)
		}
	}
}

// TestOutputExt tests a video keeps its own extension, unless FFmpeg can't write it, when none is set.
func TestOutputExt(t *testing.T) {
	for _, c := range []struct {
		path, want string
		replaced   bool
	}{
		{"/videos/a.mp4", ".mp4", false},
		{"/videos/a.rm", sharedconsts.ExtMKV, true},
		{"/videos/a.RMVB", sharedconsts.ExtMKV, true},
		{"/videos/a.unknown", ".unknown", false},
	} {
		if got, replaced := OutputExt(c.path); got != c.want || replaced != c.replaced {
			t.Errorf("OutputExt(%q) = %q, %v; want %q, %v", c.path, got, replaced, c.want, c.replaced)
		}
	}
}

// TestReadOnlyExt tests only the video extensions FFmpeg can't write are read-only.
func TestReadOnlyExt(t *testing.T) {
	for ext, want := range map[string]bool{".rm": true, ".RMVB": true, ".mkv": false, ".ts": false, ".unknown": false} {
		if got := ReadOnlyExt(ext); got != want {
			t.Errorf("ReadOnlyExt(%q) = %v, want %v", ext, got, want)
		}
	}
}

// ******************************** Metadata ******************************************************************************************

// TestFillMetaTemplateTag tests metadata tags are all filled, numbers included, and filled values aren't expanded again.
func TestFillMetaTemplateTag(t *testing.T) {
	logger.Pl.Console = io.Discard
	mtp := NewMetaTemplateParser("test.json")
	j := map[string]any{
		"title":     "Cats",
		"uploader":  "Bob",
		"duration":  float64(125),
		"rating":    4.5,
		"is_live":   false,
		"empty":     "",
		"recursive": "{{title}}",
		"tags":      []any{"a"},
	}

	for in, want := range map[string]string{
		"{{uploader}} - {{title}}":   "Bob - Cats",
		"{{missing}} {{title}}":      "{{missing}} Cats", // A missing tag doesn't stop later ones.
		"{{ title }}":                "Cats",
		"{{duration}}s, {{rating}}*": "125s, 4.5*",
		"live: {{is_live}}":          "live: false",
		"{{empty}}{{tags}}":          "{{empty}}{{tags}}",
		"{{recursive}}":              "{{title}}", // Values are literal, not re-filled.
		"no tags":                    "no tags",
	} {
		if got, _ := mtp.FillMetaTemplateTag(in, j); got != want {
			t.Errorf("FillMetaTemplateTag(%q) = %q, want %q", in, got, want)
		}
	}

	if _, isTemplate := mtp.FillMetaTemplateTag("plain", j); isTemplate {
		t.Error("plain text reported as a template")
	}
}

// TestFillAllTemplateTags tests a string only counts as filled if every tag was, however its value reads.
func TestFillAllTemplateTags(t *testing.T) {
	logger.Pl.Console = io.Discard
	mtp := NewMetaTemplateParser("test.json")
	j := map[string]any{"title": "Cats", "braces": "{{literal}}", "empty": "", "tags": []any{"a"}}

	for _, tc := range []struct {
		in, want string
		filled   bool
	}{
		{"plain", "plain", true},
		{"{{title}}", "Cats", true},
		{"{{title}} and {{title}}", "Cats and Cats", true},
		{"a {{ b", "a {{ b", true},
		{"a }} b {{ c", "a }} b {{ c", true},
		{"{{braces}}", "{{literal}}", true},
		{"{{nope}}", "", false},
		{"{{title}} {{nope}}", "", false},
		{"{{empty}}", "", false},
		{"{{tags}}", "", false},
	} {
		if got, filled := mtp.FillAllTemplateTags(tc.in, j); got != tc.want || filled != tc.filled {
			t.Errorf("FillAllTemplateTags(%q) = %q, %v; want %q, %v", tc.in, got, filled, tc.want, tc.filled)
		}
	}
}

// TestMakeDateTagFormats tests a date tag is made in every format, including the two without a year.
func TestMakeDateTagFormats(t *testing.T) {
	fd := &models.FileData{MDates: &models.MetadataDates{FormattedDate: "2024-05-01"}}
	for format, want := range map[sharedenums.DateFormat]string{
		sharedenums.DateYyyyMmDd: "[2024-05-01]",
		sharedenums.DateYyMmDd:   "[24-05-01]",
		sharedenums.DateDdMmYyyy: "[01-05-2024]",
		sharedenums.DateMmDdYy:   "[05-01-24]",
		sharedenums.DateMmDd:     "[05-01]",
		sharedenums.DateDdMm:     "[01-05]",
	} {
		if got, err := dates.MakeDateTag(nil, fd, format); err != nil || got != want {
			t.Errorf("MakeDateTag(%v) = %q, %v, want %q", format, got, err, want)
		}
	}
}
