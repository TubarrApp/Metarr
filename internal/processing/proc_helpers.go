package processing

import (
	"fmt"
	"metarr/internal/abstractions"
	"metarr/internal/domain/consts"
	"metarr/internal/domain/enums"
	"metarr/internal/domain/keys"
	"metarr/internal/models"
	"metarr/internal/transformations"
	"metarr/internal/utils/logging"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/shirou/gopsutil/cpu"
	"github.com/shirou/gopsutil/mem"
)

var (
	muPrint, muResource sync.Mutex
)

// renameFiles performs renaming operations.
func renameFiles(videoPath, metaPath string, batchID int64, fd *models.FileData, skipVideos bool) {

	var (
		replaceStyle                           enums.ReplaceToStyle
		ok                                     bool
		inputVideoDir, inputJSONDir, directory string
	)

	if abstractions.IsSet(keys.Rename) {
		if replaceStyle, ok = abstractions.Get(keys.Rename).(enums.ReplaceToStyle); !ok {
			logging.E("Received wrong type for rename style. Got %T", replaceStyle)
		} else {
			logging.D(2, "Got rename style as %T index %v", replaceStyle, replaceStyle)
		}
	}

	inputJSONDir = filepath.Dir(metaPath)
	if !skipVideos {
		inputVideoDir = filepath.Dir(videoPath)
	}

	switch {
	case inputJSONDir != "":
		directory = inputJSONDir
	case inputVideoDir != "":
		directory = inputVideoDir
	default:
		errMsg := fmt.Errorf("not renaming file, no directory detected (batch ID: %d)", batchID)
		logging.AddToErrorArray(errMsg)
		return
	}

	err := transformations.FileRename(fd, replaceStyle, skipVideos)
	if err != nil {
		logging.AddToErrorArray(err)
		logging.E("Failed to rename files: %v", err)
	} else {
		logging.S("Successfully formatted file names in directory: %s", directory)
	}
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
