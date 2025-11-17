// Package ffprobe helps determine the metadata already present in a video file.
//
// This is used to determine if a video file should be encoded.
package ffprobe

import (
	"metarr/internal/domain/logger"
	"strings"
)

type ffprobeFormat struct {
	Tags ffprobeTags `json:"tags"`
}

type ffprobeOutput struct {
	Streams []ffprobeStream `json:"streams"`
	Format  ffprobeFormat   `json:"format"`
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

type ffprobeStream struct {
	Index       int    `json:"index"`
	CodecType   string `json:"codec_type"`
	CodecName   string `json:"codec_name"`
	Disposition struct {
		AttachedPic int `json:"attached_pic"`
	} `json:"disposition"`
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
