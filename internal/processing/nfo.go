package processing

import (
	"context"
	"errors"
	"fmt"
	"metarr/internal/metadata/fieldsnfo"
	"metarr/internal/metadata/metawriters"
	"metarr/internal/models"
	"metarr/internal/utils/logging"
	"os"
	"sync"
)

var nfoEditMutexMap sync.Map

// processNFOFiles processes NFO files and sends data into the metadata model.
func processNFOFiles(ctx context.Context, fd *models.FileData) error {
	if fd == nil {
		return errors.New("model passed in null")
	}

	logging.D(2, "Beginning NFO file processing...")

	filePath := fd.MetaFilePath
	value, _ := nfoEditMutexMap.LoadOrStore(filePath, &sync.Mutex{})
	fileMutex, ok := value.(*sync.Mutex)
	if !ok {
		return fmt.Errorf("internal error: mutex map corrupted for file %s", filePath)
	}

	fileMutex.Lock()
	defer fileMutex.Unlock()

	// Open the file
	file, err := os.OpenFile(fd.MetaFilePath, os.O_RDWR, 0o644)
	if err != nil {
		logging.AddToErrorArray(err)
		return fmt.Errorf("failed to open file: %w", err)
	}
	defer func() {
		if closeErr := file.Close(); closeErr != nil {
			logging.E("Failed to close file %q: %v", file.Name(), closeErr)
		}
	}()

	nfoRW := metawriters.NewNFOFileRW(ctx, file)

	nfoData, err := nfoRW.DecodeMetadata(file)
	if err != nil || nfoData == nil {
		logging.E("Failed to decode metadata from file: %v", err)
	} else {
		// Store NFO data in model
		fd.NFOData = nfoData
	}

	edited, err := nfoRW.MakeMetaEdits(nfoRW.Meta, file, fd)
	if err != nil {
		logging.E("Encountered issue making meta edits: %v", err)
	}
	if edited {
		logging.D(2, "Refreshing NFO metadata after edits were made...")
		data, err := nfoRW.RefreshMetadata()
		if err != nil {
			return err
		}
		fd.NFOData = data
	}

	// Fill to file metadata
	if ok := fieldsnfo.FillNFO(fd); !ok {
		logging.E("No metadata filled from NFO file...")
	}
	return nil
}
