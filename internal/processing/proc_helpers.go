package processing

import (
	"fmt"
	"metarr/internal/cfg"
	keys "metarr/internal/domain/keys"
	"metarr/internal/models"
	logging "metarr/internal/utils/logging"
	"os"
	"sync"
	"time"

	"github.com/shirou/gopsutil/cpu"
	"github.com/shirou/gopsutil/mem"
)

var (
	muResource sync.Mutex
)

// sysResourceLoop checks the system resources, controlling whether a new routine should be spawned
func sysResourceLoop(fileStr string) {
	var (
		resourceMsg bool
		backoff     = time.Second
		maxBackoff  = 10 * time.Second
	)

	memoryThreshold := cfg.GetUint64(keys.MinMemMB)

	for {
		// Fetch system resources and determine if processing can proceed
		muResource.Lock()
		proceed, availableMemory, CPUUsage, err := checkSysResources(memoryThreshold)
		muResource.Unlock()

		if err != nil {
			logging.ErrorArray = append(logging.ErrorArray, err)
			logging.E(0, "Error checking system resources: %v", err)

			time.Sleep(backoff)
			backoff *= 2
			if backoff > maxBackoff {
				backoff = maxBackoff
			}
			continue
		}

		if proceed {
			resourceMsg = false
			break
		}

		// Log resource info only once when insufficient resources are detected
		if !resourceMsg {
			logging.I("Not enough system resources to process %s, waiting...", fileStr)
			logging.D(1, "Memory available: %.2f MB\tCPU usage: %.2f%%\n", float64(availableMemory)/(1024*1024), CPUUsage)
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
func checkSysResources(requiredMemory uint64) (bool, uint64, float64, error) {
	vMem, err := mem.VirtualMemory()
	if err != nil {
		return false, 0, 0, err
	}

	cpuPct, err := cpu.Percent(0, false)
	if err != nil {
		return false, 0, 0, err
	}

	maxCpuUsage := cfg.GetFloat64(keys.MaxCPU)
	return (vMem.Available >= requiredMemory && cpuPct[0] <= maxCpuUsage), vMem.Available, cpuPct[0], nil
}

// cleanupTempFiles removes temporary files
func cleanupTempFiles(files map[string]*models.FileData) error {

	var (
		errReturn error
		path      string
	)

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

// metaChanges determines if metadata should be processed
func metaChanges() bool {
	if cfg.IsSet(keys.MAppend) {
		return true
	}
	if cfg.IsSet(keys.MPrefix) {
		return true
	}
	if cfg.IsSet(keys.MNewField) {
		return true
	}
	if cfg.IsSet(keys.MTrimPrefix) {
		return true
	}
	if cfg.IsSet(keys.MTrimSuffix) {
		return true
	}
	if cfg.IsSet(keys.FileDateFmt) {
		return true
	}
	return false
}
