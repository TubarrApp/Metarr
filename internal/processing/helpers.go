package processing

import (
	"context"
	"fmt"
	"metarr/internal/abstractions"
	"metarr/internal/domain/consts"
	"metarr/internal/domain/keys"
	"metarr/internal/domain/logger"
	"metarr/internal/domain/vars"
	"metarr/internal/models"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/shirou/gopsutil/cpu"
	"github.com/shirou/gopsutil/mem"
)

var (
	muPrint, muResource sync.Mutex
)

// getFileDirs returns files and directories entered by the user.
func getFileDirs() (videoDirs, videoFiles, metaDirs, metaFiles []string, err error) {
	if abstractions.IsSet(keys.VideoFiles) {
		videoFiles = abstractions.GetStringSlice(keys.VideoFiles)
	}
	if abstractions.IsSet(keys.VideoDirs) {
		videoDirs = abstractions.GetStringSlice(keys.VideoDirs)
	}
	if abstractions.IsSet(keys.MetaFiles) {
		metaFiles = abstractions.GetStringSlice(keys.MetaFiles)
	}
	if abstractions.IsSet(keys.MetaDirs) {
		metaDirs = abstractions.GetStringSlice(keys.MetaDirs)
	}

	// Check batch pairs.
	if abstractions.IsSet(keys.BatchPairs) {
		batchPairs, ok := abstractions.Get(keys.BatchPairs).(models.BatchPairs)
		if !ok {
			return nil, nil, nil, nil, fmt.Errorf("%s got wrong type %T for expected BatchPairs model", consts.LogTagDevError, batchPairs)
		}
		videoDirs = append(videoDirs, batchPairs.VideoDirs...)
		videoFiles = append(videoFiles, batchPairs.VideoFiles...)
		metaDirs = append(metaDirs, batchPairs.MetaDirs...)
		metaFiles = append(metaFiles, batchPairs.MetaFiles...)
	}

	if err := ensureNoColons(videoDirs); err != nil {
		return nil, nil, nil, nil, err
	}
	if err := ensureNoColons(videoFiles); err != nil {
		return nil, nil, nil, nil, err
	}
	if err := ensureNoColons(metaDirs); err != nil {
		return nil, nil, nil, nil, err
	}
	if err := ensureNoColons(metaFiles); err != nil {
		return nil, nil, nil, nil, err
	}

	videoDirs, videoFiles, metaDirs, metaFiles = getValidFileDirs(videoDirs, videoFiles, metaDirs, metaFiles)
	return videoDirs, videoFiles, metaDirs, metaFiles, nil
}

// ensureNoColons returns an error if any file or folder names in the slice contain colons, FFmpeg cannot handle them properly.
func ensureNoColons(slice []string) error {
	for _, s := range slice {
		if strings.Contains(s, ":") {
			return fmt.Errorf("failed due to invalid entry %q: FFmpeg cannot properly handle filenames or folders containing colons", s)
		}
	}
	return nil
}

// getValidFileDirs checks for validity of files and directories, with fallback handling.
func getValidFileDirs(videoDirs, videoFiles, metaDirs, metaFiles []string) (vDirs, vFiles, mDirs, mFiles []string) {
	vDirs, misplacedVFiles := validatePaths("video directory", videoDirs)
	misplacedVDirs, vFiles := validatePaths("video file", videoFiles)
	mDirs, misplacedMFiles := validatePaths("Metadata directory", metaDirs)
	misplacedMDirs, mFiles := validatePaths("Metadata file", metaFiles)

	// Log and reassign misplaced entries.
	for _, f := range misplacedVFiles {
		logger.Pl.W("User entered file %q as directory, appending to video files", f)
		vFiles = append(vFiles, f)
	}
	for _, d := range misplacedVDirs {
		logger.Pl.W("User entered directory %q as file, appending to video directories", d)
		vDirs = append(vDirs, d)
	}
	for _, f := range misplacedMFiles {
		logger.Pl.W("User entered file %q as directory, appending to valid metadata files", f)
		mFiles = append(mFiles, f)
	}
	for _, d := range misplacedMDirs {
		logger.Pl.W("User entered directory %q as file, appending to valid metadata directories", d)
		mDirs = append(mDirs, d)
	}
	return vDirs, vFiles, mDirs, mFiles
}

// validatePaths checks whether each path in 'paths' is a directory or file.
//
// It classifies them into dirs and files while logging consistent warnings.
func validatePaths(kind string, paths []string) (dirs, files []string) {
	for _, p := range paths {
		info, err := os.Stat(p)
		if err != nil {
			logger.Pl.E("Failed to stat %s path %q: %v", kind, p, err)
			continue
		}
		if info.IsDir() {
			dirs = append(dirs, p)
		} else {
			files = append(files, p)
		}
	}
	return dirs, files
}

// sysResourceLoop checks the system resources, staying in the loop until resources meet the set criteria.
func sysResourceLoop(ctx context.Context, fileStr string) {
	var (
		resourceMsg bool
		backoff     = time.Second
		maxBackoff  = 10 * time.Second
	)

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}
		// Fetch system resources and determine if processing can proceed.
		muResource.Lock()
		proceed, availableMemory, CPUUsage, err := checkSysResources()
		muResource.Unlock()

		if err != nil {
			vars.AddToErrorArray(err)
			logger.Pl.E("Error checking system resources: %v", err)

			time.Sleep(backoff)
			backoff *= 2
			if backoff > maxBackoff {
				backoff = maxBackoff
			}
			continue
		}

		if proceed {
			break
		}

		// Log resource info only once when insufficient resources are detected.
		if !resourceMsg {
			logger.Pl.I("Not enough system resources to process %s, waiting...", fileStr)
			logger.Pl.D(1, "Memory available: %.2f MB\tCPU usage: %.2f%%\n", float64(availableMemory)/(consts.MB), CPUUsage)
			resourceMsg = true
		}

		time.Sleep(backoff)
		backoff *= 2
		if backoff > maxBackoff {
			backoff = maxBackoff
		}
	}
}

// checkAvailableMemory checks if enough memory is available (at least the threshold).
func checkSysResources() (proceed bool, availMem uint64, cpuUsagePct float64, err error) {
	requiredMemory := abstractions.GetUint64(keys.MinFreeMem) // Default 0.
	maxCPUUsage := abstractions.GetFloat64(keys.MaxCPU)       // Default 101.0.

	vMem, err := mem.VirtualMemory()
	if err != nil {
		return false, 0, 0, err
	}

	cpuPct, err := cpu.Percent(0, false) // "false" outputs average across all cores.
	if err != nil {
		return false, 0, 0, err
	}

	return (vMem.Available >= requiredMemory && cpuPct[0] <= maxCPUUsage), vMem.Available, cpuPct[0], nil
}

// printProgress creates a printout of the current process completion status.
func printProgress(fileType string, current, total int32, directory string) {
	muPrint.Lock()
	defer muPrint.Unlock()

	fmt.Fprintf(os.Stderr, "\n==============================================================\n")
	fmt.Fprintf(os.Stderr, "    Processed %s file %d of %d\n", fileType, current, total)
	fmt.Fprintf(os.Stderr, "    Remaining in %q: %d\n", directory, total-current)
	fmt.Fprintf(os.Stderr, "==============================================================\n\n")
}
