package metadata

import (
	"fmt"
	"metarr/internal/config"
	enums "metarr/internal/domain/enums"
	keys "metarr/internal/domain/keys"
	helpers "metarr/internal/metadata/process/helpers"
	process "metarr/internal/metadata/process/json"
	check "metarr/internal/metadata/reader/check_existing"
	tags "metarr/internal/metadata/tags"
	writer "metarr/internal/metadata/writer"
	"metarr/internal/models"
	"metarr/internal/transformations"
	logging "metarr/internal/utils/logging"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

var (
	mu sync.Mutex
)

// ProcessJSONFile reads a single JSON file and fills in the metadata
func ProcessJSONFile(fd *models.FileData) (*models.FileData, error) {

	if fd == nil {
		return nil, fmt.Errorf("model passed in null")
	}

	logging.PrintD(2, "Beginning JSON file processing...")

	// Function mutex
	mu.Lock()
	defer mu.Unlock()

	filePath := fd.JSONFilePath
	w := fd.MWebData

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

	process.FillWebpageDetails(fd, data)
	logging.PrintI("URLs grabbed: %s", w.TryURLs)

	if len(w.TryURLs) > 0 {
		transformations.TryTransPresets(w.TryURLs, fd)
	}

	// Make metadata adjustments per user selection
	edited, err := fd.JSONFileRW.MakeMetaEdits(data, file, fd)
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
	if config.IsSet(keys.MFilenamePfx) {
		logging.PrintD(3, "About to make prefix tag for: %v", file.Name())
		fd.FilenameMetaPrefix = tags.MakeFilenameTag(data, file)
	}

	// Check if metadata is already existent in target file
	if filetypeMetaCheckSwitch(fd) {
		logging.PrintI("Metadata already exists in target file '%s', will skip processing", fd.OriginalVideoBaseName)
		fd.MetaAlreadyExists = true
	}

	return fd, nil
}

func filetypeMetaCheckSwitch(fd *models.FileData) bool {

	var outExt string

	outFlagSet := config.IsSet(keys.OutputFiletype)
	if outFlagSet {
		outExt = config.GetString(keys.OutputFiletype)
	}
	currentExt := filepath.Ext(fd.OriginalVideoPath)
	currentExt = strings.TrimSpace(currentExt)

	if outFlagSet && outExt != "" && outExt != currentExt {
		logging.PrintI("Input format '%s' differs from output format '%s', will not run metadata checks", currentExt, outExt)
		return false
	}

	// Run metadata checks in all other cases
	switch currentExt {
	case ".mp4":
		return check.MP4MetaMatches(fd)
	default:
		return false
	}
}
