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
	New: func() any {
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
func processBatch(batch *batch, core *models.Core, openVideo, openMeta *os.File) (fdArray []*models.FileData, err error) {
	if batch == nil {
		return nil, errors.New("batch entered null")
	}

	if batch.bp, err = getNewBatchProcessor(batch.ID); err != nil {
		return nil, err
	}
	defer batch.bp.release()

	if fdArray, err = processFiles(batch, core, openVideo, openMeta); err != nil {
		return fdArray, err
	}

	errArray := logging.GetErrorArray()
	if len(errArray) == 0 {
		fmt.Println()
		logging.S("Successfully processed all files in directory %q with no errors.\n", filepath.Dir(batch.bp.filepaths.metaFile))
		return fdArray, nil
	}
	return fdArray, nil
}

// getBatchProcessor returns the singleton batchProcessor instance.
func getNewBatchProcessor(batchID int64) (*batchProcessor, error) {
	bp, ok := batchPool.Get().(*batchProcessor)
	if !ok || bp == nil {
		return nil, fmt.Errorf("internal error: got type %T for batch processor with ID %d", bp, batchID)
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
			logging.E("Batch finished, but some errors were encountered:")
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
	// Reset counters
	atomic.StoreInt32(&bp.counts.totalMeta, 0)
	atomic.StoreInt32(&bp.counts.totalVideo, 0)
	atomic.StoreInt32(&bp.counts.processedMeta, 0)
	atomic.StoreInt32(&bp.counts.processedVideo, 0)

	// Replace maps
	bp.files.matched = sync.Map{}
	bp.files.video = sync.Map{}

	// Reset failures
	bp.failures.mu.Lock()
	switch {
	case bp.failures.pool == nil:
		bp.failures.pool = make([]failedVideo, 0, max(32, expectedCount))

	case cap(bp.failures.pool) >= expectedCount:
		bp.failures.pool = bp.failures.pool[:0]

	default:
		newCap := max(expectedCount, cap(bp.failures.pool)*2)
		bp.failures.pool = make([]failedVideo, 0, newCap)
	}
	bp.failures.items = bp.failures.pool
	bp.failures.mu.Unlock()
}

// release returns the batchProcessor to the pool and sets values back to defaults.
func (bp *batchProcessor) release() {
	bp.reset(0)
	bp.batchID = 0
	batchPool.Put(bp)
}
