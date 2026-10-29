package processing

import (
	"context"
	"fmt"
	"metarr/internal/abstractions"
	"metarr/internal/domain/consts"
	"metarr/internal/domain/keys"
	"metarr/internal/domain/logger"
	"metarr/internal/models"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/TubarrApp/gocommon/sharedconsts"
	"github.com/TubarrApp/gocommon/sharedmodels"
	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/mem"
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
			return nil, nil, nil, nil, fmt.Errorf("%s got wrong type %T for expected BatchPairs model", sharedconsts.ErrorfDevError, batchPairs)
		}
		videoDirs = append(videoDirs, batchPairs.VideoDirs...)
		videoFiles = append(videoFiles, batchPairs.VideoFiles...)
		metaDirs = append(metaDirs, batchPairs.MetaDirs...)
		metaFiles = append(metaFiles, batchPairs.MetaFiles...)
	}

	if videoDirs, err = absPaths(videoDirs); err != nil {
		return nil, nil, nil, nil, err
	}
	if videoFiles, err = absPaths(videoFiles); err != nil {
		return nil, nil, nil, nil, err
	}
	if metaDirs, err = absPaths(metaDirs); err != nil {
		return nil, nil, nil, nil, err
	}
	if metaFiles, err = absPaths(metaFiles); err != nil {
		return nil, nil, nil, nil, err
	}

	videoDirs, videoFiles, metaDirs, metaFiles = getValidFileDirs(videoDirs, videoFiles, metaDirs, metaFiles)
	return videoDirs, videoFiles, metaDirs, metaFiles, nil
}

// absPaths returns the paths made absolute, as FFmpeg reads a relative name like "Title:x.mp4" as a protocol.
func absPaths(paths []string) ([]string, error) {
	out := make([]string, len(paths))
	for i, p := range paths {
		abs, err := filepath.Abs(p)
		if err != nil {
			return nil, fmt.Errorf("failed to make path %q absolute: %w", p, err)
		}
		out[i] = abs
	}
	return out, nil
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
		// Fetch system resources and determine if processing can proceed.
		muResource.Lock()
		proceed, availableMemory, CPUUsage, err := checkSysResources()
		muResource.Unlock()

		switch {
		case err != nil:
			logger.Pl.E("Error checking system resources: %v", err)
		case proceed:
			return
		case !resourceMsg:
			// Log resource info only once when insufficient resources are detected.
			logger.Pl.I("Not enough system resources to process %s, waiting...", fileStr)
			logger.Pl.D(1, "Memory available: %.2f MB\tCPU usage: %.2f%%\n", float64(availableMemory)/(consts.MB), CPUUsage)
			resourceMsg = true
		}

		select {
		case <-ctx.Done():
			return
		case <-time.After(backoff):
		}
		backoff = min(backoff*2, maxBackoff)
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

// resolveOps narrows the file's meta and filename operations to those that apply to it,
// which is only possible once its URLs and metadata have been read.
//
// meta may be nil for formats with no field map, in which case filter-gated operations
// cannot be evaluated and are skipped.
func resolveOps(fd *models.FileData, meta map[string]any) {
	if meta == nil {
		if abstractions.IsSet(keys.FilteredMetaOpsModels) || abstractions.IsSet(keys.FilteredFilenameOpsModels) {
			logger.Pl.W("Filter-gated operations are not yet supported for this metadata format, skipping them")
		}
		meta = map[string]any{}
	}

	metaFlat, ok := flatOps[sharedmodels.MetaOps](keys.MetaOpsFlat)
	if !ok {
		return
	}
	filteredMeta, ok := flatOps[sharedmodels.FilteredMetaOps](keys.FilteredMetaOpsModels)
	if !ok {
		return
	}
	if len(metaFlat) > 0 || len(filteredMeta) > 0 {
		if err := fd.ResolveMetaOps(metaFlat, filteredMeta, meta); err != nil {
			// Falling back to the startup set would apply operations meant for another
			// channel, so apply none and report it.
			logger.Pl.E("Failed to resolve meta operations for this file, applying none: %v", err)
			fd.MetaOps = models.NewMetaOps()
		}
	}

	filenameFlat, ok := flatOps[sharedmodels.FilenameOps](keys.FilenameOpsFlat)
	if !ok {
		return
	}
	filteredFilename, ok := flatOps[sharedmodels.FilteredFilenameOps](keys.FilteredFilenameOpsModels)
	if !ok {
		return
	}
	if len(filenameFlat) > 0 || len(filteredFilename) > 0 {
		if err := fd.ResolveFilenameOps(filenameFlat, filteredFilename, meta); err != nil {
			logger.Pl.E("Failed to resolve filename operations for this file, applying none: %v", err)
			fd.FilenameOps = models.NewFilenameOps()
		}
	}
}

// flatOps reads a stored operation slice, reporting false only when the stored value is
// the wrong type, which is a programming error rather than an absent setting.
func flatOps[T any](key string) ([]T, bool) {
	if !abstractions.IsSet(key) {
		return nil, true
	}

	v, ok := abstractions.Get(key).([]T)
	if !ok {
		logger.Pl.E("Expected %T for %q, got %T", v, key, abstractions.Get(key))
		return nil, false
	}
	return v, true
}
