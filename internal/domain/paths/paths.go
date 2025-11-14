// Package paths initializes Metarr's filepaths, directories, etc.
package paths

import (
	"errors"
	"fmt"
	"metarr/internal/abstractions"
	"metarr/internal/domain/consts"
	"metarr/internal/domain/keys"
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
	HomeMetarrDir string
	LogFilePath   string
	BenchmarkDir  string
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

	// Main files
	LogFilePath = filepath.Join(HomeMetarrDir, logFile)

	// Benchmark directory
	if abstractions.IsSet(keys.Benchmarking) {
		BenchmarkDir = filepath.Join(HomeMetarrDir, benchmarkDir)
		if _, err := os.Stat(BenchmarkDir); os.IsNotExist(err) {
			if err := os.MkdirAll(BenchmarkDir, consts.PermsGenericDir); err != nil {
				return fmt.Errorf("failed to make benchmark directory: %w", err)
			}
		}
	}
	return nil
}
