package processing

import (
	"context"
	"errors"
	"fmt"
	"metarr/internal/domain/logger"
	"metarr/internal/domain/vars"
	"metarr/internal/metadata/fieldsnfo"
	"metarr/internal/metadata/metawriters"
	"metarr/internal/models"
	"os"
	"sync"
)

var nfoEditMutexMap sync.Map

// processNFOFiles processes NFO files and sends data into the metadata model.
func processNFOFiles(ctx context.Context, fd *models.FileData, skipVideos bool) error {
	if fd == nil {
		return errors.New("model passed in null")
	}

	logger.Pl.D(2, "Beginning NFO file processing...")

	filePath := fd.MetaFilePath
	value, _ := nfoEditMutexMap.LoadOrStore(filePath, &sync.Mutex{})
	fileMutex, ok := value.(*sync.Mutex)
	if !ok {
		return fmt.Errorf("internal error: mutex map corrupted for file %s", filePath)
	}

	fileMutex.Lock()
	defer fileMutex.Unlock()

	// Open the file.
	file, err := os.OpenFile(fd.MetaFilePath, os.O_RDWR, 0o644)
	if err != nil {
		vars.AddToErrorArray(err)
		return fmt.Errorf("failed to open file: %w", err)
	}
	defer func() {
		if closeErr := file.Close(); closeErr != nil {
			logger.Pl.E("Failed to close file %q: %v", file.Name(), closeErr)
		}
	}()

	nfoRW := metawriters.NewNFOFileRW(ctx, file)

	nfoData, err := nfoRW.DecodeMetadata(file)
	if err != nil || nfoData == nil {
		logger.Pl.E("Failed to decode metadata from file: %v", err)
	} else {
		// Store NFO data in model.
		fd.NFOData = nfoData
	}

	// Read web details before editing, so channel-scoped operations can be narrowed.
	if fd.NFOData != nil {
		fieldsnfo.FillWebData(fd)
	}
	resolveOps(fd, nil)

	edited, err := nfoRW.MakeMetaEdits(nfoRW.Meta, file, fd)
	if err != nil {
		logger.Pl.E("Encountered issue making meta edits: %v", err)
	}
	if edited {
		logger.Pl.D(2, "Refreshing NFO metadata after edits were made...")
		data, err := nfoRW.RefreshMetadata()
		if err != nil {
			return err
		}
		fd.NFOData = data
	}

	// Fill to file metadata.
	if ok := fieldsnfo.FillNFO(fd); !ok {
		logger.Pl.E("No metadata filled from NFO file...")
	}

	// Check if metadata is already existent in target file.
	if !skipVideos {
		logger.Pl.D(1, "Checking if metadata already exists in target file %q...", fd.OriginalVideoPath)
		if filetypeMetaCheckSwitch(ctx, fd) {
			logger.Pl.I("Metadata already exists in target file %q", fd.OriginalVideoPath)
			fd.MetaAlreadyExists = true
		}
	}
	return nil
}
