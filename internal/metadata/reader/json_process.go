package metadata

import (
	"Metarr/internal/config"
	enums "Metarr/internal/domain/enums"
	keys "Metarr/internal/domain/keys"
	helpers "Metarr/internal/metadata/process/helpers"
	process "Metarr/internal/metadata/process/json"
	tags "Metarr/internal/metadata/tags"
	writer "Metarr/internal/metadata/writer"
	"Metarr/internal/types"
	logging "Metarr/internal/utils/logging"
	"fmt"
	"os"
	"sync"
)

var (
	mu sync.Mutex
)

// ProcessJSONFile reads a single JSON file and fills in the metadata
func ProcessJSONFile(fd *types.FileData) (*types.FileData, error) {

	logging.PrintD(2, "Beginning JSON file processing...")

	if fd == nil {
		return nil, fmt.Errorf("model passed in null")
	}

	// Function mutex
	mu.Lock()
	defer mu.Unlock()

	filePath := fd.JSONFilePath

	// Open the file
	file, err := os.OpenFile(filePath, os.O_RDWR, 0644)
	if err != nil {
		logging.ErrorArray = append(logging.ErrorArray, err)
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	// Grab and store metadata reader/writer
	jsonRW := writer.NewJSONFileRW(file)
	if jsonRW != nil {
		fd.JSONFileRW = jsonRW
	}

	data, err := fd.JSONFileRW.DecodeMetadata(file)
	if err != nil {
		return nil, err
	}

	logging.PrintD(3, "%v", data)

	// Make metadata adjustments per user selection
	edited, err := fd.JSONFileRW.MakeMetaEdits(data, file)
	if err != nil {
		return nil, err
	}
	if edited {
		logging.PrintD(2, "Refreshing JSON metadata after edits were made...")
		data, err = fd.JSONFileRW.RefreshMetadata()
		if err != nil {
			return nil, err
		}
	}

	var ok bool
	if data, ok = process.FillMetaFields(fd, data); !ok {
		logging.PrintD(2, "Some metafields were unfilled")
	}

	if fd.MDates.FormattedDate == "" {
		helpers.FormatAllDates(fd)
	}

	// Make date tag
	logging.PrintD(3, "About to make date tag for: %v", file.Name())
	if config.Get(keys.FileDateFmt).(enums.FilenameDateFormat) != enums.FILEDATE_SKIP {
		fd.FilenameDateTag, err = tags.MakeDateTag(data, file.Name())
		if err != nil {
			logging.PrintE(0, "Failed to make date tag: %v", err)
		}
	}

	// Add new filename tag for files
	logging.PrintD(3, "About to make prefix tag for: %v", file.Name())
	fd.FilenameMetaPrefix = tags.MakeFilenameTag(data, file)

	return fd, nil
}
