package parsing

import (
	"metarr/internal/models"
	"testing"
)

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
