// Package paths initializes Metarr's filepaths, directories, etc.
package paths

import (
	"errors"
	"fmt"
	"metarr/internal/domain/consts"
	"os"
	"path/filepath"
)

const (
	mDir         = ".metarr"
	logFile      = "metarr.log"
	benchmarkDir = "benchmark"
)

// File and directory path strings.
var (
	HomeMetarrDir     string
	MetarrLogFilePath string
	BenchmarkDir      string
)

// InitProgFilesDirs initializes necessary program directories and filepaths.
func InitProgFilesDirs() error {
	dir, err := os.UserHomeDir()
	if err != nil {
		return errors.New("failed to get home directory")
	}
	HomeMetarrDir = filepath.Join(dir, mDir)
	if _, err := os.Stat(HomeMetarrDir); os.IsNotExist(err) {
		if err := os.MkdirAll(HomeMetarrDir, consts.PermsHomeMetarrDir); err != nil {
			return fmt.Errorf("failed to make directories: %w", err)
		}
	}

	// Main files.
	MetarrLogFilePath = filepath.Join(HomeMetarrDir, logFile)
	fmt.Fprintf(os.Stderr, "Metarr log file path: %q\n", MetarrLogFilePath)

	// Benchmark directory, made by the benchmark setup if --benchmark is set (flags aren't parsed yet here).
	BenchmarkDir = filepath.Join(HomeMetarrDir, benchmarkDir)
	return nil
}
