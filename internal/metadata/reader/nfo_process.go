package metadata

import (
	"fmt"
	nfo "metarr/internal/metadata/process/nfo"
	nfoRw "metarr/internal/metadata/writer/nfo"
	"metarr/internal/models"
	logging "metarr/internal/utils/logging"
	"os"
)

// ProcessNFOFiles processes NFO files and sends data into the metadata model
func ProcessNFOFiles(fd *models.FileData) (*models.FileData, error) {
	if fd == nil {
		return nil, fmt.Errorf("model passed in null")
	}

	logging.D(2, "Beginning NFO file processing...")

	// Open the file
	file, err := os.OpenFile(fd.NFOFilePath, os.O_RDWR, 0644)
	if err != nil {
		logging.ErrorArray = append(logging.ErrorArray, err)
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	nfoRW := nfoRw.NewNFOFileRW(file)
	if nfoRW != nil {
		// Store NFO RW in model
		fd.NFOFileRW = nfoRW
	}

	data, err := nfoRW.DecodeMetadata(file)
	if err != nil || data == nil {
		logging.E(0, "Failed to decode metadata from file: %v", err)
	} else {
		// Store NFO data in model
		fd.NFOData = data
	}

	edited, err := nfoRW.MakeMetaEdits(nfoRW.Meta, file, fd)
	if err != nil {
		logging.E(0, "Encountered issue making meta edits: %v", err)
	}
	if edited {
		logging.D(2, "Refreshing NFO metadata after edits were made...")
		data, err := fd.NFOFileRW.RefreshMetadata()
		if err != nil {
			return nil, err
		} else {
			fd.NFOData = data
		}
	}

	// Fill to file metadata
	if ok := nfo.FillNFO(fd); !ok {
		logging.E(0, "No metadata filled from NFO file...")
	}
	return fd, nil
}
