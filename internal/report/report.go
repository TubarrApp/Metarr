// Package report prints what a caller of Metarr (e.g. Tubarr) reads from its stdout: file paths and errors.
package report

import (
	"fmt"
	"metarr/internal/domain/logger"
	"os"
	"strings"
)

// VideoPath prints the video's path as soon as it changes on disk, so a caller can find it if the run fails later.
func VideoPath(path string) {
	if _, err := fmt.Fprintf(os.Stdout, "current video path: %s\n", path); err != nil {
		logger.Pl.E("Failed to output current video path %q due to error: %v", path, err)
	}
}

// FinalPaths prints the video and metadata paths once all transformations are complete.
func FinalPaths(videoPath, metaPath string) {
	if _, err := fmt.Fprintf(os.Stdout, "final video path: %s\n", videoPath); err != nil {
		logger.Pl.E("Failed to output final video path for %q due to error: %v", videoPath, err)
	}
	if _, err := fmt.Fprintf(os.Stdout, "final json path: %s\n", metaPath); err != nil {
		logger.Pl.E("Failed to output final JSON path for %q due to error: %v", metaPath, err)
	}
}

// Error prints an error's first line on stdout, beside the paths, so a caller can show why a run failed.
func Error(err error) {
	msg, _, _ := strings.Cut(err.Error(), "\n")
	if _, err := fmt.Fprintf(os.Stdout, "metarr error: %s\n", msg); err != nil {
		logger.Pl.E("Failed to output error %q due to error: %v", msg, err)
	}
}
