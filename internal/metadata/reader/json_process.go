package metadata

import (
	"fmt"
	"metarr/internal/config"
	"metarr/internal/dates"
	enums "metarr/internal/domain/enums"
	keys "metarr/internal/domain/keys"
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

	logging.D(2, "Beginning JSON file processing...")

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

	logging.D(3, "%v", data)

	var (
		ok,
		gotTime bool
	)

	if ok = process.FillWebpageDetails(fd, data); ok {
		logging.I("URLs grabbed: %s", w.TryURLs)
	}

	if len(w.TryURLs) > 0 {
		transformations.TryTransPresets(w.TryURLs, fd)
	}

	// Make metadata adjustments per user selection
	edited, err := fd.JSONFileRW.MakeMetaEdits(data, file, fd)
	if err != nil {
		return nil, err
	}
	if edited {
		logging.D(2, "Refreshing JSON metadata after edits were made...")
		if data, err = fd.JSONFileRW.RefreshMetadata(); err != nil {
			return nil, err
		}
	}

	if data, ok = process.FillTimestamps(fd, data); !ok {
		logging.I("No date metadata found")
	}

	if config.IsSet(keys.MDateTagMap) {
		ok, err = jsonRW.MakeDateTagEdits(data, file, fd)
		if err != nil {
			logging.E(0, err.Error())
		} else if !ok {
			logging.E(0, "Did not make date tag edits for metadata, tag already exists?")
		}
	} else {
		logging.D(4, "Skipping making metadata date tag edits, key not set")
	}

	if data, ok = process.FillMetaFields(fd, data, gotTime); !ok {
		logging.D(2, "Some metafields were unfilled")
	}

	if fd.MDates.FormattedDate == "" {
		dates.FormatAllDates(fd)
	}

	// Make date tag
	logging.D(3, "About to make date tag for: %v", file.Name())
	if config.IsSet(keys.FileDateFmt) {

		if dateFmt, ok := config.Get(keys.FileDateFmt).(enums.DateFormat); !ok {
			logging.E(0, "Got null or wrong type for file date format. Got type %T", dateFmt)
		} else if dateFmt != enums.DATEFMT_SKIP {
			fd.FilenameDateTag, err = tags.MakeFileDateTag(data, file.Name(), dateFmt)
			if err != nil {
				logging.E(0, "Failed to make date tag: %v", err)
			}
		} else {
			logging.D(1, "Set file date tag format to skip, not making date tag for '%s'", file.Name())
		}
	}

	// Add new filename tag for files
	if config.IsSet(keys.MFilenamePfx) {
		logging.D(3, "About to make prefix tag for: %v", file.Name())
		fd.FilenameMetaPrefix = tags.MakeFilenameTag(data, file)
	}

	// Check if metadata is already existent in target file
	if filetypeMetaCheckSwitch(fd) {
		logging.I("Metadata already exists in target file '%s', will skip processing", fd.OriginalVideoBaseName)
		fd.MetaAlreadyExists = true
	}

	return fd, nil
}

func filetypeMetaCheckSwitch(fd *models.FileData) bool {

	logging.D(4, "Entering filetypeMetaCheckSwitch with '%s'", fd.OriginalVideoPath)

	var outExt string
	outFlagSet := config.IsSet(keys.OutputFiletype)

	if outFlagSet {
		outExt = config.GetString(keys.OutputFiletype)
	} else {
		outExt = filepath.Ext(fd.OriginalVideoPath)
		logging.D(2, "Got output extension as %s", outExt)
	}

	currentExt := filepath.Ext(fd.OriginalVideoPath)
	currentExt = strings.TrimSpace(currentExt)

	if outExt != "" && !strings.HasPrefix(outExt, ".") {
		outExt = "." + outExt

		logging.D(2, "Added dot to outExt: %s, currentExt is %s", outExt, currentExt)
	}

	if outFlagSet && outExt != "" && !strings.EqualFold(outExt, currentExt) {
		logging.I("Input format '%s' differs from output format '%s', will not run metadata checks", currentExt, outExt)
		return false
	}

	// Run metadata checks in all other cases
	switch currentExt {
	case ".mp4":
		return check.MP4MetaMatches(fd)
	default:
		logging.I("Checks not currently implemented for this filetype")
		return false
	}
}
