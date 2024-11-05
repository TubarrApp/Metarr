package metadata

import (
	logging "Metarr/internal/utils/logging"
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

// safeGetDatePart safely extracts the date part before 'T' if it exists
func safeGetDatePart(timeStr string) string {
	timeStr = strings.TrimSpace(timeStr)
	if parts := strings.Split(timeStr, "T"); len(parts) > 0 {
		return parts[0]
	}
	return timeStr
}

func printArray(s []string) {
	str := strings.Join(s, ", ")
	logging.PrintI("FFprobe captured %s", str)
}
