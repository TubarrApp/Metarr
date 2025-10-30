package processing

import (
	"fmt"
	"metarr/internal/abstractions"
	"metarr/internal/domain/consts"
	"metarr/internal/domain/keys"
	"metarr/internal/models"
	"metarr/internal/utils/logging"
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
func getFileDirs() (videoDirs, videoFiles, jsonDirs, jsonFiles []string, err error) {
	if abstractions.IsSet(keys.VideoFiles) {
		videoFiles = abstractions.GetStringSlice(keys.VideoFiles)
	}
	if abstractions.IsSet(keys.VideoDirs) {
		videoDirs = abstractions.GetStringSlice(keys.VideoDirs)
	}
	if abstractions.IsSet(keys.JSONFiles) {
		jsonFiles = abstractions.GetStringSlice(keys.JSONFiles)
	}
	if abstractions.IsSet(keys.JSONDirs) {
		jsonDirs = abstractions.GetStringSlice(keys.JSONDirs)
	}

	// Check batch pairs.
	if abstractions.IsSet(keys.BatchPairs) {
		batchPairs, ok := abstractions.Get(keys.BatchPairs).(models.BatchPairs)
		if !ok {
			return nil, nil, nil, nil, fmt.Errorf("%s got wrong type %T for expected BatchPairs model", consts.LogTagDevError, batchPairs)
		}
		videoDirs = append(videoDirs, batchPairs.VideoDirs...)
		videoFiles = append(videoFiles, batchPairs.VideoFiles...)
		jsonDirs = append(jsonDirs, batchPairs.MetaDirs...)
		jsonFiles = append(jsonFiles, batchPairs.MetaFiles...)
	}

	if err := ensureNoColons(videoDirs); err != nil {
		return nil, nil, nil, nil, err
	}
	if err := ensureNoColons(videoFiles); err != nil {
		return nil, nil, nil, nil, err
	}
	if err := ensureNoColons(jsonDirs); err != nil {
		return nil, nil, nil, nil, err
	}
	if err := ensureNoColons(jsonFiles); err != nil {
		return nil, nil, nil, nil, err
	}

	videoDirs, videoFiles, jsonDirs, jsonFiles = getValidFileDirs(videoDirs, videoFiles, jsonDirs, jsonFiles)
	return videoDirs, videoFiles, jsonDirs, jsonFiles, nil
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
func getValidFileDirs(videoDirs, videoFiles, jsonDirs, jsonFiles []string) (vDirs, vFiles, jDirs, jFiles []string) {
	vDirs, misplacedVFiles := validatePaths("video directory", videoDirs)
	misplacedVDirs, vFiles := validatePaths("video file", videoFiles)
	jDirs, misplacedJFiles := validatePaths("JSON directory", jsonDirs)
	misplacedJDirs, jFiles := validatePaths("JSON file", jsonFiles)

	// Log and reassign misplaced entries
	for _, f := range misplacedVFiles {
		logging.W("User entered file %q as directory, appending to video files", f)
		vFiles = append(vFiles, f)
	}
	for _, d := range misplacedVDirs {
		logging.W("User entered directory %q as file, appending to video directories", d)
		vDirs = append(vDirs, d)
	}
	for _, f := range misplacedJFiles {
		logging.W("User entered file %q as directory, appending to valid JSON files", f)
		jFiles = append(jFiles, f)
	}
	for _, d := range misplacedJDirs {
		logging.W("User entered directory %q as file, appending to valid JSON directories", d)
		jDirs = append(jDirs, d)
	}

	return vDirs, vFiles, jDirs, jFiles
}

// validatePaths checks whether each path in 'paths' is a directory or file.
//
// It classifies them into dirs and files while logging consistent warnings.
func validatePaths(kind string, paths []string) (dirs, files []string) {
	for _, p := range paths {
		info, err := os.Stat(p)
		if err != nil {
			logging.E("Failed to stat %s path %q: %v", kind, p, err)
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
func sysResourceLoop(fileStr string) {
	var (
		resourceMsg bool
		backoff     = time.Second
		maxBackoff  = 10 * time.Second
	)

	for {
		// Fetch system resources and determine if processing can proceed
		muResource.Lock()
		proceed, availableMemory, CPUUsage, err := checkSysResources()
		muResource.Unlock()

		if err != nil {
			logging.AddToErrorArray(err)
			logging.E("Error checking system resources: %v", err)

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

		// Log resource info only once when insufficient resources are detected
		if !resourceMsg {
			logging.I("Not enough system resources to process %s, waiting...", fileStr)
			logging.D(1, "Memory available: %.2f MB\tCPU usage: %.2f%%\n", float64(availableMemory)/(consts.MB), CPUUsage)
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

	requiredMemory := abstractions.GetUint64(keys.MinFreeMem) // Default 0
	maxCPUUsage := abstractions.GetFloat64(keys.MaxCPU)       // Default 101.0

	vMem, err := mem.VirtualMemory()
	if err != nil {
		return false, 0, 0, err
	}

	cpuPct, err := cpu.Percent(0, false) // "false" outputs average across all cores
	if err != nil {
		return false, 0, 0, err
	}

	return (vMem.Available >= requiredMemory && cpuPct[0] <= maxCPUUsage), vMem.Available, cpuPct[0], nil
}

// cleanupTempFiles removes temporary files
func cleanupTempFiles(files map[string]*models.FileData) error {

	var (
		errReturn error
		path      string
	)

	if len(files) == 0 {
		logging.I("No temporary files to clean up")
		return nil
	}

	for _, data := range files {
		path = data.TempOutputFilePath
		if _, err := os.Stat(path); err == nil {
			fmt.Printf("Removing temp file: %s\n", path)
			err = os.Remove(path)
			if err != nil {
				errReturn = fmt.Errorf("error removing temp file: %w", err)
			}
		}
	}
	return errReturn
}

// printProgress creates a printout of the current process completion status.
func printProgress(fileType string, current, total int32, directory string) {
	muPrint.Lock()
	defer muPrint.Unlock()

	fmt.Printf("\n==============================================================\n")
	fmt.Printf("    Processed %s file %d of %d\n", fileType, current, total)
	fmt.Printf("    Remaining in %q: %d\n", directory, total-current)
	fmt.Printf("==============================================================\n\n")
}
