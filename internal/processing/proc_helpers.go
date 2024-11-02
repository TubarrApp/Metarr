package processing

import (
	"Metarr/internal/config"
	keys "Metarr/internal/domain/keys"
	"Metarr/internal/types"
	logging "Metarr/internal/utils/logging"
	"fmt"
	"os"
	"time"

	"github.com/shirou/gopsutil/cpu"
	"github.com/shirou/gopsutil/mem"
)

// sysResourceLoop checks the system resources, controlling whether a new routine should be spawned
func sysResourceLoop(fileStr string) {

	var resourceMsg bool
	var audioMemoryThreshold uint64 = config.GetUint64(keys.MinMemMB)

	for {
		// Fetch system resources and determine if processing can proceed
		proceed, availableMemory, CPUUsage, err := checkSysResources(audioMemoryThreshold)
		if err != nil {
			logging.ErrorArray = append(logging.ErrorArray, err)
			logging.PrintE(0, "Error checking system resources: %v", err)
		}
		if proceed {
			resourceMsg = false
			break
		}

		// Log resource info only once when insufficient resources are detected
		if !resourceMsg {
			logging.PrintI("Not enough system resources to process %s, waiting...", fileStr)
			logging.PrintD(1, "Memory available: %.2f MB\tCPU usage: %.2f%%\n", float64(availableMemory)/(1024*1024), CPUUsage)
			resourceMsg = true
		}
		time.Sleep(1 * time.Second) // Wait before checking again
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

	maxCpuUsage := config.GetFloat64(keys.MaxCPU)
	return (vMem.Available >= requiredMemory && cpuPct[0] <= maxCpuUsage), vMem.Available, cpuPct[0], nil
}

// cleanupTempFiles removes temporary files
func cleanupTempFiles(files map[string]*types.FileData) error {

	var errReturn error
	var path string

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
