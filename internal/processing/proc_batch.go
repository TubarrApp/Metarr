package processing

import (
	"errors"
	"fmt"
	"metarr/internal/models"
	"metarr/internal/utils/logging"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
)

var batchPool = &sync.Pool{
	New: func() interface{} {
		return &batchProcessor{
			failures: struct {
				items []failedVideo
				pool  []failedVideo
				mu    sync.Mutex
			}{
				items: make([]failedVideo, 0, 32),
				pool:  make([]failedVideo, 0, 32),
			},
		}
	},
}

// processBatch is the entrypoint for batch processing.
func processBatch(batch *batch, core *models.Core, openVideo, openMeta *os.File) error {
	if batch == nil {
		return errors.New("batch entered null")
	}

	var err error
	if batch.bp, err = getNewBatchProcessor(batch.ID); err != nil {
		return err
	}
	defer batch.bp.release()

	if err = processFiles(batch, core, openVideo, openMeta); err != nil {
		return err
	}

	errArray := logging.GetErrorArray()
	if len(errArray) == 0 {
		logging.S("Successfully processed all files in directory %q with no errors.\n", filepath.Dir(batch.bp.filepaths.metaFile))
		return nil
	}

	return nil
}

// getBatchProcessor returns the singleton batchProcessor instance
func getNewBatchProcessor(batchID int64) (*batchProcessor, error) {
	bp := batchPool.Get().(*batchProcessor)
	if bp == nil {
		return nil, fmt.Errorf("failed to get batch processor from pool for batch with ID %d", batchID)
	}
	bp.batchID = batchID
	return bp, nil
}

// addFailedVideo aadds a new failed video to the array.
func (bp *batchProcessor) addFailure(f failedVideo) {
	bp.failures.mu.Lock()
	bp.failures.items = append(bp.failures.items, f)
	bp.failures.mu.Unlock()
}

// logFailedVideos logs videos which failed during this batch.
func (bp *batchProcessor) logFailedVideos() {

	if len(bp.failures.items) == 0 {
		return
	}

	for i, failed := range bp.failures.items {
		if i == 0 {
			logging.E("Program finished, but some errors were encountered:")
		}
		fmt.Println()
		logging.P("Filename: %v", failed.filename)
		logging.P("Error: %v", failed.err)
	}
	fmt.Println()
}

// syncMapToRegularMap converts the sync map back to a regular map for further processing.
func (bp *batchProcessor) syncMapToRegularMap(m *sync.Map) map[string]*models.FileData {
	result := make(map[string]*models.FileData)
	m.Range(func(key, value interface{}) bool {
		if fd, ok := value.(*models.FileData); ok {
			result[key.(string)] = fd
		}
		return true
	})
	return result
}

// reset prepares the batch processor for new batch operation.
func (bp *batchProcessor) reset(expectedCount int) {
	// Reset counters atomically
	atomic.StoreInt32(&bp.counts.totalMeta, 0)
	atomic.StoreInt32(&bp.counts.totalVideo, 0)
	atomic.StoreInt32(&bp.counts.totalMeta, 0)
	atomic.StoreInt32(&bp.counts.processedMeta, 0)
	atomic.StoreInt32(&bp.counts.processedVideo, 0)

	// Clear sync.Maps
	bp.files.matched.Range(func(k, v interface{}) bool {
		bp.files.matched.Delete(k)
		return true
	})
	bp.files.video.Range(func(k, v interface{}) bool {
		bp.files.video.Delete(k)
		return true
	})

	// Reset failures
	bp.failures.mu.Lock()
	if bp.failures.pool == nil {
		bp.failures.pool = make([]failedVideo, 0, max(32, expectedCount))
		bp.failures.items = bp.failures.pool
	} else if cap(bp.failures.pool) >= expectedCount {
		bp.failures.items = bp.failures.pool[:0]
	} else {
		newCap := max(expectedCount, cap(bp.failures.pool)*2)
		newPool := make([]failedVideo, 0, newCap)
		bp.failures.pool = newPool
		bp.failures.items = newPool
	}
	bp.failures.mu.Unlock()
}

// release returns the batchProcessor to the pool and sets values back to defaults.
func (bp *batchProcessor) release() {
	bp.reset(0)
	bp.batchID = 0
	batchPool.Put(bp)
}
