package procmeta

import (
	"context"
	"errors"
	"fmt"
	"metarr/internal/metadata/procmeta/nfofields"
	"metarr/internal/metadata/writer/nforw"
	"metarr/internal/models"
	"metarr/internal/utils/logging"
	"os"
)

// ProcessNFOFiles processes NFO files and sends data into the metadata model
func ProcessNFOFiles(ctx context.Context, fd *models.FileData) (*models.FileData, error) {
	if fd == nil {
		return nil, errors.New("model passed in null")
	}

	logging.D(2, "Beginning NFO file processing...")

	// Open the file
	file, err := os.OpenFile(fd.NFOFilePath, os.O_RDWR, 0o644)
	if err != nil {
		logging.AddToErrorArray(err)
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer func() {
		if err := file.Close(); err != nil {
			logging.E("Failed to close file %q: %v", file.Name(), err)
		}
	}()

	nfoRW := nforw.NewNFOFileRW(ctx, file)
	if nfoRW != nil {
		// Store NFO RW in model
		fd.NFOFileRW = nfoRW
	}

	data, err := nfoRW.DecodeMetadata(file)
	if err != nil || data == nil {
		logging.E("Failed to decode metadata from file: %v", err)
	} else {
		// Store NFO data in model
		fd.NFOData = data
	}

	edited, err := nfoRW.MakeMetaEdits(nfoRW.Meta, file, fd)
	if err != nil {
		logging.E("Encountered issue making meta edits: %v", err)
	}
	if edited {
		logging.D(2, "Refreshing NFO metadata after edits were made...")
		data, err := fd.NFOFileRW.RefreshMetadata()
		if err != nil {
			return nil, err
		}
		fd.NFOData = data
	}

	// Fill to file metadata
	if ok := nfofields.FillNFO(fd); !ok {
		logging.E("No metadata filled from NFO file...")
	}
	return fd, nil
}
