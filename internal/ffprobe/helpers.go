// Package ffprobe helps determine the metadata already present in a video file.
//
// This is used to determine if a video file should be encoded.
package ffprobe

import (
	"metarr/internal/utils/logging"
	"strings"
)

type ffprobeFormat struct {
	Tags ffprobeTags `json:"tags"`
}

type ffprobeOutput struct {
	Format ffprobeFormat `json:"format"`
}

type ffprobeTags struct {
	Description  string `json:"description"`
	Synopsis     string `json:"synopsis"`
	Title        string `json:"title"`
	CreationTime string `json:"creation_time"`
	Date         string `json:"date"`
	Artist       string `json:"artist"`
	Composer     string `json:"composer"`
}

// getDatePart safely extracts the date part before 'T' if it exists.
func getDatePart(timeStr string) string {
	timeStr = strings.TrimSpace(timeStr)
	if parts := strings.Split(timeStr, "T"); len(parts) > 0 {
		return parts[0]
	}
	return timeStr
}

// printArray provides a simple print of metadata captured by FFprobe.
func printArray(s []string) {
	str := strings.Join(s, ", ")
	logging.I("FFprobe captured %s", str)
}
